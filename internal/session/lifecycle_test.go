package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

// The manifest lands last, so a directory without one is a session still being
// assembled or one whose row has outlived it. Either way there is nothing to
// resume.
func TestResumableNamesOnlyADirectoryHoldingAManifest(t *testing.T) {
	root := t.TempDir()
	half := filepath.Join(root, "octopus")
	if err := os.MkdirAll(half, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Resumable(root, "octopus"); got != "" {
		t.Fatalf("a directory with no manifest resumed as %q", got)
	}
	if err := WriteManifest(half, Manifest{Slug: "octopus", Mode: ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	if got := Resumable(root, "octopus"); got != half {
		t.Fatalf("Resumable = %q, want %q", got, half)
	}
	for _, tc := range [][2]string{{"", "octopus"}, {root, ""}, {root, "never-existed"}} {
		if got := Resumable(tc[0], tc[1]); got != "" {
			t.Fatalf("Resumable(%q, %q) = %q, want nothing", tc[0], tc[1], got)
		}
	}
}

// Remove resolves its target from the manifest, so a directory holding another
// session's manifest would take that session's worktrees.
func TestRemoveRefusesADirectoryHoldingAnotherSessionsManifest(t *testing.T) {
	root := t.TempDir()
	kraken := filepath.Join(root, "kraken")
	stray := filepath.Join(root, "webhook")
	for _, dir := range []string{kraken, stray} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{kraken, stray} {
		if err := WriteManifest(dir, Manifest{Slug: "kraken", Mode: ModeAssistant}); err != nil {
			t.Fatal(err)
		}
	}

	err := Remove(root, stray)
	if err == nil {
		t.Fatal("a directory holding another session's manifest was removed anyway")
	}
	for _, name := range []string{"webhook", "kraken"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("refusal %q names neither the directory asked for nor the one its manifest claims", err)
		}
	}
	for _, dir := range []string{kraken, stray} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("%s was removed by the refused cleanup: %v", dir, err)
		}
	}

	if err := Remove(root, kraken); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(kraken); !os.IsNotExist(err) {
		t.Fatal("the session naming its own directory survived its removal")
	}
}

// Uncommitted reads the session's own manifest, so a directory with none is a
// refusal rather than a clean answer.
func TestUncommittedRefusesADirectoryWithNoManifest(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "octopus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Uncommitted(root, dir); err == nil {
		t.Fatal("a directory with no manifest reported its repositories clean")
	}
	if err := WriteManifest(dir, Manifest{Slug: "octopus", Mode: ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	dirty, err := Uncommitted(root, dir)
	if err != nil || len(dirty) != 0 {
		t.Fatalf("a session holding no repositories = %v, %v", dirty, err)
	}
}

func runGitTest(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestOutlivedCheckoutRecognisesEachGitVersionsMessage(t *testing.T) {
	for _, output := range []string{
		"fatal: not a git repository: /tmp/checkout/src/api/../.mirrors/acme/api.git/worktrees/api\n",
		"fatal: gitfile does not point to a valid repository: /tmp/checkout/src/api/.git\n",
	} {
		if !outlivedCheckout(output) {
			t.Errorf("outlivedCheckout(%q) = false", output)
		}
	}
	if outlivedCheckout("fatal: detected dubious ownership in repository at '/tmp/checkout/src/api'\n") {
		t.Error("dubious ownership read as an outlived checkout")
	}
}

func TestDeleteLeavesADirectoryOutsideTheSessionAlone(t *testing.T) {
	root := t.TempDir()
	victim := filepath.Join(root, "victim")
	if err := os.MkdirAll(filepath.Join(root, "s"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Slug: "s", Repos: []ManifestRepo{{Org: "acme", Name: "api", WorktreePath: "../victim"}}}
	if err := Delete(root, m); !errors.Is(err, ErrCheckoutOutsideSrc) {
		t.Fatalf("Delete = %v, want ErrCheckoutOutsideSrc", err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("a directory outside the session was removed: %v", err)
	}
}
