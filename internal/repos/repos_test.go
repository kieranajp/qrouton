package repos

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-qm", "init")
}

func writeManifest(t *testing.T, root string, m session.Manifest) {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "qrouton.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusLinesRenderBranchRoleAndDirtyState(t *testing.T) {
	root := t.TempDir()

	active := filepath.Join(root, "src", "api")
	initRepo(t, active)
	if err := os.WriteFile(filepath.Join(active, "dirty"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	reference := filepath.Join(root, "src", "lib")
	initRepo(t, reference)
	run(t, reference, "checkout", "-q", "--detach")

	writeManifest(t, root, session.Manifest{Repos: []session.ManifestRepo{
		{Name: "api", Org: "org", WorktreePath: "src/api"},
		{Name: "lib", Org: "org", Role: session.RepoRoleReference, WorktreePath: "src/lib"},
		{Name: "gone", Org: "org", WorktreePath: "src/gone"},
	}})

	lines := statusLines(root)
	if len(lines) != 4 {
		t.Fatalf("lines = %d, want header + 3 repos:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[1], "src/api") || !strings.Contains(lines[1], "main") || !strings.Contains(lines[1], "1 changed") {
		t.Fatalf("active repo line = %q", lines[1])
	}
	// The old status.sh printed a blank branch for detached references; the
	// output must show the pinned revision and the role instead.
	if !strings.Contains(lines[2], "@ ") || !strings.Contains(lines[2], "reference · clean") {
		t.Fatalf("reference repo line = %q", lines[2])
	}
	if !strings.Contains(lines[3], "src/gone") || !strings.Contains(lines[3], "missing") {
		t.Fatalf("missing worktree line = %q", lines[3])
	}
	for _, line := range lines {
		if strings.Contains(line, "\x1b") {
			t.Fatalf("line carries escape sequences: %q", line)
		}
	}
}

func TestStatusLinesRenderEmptyStateForZeroRepoManifest(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, session.Manifest{Slug: "scratch-4f3a"})
	lines := statusLines(root)
	if len(lines) != 3 || !strings.Contains(lines[1], "no repositories yet") || !strings.Contains(lines[2], "add them from the workbench") {
		t.Fatalf("zero-repo lines = %#v", lines)
	}
}

func TestStatusLinesWithoutManifest(t *testing.T) {
	lines := statusLines(t.TempDir())
	if len(lines) != 2 || !strings.Contains(lines[1], "No session manifest") {
		t.Fatalf("missing-manifest lines = %#v", lines)
	}
}

func TestStatusLinesSurviveBrokenRepository(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src", "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, root, session.Manifest{Repos: []session.ManifestRepo{
		{Name: "broken", Org: "org", WorktreePath: "src/broken"},
	}})
	lines := statusLines(root)
	if len(lines) != 2 || !strings.Contains(lines[1], "unavailable") {
		t.Fatalf("broken repo lines = %#v", lines)
	}
}
