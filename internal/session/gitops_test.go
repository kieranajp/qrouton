package session

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/gittest"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

const cloneStderr = "Cloning into bare repository 'svc.git'...\n" +
	"remote: Counting objects:  50% (3/6)\r" +
	"remote: Counting objects: 100% (6/6), done.\r\n" +
	"Receiving objects:  47% (1234/2626)\r" +
	"Receiving objects: 100% (2626/2626), 1.10 MiB | 5.00 MiB/s, done.\r\n" +
	"Resolving deltas:  80% (400/500)\r"

type progressUpdate struct {
	phase   string
	percent int
}

type updatePerRead struct{ updates []string }

func (r *updatePerRead) Read(p []byte) (int, error) {
	if len(r.updates) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.updates[0])
	r.updates = r.updates[1:]
	return n, nil
}

func scan(r io.Reader, onProgress func(string, int)) string {
	s := &progressScanner{onProgress: onProgress, lastPercent: -1}
	_, _ = io.Copy(s, r)
	return s.tail()
}

func scanAll(r io.Reader) ([]progressUpdate, string) {
	var got []progressUpdate
	tail := scan(r, func(phase string, percent int) {
		got = append(got, progressUpdate{phase, percent})
	})
	return got, tail
}

func TestScanProgressReportsEachUpdateInGitsOrder(t *testing.T) {
	got, tail := scanAll(&updatePerRead{strings.SplitAfter(cloneStderr, "\r")})
	want := []progressUpdate{
		{"Counting objects", 50}, {"Counting objects", 100},
		{"Receiving objects", 47}, {"Receiving objects", 100},
		{"Resolving deltas", 80},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("progress = %v, want %v", got, want)
	}
	if !strings.Contains(tail, "Cloning into bare repository") {
		t.Fatalf("tail lost git's own output: %q", tail)
	}
}

func TestScanProgressReportsOnlyTheNewestUpdateInOneRead(t *testing.T) {
	got, _ := scanAll(strings.NewReader(cloneStderr))
	if want := []progressUpdate{{"Resolving deltas", 80}}; !slices.Equal(got, want) {
		t.Fatalf("progress = %v, want %v", got, want)
	}
}

// A nil callback is the silent path (resume, tests); it must not panic and must
// still capture the tail for error reporting.
func TestScanProgressToleratesNoConsumer(t *testing.T) {
	tail := scan(strings.NewReader("fatal: Could not read from remote repository.\n"), nil)
	if !strings.Contains(tail, "Could not read from remote") {
		t.Fatalf("tail = %q", tail)
	}
}

// fakeGit puts a shell script named git first on PATH for the rest of the test.
func fakeGit(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, gitBin), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func shortenStalls(t *testing.T, first, next time.Duration) {
	t.Helper()
	savedFirst, savedNext := gitFirstOutputStall, gitOutputStall
	gitFirstOutputStall, gitOutputStall = first, next
	t.Cleanup(func() { gitFirstOutputStall, gitOutputStall = savedFirst, savedNext })
}

// The stall watch hears only what git writes, and git writes progress to a pipe
// only when asked, so a quiet mirror with nobody drawing a bar would be killed.
func TestCloneAndFetchAlwaysAskForProgress(t *testing.T) {
	log := filepath.Join(t.TempDir(), "args")
	t.Setenv("FAKE_GIT_LOG", log)
	fakeGit(t, `echo "$@" >>"$FAKE_GIT_LOG"
if [ "$1" = clone ]; then for a; do target=$a; done; mkdir -p "$target"; fi
`)
	if err := ensureMirror(t.Context(), t.TempDir(), "org", "svc", "git@example.invalid:org/svc.git", "main", nil); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	slow := 0
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		fields := strings.Fields(line)
		if !slices.Contains(fields, cloneCmd) && !slices.Contains(fields, fetchCmd) {
			continue
		}
		slow++
		if !slices.Contains(fields, progressFlag) {
			t.Errorf("git %s ran without %s", line, progressFlag)
		}
	}
	if slow != 2 {
		t.Fatalf("saw %d clones and fetches, want one of each:\n%s", slow, body)
	}
}

