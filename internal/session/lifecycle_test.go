package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStatusInfersRPIFromDocuments(t *testing.T) {
	root := t.TempDir()
	m := Manifest{Slug: "checkout"}
	shared := filepath.Join(root, m.Slug, "thoughts", "shared")
	for _, dir := range []string{"research", "plans"} {
		if err := os.MkdirAll(filepath.Join(shared, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	research := filepath.Join(shared, "research", "R1-retry.md")
	os.WriteFile(research, []byte("# Retry\n\n## Summary\n\nWhat is being looked at.\n\n## How does it retry?\n\n> Start in retry.go.\n"), 0o644)
	os.WriteFile(filepath.Join(shared, "plans", "P1.md"), []byte("- [x] first\n- [ ] second\n"), 0o644)
	got := Status(root, m)
	if got.Research || !got.Plan || got.Implement {
		t.Fatalf("framed research and partial plan status = %#v", got)
	}
	os.WriteFile(research, []byte("# Retry\n\n## Summary\n\nIt backs off three times.\n\n## How does it retry?\n\nThree attempts, doubling the wait.\n"), 0o644)
	os.WriteFile(filepath.Join(shared, "plans", "P1.md"), []byte("- [x] first\n- [x] second\n"), 0o644)
	got = Status(root, m)
	if !got.Research || !got.Plan || !got.Implement {
		t.Fatalf("answered document status = %#v", got)
	}
}

// Sessions older than the single document keep their questions in a file named
// for them and their findings in prose that opens no sections at all.
func TestStatusReadsResearchWrittenAsAPair(t *testing.T) {
	root := t.TempDir()
	m := Manifest{Slug: "checkout"}
	research := filepath.Join(root, m.Slug, "thoughts", "shared", "research")
	if err := os.MkdirAll(research, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(research, "R1-retry-questions.md"), []byte("# Retry questions\n\n- Where is cancellation enforced?\n"), 0o644)
	if got := Status(root, m); got.Research {
		t.Fatalf("a questions file alone read as research: %#v", got)
	}

	os.WriteFile(filepath.Join(research, "R1-retry.md"), []byte("# Retry\n\nThe service bounds attempts but takes no context.\n"), 0o644)
	if got := Status(root, m); !got.Research {
		t.Fatalf("prose findings did not read as research: %#v", got)
	}
}

func TestDirtyWorktreesAndDelete(t *testing.T) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	mirror := mirrorPath(root, "acme", "api")
	worktree := filepath.Join(root, "checkout", "src", "api")
	runGitTest(t, "init", "-b", "main", seed)
	runGitTest(t, "-C", seed, "config", "user.name", "Qrouton Test")
	runGitTest(t, "-C", seed, "config", "user.email", "qrouton@example.test")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, "-C", seed, "add", "README.md")
	runGitTest(t, "-C", seed, "commit", "-m", "seed")
	if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, "clone", "--bare", seed, mirror)
	if err := os.MkdirAll(filepath.Dir(worktree), 0o755); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, "-C", mirror, "worktree", "add", worktree, "main")
	if err := os.WriteFile(filepath.Join(worktree, "dirty.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := Manifest{Slug: "checkout", Repos: []ManifestRepo{{Org: "acme", Name: "api", WorktreePath: filepath.Join("src", "api")}}}
	dirty, err := DirtyWorktrees(root, m)
	if err != nil || len(dirty) != 1 || dirty[0] != "acme/api" {
		t.Fatalf("dirty worktrees = %v, %v", dirty, err)
	}
	if err := Delete(root, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "checkout")); !os.IsNotExist(err) {
		t.Fatalf("session still exists after delete: %v", err)
	}
	if _, err := os.Stat(mirror); err != nil {
		t.Fatalf("shared mirror should remain: %v", err)
	}
}

func TestDeleteContinuesWhenWorktreeMetadataIsBroken(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "checkout")
	worktree := filepath.Join(dir, "src", "api")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+filepath.Join(root, ".mirrors", "acme", "api.git", "worktrees", "api")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "unreadable-by-git.txt"), []byte("stale checkout\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Slug: "checkout", Repos: []ManifestRepo{{Org: "acme", Name: "api", WorktreePath: filepath.Join("src", "api")}}}

	dirty, err := DirtyWorktrees(root, m)
	if err != nil || len(dirty) != 0 {
		t.Fatalf("broken worktree dirty check = %v, %v", dirty, err)
	}
	if err := Delete(root, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("session still exists after fallback delete: %v", err)
	}
}

func runGitTest(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
