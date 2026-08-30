package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/gittest"
)

// A --mirror-style +refs/*:refs/* refspec would prune session branches on the next
// fetch, silently breaking earlier sessions' worktrees. The bare clone's
// remote-tracking-only refspec leaves them alone.
func TestFetchAfterSessionBranchDoesNotPrune(t *testing.T) {
	tmp := t.TempDir()

	// local "origin" with a commit on main and a doomed extra branch
	origin := filepath.Join(tmp, "origin")
	gittest.Run(t, "", "init", "-b", "main", origin)
	os.WriteFile(filepath.Join(origin, "f"), []byte("x"), 0o644)
	gittest.Run(t, origin, "add", ".")
	gittest.Run(t, origin, "commit", "-m", "init")
	gittest.Run(t, origin, "branch", "doomed")

	root := filepath.Join(tmp, "root")
	if err := ensureMirror(root, "org", "repo", origin, nil); err != nil {
		t.Fatal(err)
	}
	mp := mirrorPath(root, "org", "repo")
	if err := addWorktree(mp, filepath.Join(tmp, "wt"), "feat/session", "origin/main"); err != nil {
		t.Fatal(err)
	}

	// origin deletes a branch; our prune-fetch must drop its remote ref but keep the session branch
	gittest.Run(t, origin, "branch", "-D", "doomed")
	if err := ensureMirror(root, "org", "repo", origin, nil); err != nil {
		t.Fatal(err)
	}

	if !gitOK("-C", mp, "show-ref", "--verify", "--quiet", "refs/heads/feat/session") {
		t.Fatal("session branch pruned by fetch — refspec is wrong")
	}
	if gitOK("-C", mp, "show-ref", "--verify", "--quiet", "refs/remotes/origin/doomed") {
		t.Fatal("deleted origin branch not pruned — --prune not working")
	}
}

// Resume path: a pruned worktree re-materialises on the existing branch, keeping its commits.
func TestWorktreeRematerialisesOnExistingBranch(t *testing.T) {
	tmp := t.TempDir()
	origin := filepath.Join(tmp, "origin")
	gittest.Run(t, "", "init", "-b", "main", origin)
	os.WriteFile(filepath.Join(origin, "f"), []byte("x"), 0o644)
	gittest.Run(t, origin, "add", ".")
	gittest.Run(t, origin, "commit", "-m", "init")

	root := filepath.Join(tmp, "root")
	if err := ensureMirror(root, "org", "repo", origin, nil); err != nil {
		t.Fatal(err)
	}
	mp := mirrorPath(root, "org", "repo")
	wt := filepath.Join(tmp, "wt")
	if err := addWorktree(mp, wt, "feat/session", "origin/main"); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(wt, "work"), []byte("y"), 0o644)
	gittest.Run(t, wt, "add", ".")
	gittest.Run(t, wt, "commit", "-m", "session work")

	os.RemoveAll(wt) // simulate a pruned/deleted worktree
	if err := addWorktree(mp, wt, "feat/session", "origin/main"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt, "work")); err != nil {
		t.Fatal("session commit lost on re-materialise:", err)
	}
}