func TestASilentGitIsKilledAfterTheStallWindow(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		first, next  time.Duration
		window       string
	}{
		{"before its first output", "sleep 30\n", 200 * time.Millisecond, time.Hour, "nothing in its first 200ms"},
		{"after reporting once", "printf 'Receiving objects:  10%%\\r' >&2\nsleep 30\n", 2 * gitWaitDelay, 200 * time.Millisecond, "no progress for 200ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeGit(t, tc.script)
			shortenStalls(t, tc.first, tc.next)
			start := time.Now()
			err := gitSlow(t.Context(), nil, fetchCmd, progressFlag)
			if !errors.Is(err, ErrGitStalled) {
				t.Fatalf("gitSlow = %v, want a stall", err)
			}
			if !strings.Contains(err.Error(), tc.window) {
				t.Fatalf("stall error %q does not say %q", err, tc.window)
			}
			// sleep is git's child here, so only a group kill frees the pipe sooner.
			if elapsed := time.Since(start); elapsed >= gitWaitDelay {
				t.Fatalf("stopping took %s; the child outlived git", elapsed)
			}
		})
	}
}

func TestAReportingGitOutlivesTheStallWindow(t *testing.T) {
	fakeGit(t, `i=0
while [ $i -lt 40 ]; do printf 'Receiving objects: %d%%\r' $((i * 2)) >&2; sleep 0.05; i=$((i + 1)); done
`)
	// Twice the first window: only the reset on each report keeps git alive.
	shortenStalls(t, time.Second, 300*time.Millisecond)
	var reported int
	if err := gitSlow(t.Context(), func(string, int) { reported++ }, fetchCmd, progressFlag); err != nil {
		t.Fatalf("a git that kept reporting well past the window was stopped: %v", err)
	}
	if reported == 0 {
		t.Fatal("no progress reached the callback")
	}
}

func TestGitRunsWithoutATerminal(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env")
	t.Setenv("FAKE_GIT_OUT", out)
	fakeGit(t, `exec >"$FAKE_GIT_OUT" 2>&1
echo "prompt=$GIT_TERMINAL_PROMPT"
tty
if (: </dev/tty) 2>/dev/null; then echo "controlling=yes"; else echo "controlling=no"; fi
echo "group=$(ps -o pgid= -p $$ | tr -d ' ') self=$$"
`)
	for name, run := range map[string]func() error{
		"git":     func() error { return git(t.Context(), statusCmd) },
		"gitSlow": func() error { return gitSlow(t.Context(), nil, fetchCmd, progressFlag) },
	} {
		if err := run(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		body, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		got := string(body)
		for _, want := range []string{"prompt=0", "not a tty", "controlling=no", groupLeader(got)} {
			if !strings.Contains(got, want) {
				t.Errorf("%s ran git with %q, missing %q", name, got, want)
			}
		}
	}
}

func TestACancelledCloneLeavesNoMirror(t *testing.T) {
	fakeGit(t, `for a; do target=$a; done
mkdir -p "$target/objects"
printf 'Receiving objects:  10%%\r' >&2
sleep 30
`)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err := ensureMirror(ctx, root, "org", "svc", "git@example.invalid:org/svc.git", "main",
		func(string, int) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ensureMirror = %v, want cancelled", err)
	}
	mp := mirrorPath(root, "org", "svc")
	for _, path := range []string{mp, mp + partialCloneSuffix} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("%s survived the cancel: %v", path, err)
		}
	}
}

