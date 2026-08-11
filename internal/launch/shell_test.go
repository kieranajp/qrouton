package launch

import (
	"context"
	"testing"
)

func stubShellProcess(t *testing.T) *int {
	t.Helper()
	originalTree, originalShell := showShellTree, runLoginShell
	runs := 0
	showShellTree = func(string) {}
	runLoginShell = func(ctx context.Context, _ string) error {
		runs++
		return ctx.Err()
	}
	t.Cleanup(func() {
		showShellTree, runLoginShell = originalTree, originalShell
	})
	return &runs
}

// The affordance is having a shell at all, so exiting one starts the next.
func TestShellRestartsAfterTheLoginShellExits(t *testing.T) {
	runs := stubShellProcess(t)
	ctx, cancel := context.WithCancel(context.Background())
	originalShell := runLoginShell
	runLoginShell = func(c context.Context, dir string) error {
		err := originalShell(c, dir)
		if *runs == 2 {
			cancel()
		}
		return err
	}
	if err := Shell(ctx, t.TempDir()); err != context.Canceled {
		t.Fatalf("Shell returned %v, want the cancelled context", err)
	}
	if *runs != 2 {
		t.Fatalf("login shell ran %d times, want 2", *runs)
	}
}
