package session

import (
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
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

func scanAll(r io.Reader) ([]progressUpdate, string) {
	var got []progressUpdate
	tail := scanProgress(r, func(phase string, percent int) {
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
	tail := scanProgress(strings.NewReader("fatal: Could not read from remote repository.\n"), nil)
	if !strings.Contains(tail, "Could not read from remote") {
		t.Fatalf("tail = %q", tail)
	}
}

func TestVerbosityFlagAsksForProgressOnlyWhenDrawn(t *testing.T) {
	// Without --progress git stays silent on a pipe, so a drawn bar would never move.
	if got := verbosityFlag(func(string, int) {}); got != progressFlag {
		t.Fatalf("verbosityFlag with a consumer = %q, want %q", got, progressFlag)
	}
	if got := verbosityFlag(nil); got != quietLongFlag {
		t.Fatalf("verbosityFlag without a consumer = %q, want %q", got, quietLongFlag)
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