// A partial clone a crash left behind is replaced, not fetched into.
func TestAStalePartialCloneDoesNotBlockTheMirror(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	mp := mirrorPath(root, "org", "svc")
	gittest.WriteFile(t, mp+partialCloneSuffix, "HEAD", "junk")
	if err := ensureMirror(t.Context(), root, "org", "svc", gittest.Origin(t, "svc"), "main", nil); err != nil {
		t.Fatal(err)
	}
	if !gitOK(t.Context(), dirFlag, mp, showRefCmd, verifyFlag, quietLongFlag, localBranchRef+"main") {
		t.Fatal("the mirror has no main branch")
	}
	if _, err := os.Lstat(mp + partialCloneSuffix); !os.IsNotExist(err) {
		t.Fatalf("the partial clone is still there: %v", err)
	}
}

func TestCurrentBranchReadsADetachedHeadAsNoBranch(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", dir},
		{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "first"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if branch, err := currentBranch(dir); err != nil || branch != "main" {
		t.Fatalf("on main: currentBranch = %q, %v", branch, err)
	}
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "--detach").CombinedOutput(); err != nil {
		t.Fatalf("detach: %v\n%s", err, out)
	}
	if branch, err := currentBranch(dir); err != nil || branch != "" {
		t.Fatalf("detached: currentBranch = %q, %v; want no branch", branch, err)
	}
}

// groupLeader is the line a git that leads its own process group prints, which
// is what lets a cancel kill git and its SSH child together.
func groupLeader(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if self, ok := strings.CutPrefix(line, "group="); ok {
			if _, pid, ok := strings.Cut(self, " self="); ok {
				return "group=" + pid + " self=" + pid
			}
		}
	}
	return "group="
}

// Sessions share mirrors, so two adds of the same new repository must not both
// clone it, nor remove each other's partial clone.
func TestConcurrentAddsCloneASharedMirrorOnce(t *testing.T) {
	log := filepath.Join(t.TempDir(), "clones")
	t.Setenv("FAKE_GIT_LOG", log)
	fakeGit(t, `if [ "$1" = clone ]; then for a; do target=$a; done; echo clone >>"$FAKE_GIT_LOG"; mkdir -p "$target"; sleep 0.3; fi
`)
	root := t.TempDir()
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			errs <- ensureMirror(t.Context(), root, "org", "svc", "git@example.invalid:org/svc.git", "main", nil)
		}()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if body, _ := os.ReadFile(log); strings.Count(string(body), "clone") != 1 {
		t.Fatalf("the mirror was cloned %d times", strings.Count(string(body), "clone"))
	}
}

// A worktree step that fails after git made the checkout must not leave it
// behind, or a retry trips over a directory the manifest never recorded.
func TestAFailedWorktreeStepLeavesNoCheckoutBehind(t *testing.T) {
	requireGit(t)
	real, err := exec.LookPath(gitBin)
	if err != nil {
		t.Fatal(err)
	}
	origin := gittest.Origin(t, "svc")
	t.Setenv("REAL_GIT", real)
	fakeGit(t, `"$REAL_GIT" "$@"; status=$?
if [ "$3 $4" = "worktree add" ]; then exit 1; fi
exit $status
`)
	cfg := &config.Config{Root: t.TempDir()}
	repo := github.Repo{Org: "org", Name: "svc", SSHURL: origin, DefaultBranch: "main"}
	_, err = ComposeRepos(t.Context(), cfg, Manifest{Slug: "s"}, []RepoSelection{{Repo: repo, Role: RepoRoleEditing}}, "feat/s", nil)
	if err == nil {
		t.Fatal("the failing worktree step reported success")
	}
	checkout := filepath.Join(cfg.Root, "s", sessionpaths.SrcDirName, "svc")
	if _, err := os.Lstat(checkout); !os.IsNotExist(err) {
		t.Fatalf("the half-made checkout is still there: %v", err)
	}
	list, err := gitOutput(t.Context(), dirFlag, mirrorPath(cfg.Root, "org", "svc"), worktreeCmd, "list")
	if err != nil || strings.Contains(list, checkout) {
		t.Fatalf("the mirror still lists the checkout: %q, %v", list, err)
	}
}
