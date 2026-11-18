package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// gitCommand runs git in a session of its own with stdin on the null device, so
// SSH can find no terminal to prompt on and fails with its own message instead.
// Cancelling kills the whole group, so an SSH child dies with git.
func gitCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, gitBin, args...)
	cmd.Env = append(os.Environ(), gitNoPromptEnv)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return nil
	}
	cmd.WaitDelay = gitWaitDelay
	return cmd
}

// gitFailed names the context's cause when there is one: a killed git says only
// "signal: killed", which reads the same for a cancel and a stall.
func gitFailed(ctx context.Context, args []string, err error, output []byte) error {
	if cause := context.Cause(ctx); cause != nil {
		err = cause
	}
	return fmt.Errorf("git %v: %w\n%s", args, err, output)
}

// lingeredOnly is a git that succeeded while something it started, such as an
// SSH control master, kept its output open past the wait delay.
func lingeredOnly(cmd *exec.Cmd, err error) bool {
	return errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success()
}

func git(ctx context.Context, args ...string) error {
	cmd := gitCommand(ctx, args...)
	out, err := cmd.CombinedOutput()
	if err != nil && !lingeredOnly(cmd, err) {
		return gitFailed(ctx, args, err, out)
	}
	return nil
}

// gitOutput returns stdout. On failure the error wraps the *exec.ExitError and
// carries git's stderr.
func gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := gitCommand(ctx, args...)
	out, err := cmd.Output()
	if err != nil && !lingeredOnly(cmd, err) {
		var stderr []byte
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			stderr = exit.Stderr
		}
		return "", gitFailed(ctx, args, err, stderr)
	}
	return string(out), nil
}

func gitOK(ctx context.Context, args ...string) bool { return gitCommand(ctx, args...).Run() == nil }

// progressPattern matches git's own progress reports — "Receiving objects:  47%
// (1234/2626)" — which it writes to stderr in carriage-return-separated
// updates. The phase name is captured too: a clone runs through four of them,
// and a bar that silently restarts three times reads as a stuck one.
var progressPattern = regexp.MustCompile(`([A-Za-z][A-Za-z ]*):\s+(\d+)%`)

// gitSlow is for clone and fetch, whose callers pass progressFlag: git reports
// progress to a pipe only when asked, and the stall watch depends on hearing it.
func gitSlow(ctx context.Context, onProgress func(phase string, percent int), args ...string) error {
	ctx, stop := context.WithCancelCause(ctx)
	defer stop(nil)
	scanner := &progressScanner{onProgress: onProgress, lastPercent: -1}
	watch := watchForStall(stop)
	defer watch.timer.Stop()
	cmd := gitCommand(ctx, args...)
	cmd.Stderr = io.MultiWriter(watch, scanner)
	err := cmd.Run()
	if err != nil && !lingeredOnly(cmd, err) {
		return gitFailed(ctx, args, err, []byte(scanner.tail()))
	}
	return nil
}

// stallWatch stops git once stderr has been quiet for longer than its window:
// the longer one before the first byte, while SSH may still be authenticating.
type stallWatch struct {
	timer *time.Timer
	next  time.Duration
	heard atomic.Bool
}

func watchForStall(stop context.CancelCauseFunc) *stallWatch {
	first, next := gitFirstOutputStall, gitOutputStall
	w := &stallWatch{next: next}
	w.timer = time.AfterFunc(first, func() {
		if w.heard.Load() {
			stop(fmt.Errorf("%w: "+gitOutputStallText, ErrGitStalled, next))
			return
		}
		stop(fmt.Errorf("%w: "+gitFirstOutputStallText, ErrGitStalled, first))
	})
	return w
}

func (w *stallWatch) Write(p []byte) (int, error) {
	if len(p) > 0 {
		w.heard.Store(true)
		w.timer.Reset(w.next)
	}
	return len(p), nil
}

// progressScanner takes git's stderr as it arrives rather than by line: git
// separates updates with carriage returns, so a line scanner would deliver one
// enormous line at the end and report nothing until then. The capped tail is
// for error reporting.
type progressScanner struct {
	onProgress  func(phase string, percent int)
	tailText    strings.Builder
	lastPhase   string
	lastPercent int
	lastEmit    time.Time
}

func (s *progressScanner) Write(p []byte) (int, error) {
	chunk := string(p)
	if s.tailText.Len() < progressTailLimit {
		s.tailText.WriteString(chunk)
	}
	if s.onProgress == nil {
		return len(p), nil
	}
	if phase, percent, ok := lastProgress(chunk); ok && (phase != s.lastPhase || percent != s.lastPercent) {
		// Rate limiting before fan-out prevents a slow consumer from throttling git.
		// Phase changes and completion always pass so progress cannot stick short.
		if percent == progressComplete || phase != s.lastPhase ||
			time.Since(s.lastEmit) >= progressEmitInterval {
			s.lastEmit = time.Now()
			s.onProgress(phase, percent)
		}
		s.lastPhase, s.lastPercent = phase, percent
	}
	return len(p), nil
}

