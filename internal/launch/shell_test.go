package launch

import (
	"context"
	"errors"
	"testing"
)

type fakeShellStack struct {
	joined int
	counts []int
	err    error
}

func (f *fakeShellStack) JoinCurrent(context.Context, string, string) (int, error) {
	f.joined++
	return 1, f.err
}

func (f *fakeShellStack) Count(context.Context, string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	count := f.counts[0]
	f.counts = f.counts[1:]
	return count, nil
}

func stubShellProcess(t *testing.T) *int {
	t.Helper()
	originalTree, originalShell := showShellTree, runLoginShell
	runs := 0
	showShellTree = func(string) {}
	runLoginShell = func(context.Context, string) error {
		runs++
		return nil
	}
	t.Cleanup(func() {
		showShellTree, runLoginShell = originalTree, originalShell
	})
	return &runs
}

func TestShellClosesWhenAnotherShellRemains(t *testing.T) {
	runs := stubShellProcess(t)
	stack := &fakeShellStack{counts: []int{2}}
	if err := Shell(context.Background(), t.TempDir(), stack); err != nil {
		t.Fatal(err)
	}
	if stack.joined != 1 || *runs != 1 {
		t.Fatalf("joined=%d shell runs=%d, want 1 and 1", stack.joined, *runs)
	}
}

func TestShellRestartsInsteadOfRemovingTheFinalPane(t *testing.T) {
	runs := stubShellProcess(t)
	stack := &fakeShellStack{counts: []int{1, 2}}
	if err := Shell(context.Background(), t.TempDir(), stack); err != nil {
		t.Fatal(err)
	}
	if *runs != 2 {
		t.Fatalf("login shell ran %d times, want 2", *runs)
	}
}

func TestShellStopsWhenItCannotJoinTheStack(t *testing.T) {
	stubShellProcess(t)
	stack := &fakeShellStack{err: errors.New("no pane")}
	if err := Shell(context.Background(), t.TempDir(), stack); err == nil {
		t.Fatal("shell started outside its managed stack")
	}
}
