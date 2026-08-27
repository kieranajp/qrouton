package session

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func git(args ...string) error {
	out, err := exec.Command(gitBin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

func gitOK(args ...string) bool { return exec.Command(gitBin, args...).Run() == nil }

// progressPattern matches git's own progress reports — "Receiving objects:  47%
// (1234/2626)" — which it writes to stderr in carriage-return-separated
// updates. The phase name is captured too: a clone runs through four of them,
// and a bar that silently restarts three times reads as a stuck one.
var progressPattern = regexp.MustCompile(`([A-Za-z][A-Za-z ]*):\s+(\d+)%`)

// gitSlow runs a long git command (clone, fetch) with its stderr captured
// rather than sprayed over the terminal — which in the fullscreen TUI meant
// scribbling over the alt screen. Parsed phase and percentage go to onProgress;
// a nil onProgress runs the command silently.
//
// Two details are load-bearing. Git suppresses progress entirely when stderr
// is not a terminal, which it no longer is, so verbosityFlag must ask for it
// explicitly. And stdin stays attached: ssh prompts for a passphrase or an
// unknown host key on the tty, and swallowing that would turn a prompt into a
// hang. The captured tail rides along in any error so an auth or host-key
// failure is still diagnosable.
func gitSlow(onProgress func(phase string, percent int), args ...string) error {
	cmd := exec.Command(gitBin, args...)
	cmd.Stdin = os.Stdin
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	tail := scanProgress(stderr, onProgress)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, tail)
	}
	return nil
}

// verbosityFlag asks git for machine-readable progress when someone is
// rendering it, and for silence otherwise. Callers place it themselves so it
// lands before a positional argument.
func verbosityFlag(onProgress func(phase string, percent int)) string {
	if onProgress == nil {
		return quietLongFlag
	}
	return progressFlag
}

// scanProgress drains r, reporting each change of phase or percentage, and
// returns what it read (capped) for error reporting. Reads raw rather than by
// line: git separates updates with carriage returns, so a line scanner would
// deliver one enormous line at the end and report nothing until then.
func scanProgress(r io.Reader, onProgress func(phase string, percent int)) string {
	var tail strings.Builder
	buf := make([]byte, progressChunkBytes)
	lastPhase, lastPercent, lastEmit := "", -1, time.Time{}
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			if tail.Len() < progressTailLimit {
				tail.WriteString(chunk)
			}
			if onProgress != nil {
				if phase, percent, ok := lastProgress(chunk); ok && (phase != lastPhase || percent != lastPercent) {
					// Git reports hundreds of times a second on a large
					// repository. Rate-limit here rather than in the renderer:
					// every consumer would otherwise have to, and one of them
					// reads these off a channel that would throttle the clone
					// to the frame rate. Phase changes and completion always
					// pass, so no bar sticks short of full.
					if percent == progressComplete || phase != lastPhase ||
						time.Since(lastEmit) >= progressEmitInterval {
						lastEmit = time.Now()
						onProgress(phase, percent)
					}
					lastPhase, lastPercent = phase, percent
				}
			}
		}
		if readErr != nil {
			return strings.TrimSpace(tail.String())
		}
	}
}

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

// ensureMirror clones a bare mirror on first use, otherwise fetches. The fetch refspec maps origin's
// heads to refs/remotes/origin/* only, so --prune can never touch session branches under refs/heads/*.
// (A literal --mirror clone's +refs/*:refs/* refspec would prune them — the hazard mirror_test guards.)
func ensureMirror(root, org, repo, url string, onProgress func(phase string, percent int)) error {
	mp := mirrorPath(root, org, repo)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		if err := gitSlow(onProgress, cloneCmd, bareFlag, verbosityFlag(onProgress), url, mp); err != nil {
			return err
		}
	}
	// Set the remote-tracking-only refspec on every call, not just right after clone: a bare
	// clone configures no fetch refspec, so an interrupted first run (or a failed config step)
	// would otherwise leave a mirror that never populates refs/remotes/origin/* — worktree
	// creation then fails on every resume and never self-heals. This is idempotent.
	if err := git(dirFlag, mp, configCmd, fetchRefspecKey, fetchRefspec); err != nil {
		return err
	}
	return gitSlow(onProgress, dirFlag, mp, fetchCmd, pruneFlag, verbosityFlag(onProgress), remoteName)
}

// addWorktree materialises a worktree at path: on the existing session branch if it exists
// (resume after prune), otherwise on a new branch off the freshly-fetched origin default branch.
func addWorktree(mirror, path, branch, startRef string) error {
	if err := git(dirFlag, mirror, worktreeCmd, worktreePrune); err != nil {
		return err
	}
	if mirrorHasBranch(mirror, branch) {
		return git(dirFlag, mirror, worktreeCmd, worktreeAdd, path, branch)
	}
	return git(dirFlag, mirror, worktreeCmd, worktreeAdd, branchFlag, branch, path, startRef)
}

// mirrorHasBranch reports whether qrouton has already cut this session branch in
// the mirror, which is what a resume after a prune finds.
func mirrorHasBranch(mirror, branch string) bool {
	return gitOK(dirFlag, mirror, showRefCmd, verifyFlag, quietLongFlag, localBranchRef+branch)
}