func (s *progressScanner) tail() string { return strings.TrimSpace(s.tailText.String()) }

// lastProgress reports the most recent phase and percentage in a chunk of git's
// stderr; a chunk may carry several updates and only the newest is worth drawing.
func lastProgress(chunk string) (string, int, bool) {
	matches := progressPattern.FindAllStringSubmatch(chunk, -1)
	if len(matches) == 0 {
		return "", 0, false
	}
	last := matches[len(matches)-1]
	percent, err := strconv.Atoi(last[2])
	if err != nil {
		return "", 0, false
	}
	return strings.TrimSpace(last[1]), percent, true
}

func mirrorPath(root, org, repo string) string {
	return filepath.Join(root, mirrorsDirName, org, repo+gitDirSuffix)
}

// ensureMirror clones a bare mirror on first use, otherwise fetches. Remote heads stay under
// refs/remotes so pruning cannot touch session branches; the clone-seeded local default branch
// is advanced separately after each fetch.
func ensureMirror(ctx context.Context, root, org, repo, url, defaultBranch string, onProgress func(phase string, percent int)) error {
	mp := mirrorPath(root, org, repo)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		if err := cloneMirror(ctx, url, mp, onProgress); err != nil {
			return err
		}
	}
	// Set the remote-tracking-only refspec on every call, not just right after clone: a bare
	// clone configures no fetch refspec, so an interrupted first run (or a failed config step)
	// would otherwise leave a mirror that never populates refs/remotes/origin/* — worktree
	// creation then fails on every resume and never self-heals. This is idempotent.
	if err := git(ctx, dirFlag, mp, configCmd, fetchRefspecKey, fetchRefspec); err != nil {
		return err
	}
	if err := gitSlow(ctx, onProgress, dirFlag, mp, fetchCmd, pruneFlag, progressFlag, remoteName); err != nil {
		return err
	}
	if defaultBranch == "" {
		return nil
	}
	return git(ctx, dirFlag, mp, updateRefCmd, localBranchRef+defaultBranch, remoteRefPrefix+defaultBranch)
}

// cloneMirror clones beside the mirror and renames into place, so a clone that
// is killed part-way leaves no mirror for a later add to take as complete.
func cloneMirror(ctx context.Context, url, mp string, onProgress func(phase string, percent int)) error {
	partial := mp + partialCloneSuffix
	if err := os.RemoveAll(partial); err != nil {
		return err
	}
	err := gitSlow(ctx, onProgress, cloneCmd, bareFlag, progressFlag, url, partial)
	if err == nil {
		err = os.Rename(partial, mp)
	}
	if err != nil {
		_ = os.RemoveAll(partial)
	}
	return err
}

func addWorktree(ctx context.Context, mirror, path, branch, startRef string) error {
	if err := git(ctx, dirFlag, mirror, worktreeCmd, worktreePrune); err != nil {
		return err
	}
	if mirrorHasBranch(ctx, mirror, branch) {
		return gitWorktreeAdd(ctx, mirror, path, branch)
	}
	return gitWorktreeAdd(ctx, mirror, branchFlag, branch, path, startRef)
}

// gitWorktreeAdd records relative paths both ways, so a session root that moves
// or is read on another machine keeps its checkouts.
func gitWorktreeAdd(ctx context.Context, mirror string, args ...string) error {
	err := git(ctx, append([]string{dirFlag, mirror, worktreeCmd, worktreeAdd, relativePathsFlag}, args...)...)
	if err != nil && strings.Contains(err.Error(), unknownRelativePaths) {
		return fmt.Errorf("%w: %w", ErrGitTooOld, err)
	}
	return err
}

// removeHalfMadeWorktree runs on a context of its own, since the add's may be
// the one that was cancelled.
func removeHalfMadeWorktree(mirror, path string) {
	ctx, cancel := context.WithTimeout(context.Background(), worktreeCleanupTimeout)
	defer cancel()
	_ = git(ctx, dirFlag, mirror, worktreeCmd, worktreeRemove, forceFlag, path)
	_ = os.RemoveAll(path)
	_ = git(ctx, dirFlag, mirror, worktreeCmd, worktreePrune)
}

// mirrorHasBranch reports whether qrouton has already cut this session branch in
// the mirror, which is what a resume after a prune finds.
func mirrorHasBranch(ctx context.Context, mirror, branch string) bool {
	return gitOK(ctx, dirFlag, mirror, showRefCmd, verifyFlag, quietLongFlag, localBranchRef+branch)
}
