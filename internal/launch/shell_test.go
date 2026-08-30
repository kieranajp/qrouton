package launch

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

// A shell that exits is done. The tab's own lifecycle takes it from there, and
// another shell is one button away.
func TestShellEndsWhenTheLoginShellExits(t *testing.T) {
	originalTree, originalShell := showShellTree, runLoginShell
	t.Cleanup(func() { showShellTree, runLoginShell = originalTree, originalShell })
	showShellTree = func(string) {}

	exited := exec.Command("/bin/sh", "-c", "exit 1").Run()
	runs := 0
	runLoginShell = func(context.Context, string) error {
		runs++
		if runs > 1 {
			return errors.New("the shell was started again")
		}
		return exited
	}

	if err := Shell(context.Background(), t.TempDir()); !errors.Is(err, exited) {
		t.Fatalf("Shell returned %v, want %v", err, exited)
	}
	if runs != 1 {
		t.Fatalf("login shell ran %d times, want 1", runs)
	}
}
