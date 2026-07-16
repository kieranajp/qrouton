package session

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
)

func makeOrigin(t *testing.T, name string) (string, string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), name)
	run(t, "", "init", "-b", "main", origin)
	os.WriteFile(filepath.Join(origin, "version"), []byte("one"), 0o644)
	run(t, origin, "add", ".")
	run(t, origin, "commit", "-m", "initial")
	out, err := exec.Command("git", "-C", origin, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return origin, strings.TrimSpace(string(out))
}

func TestCreateSessionWithActiveAndPinnedReference(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	activeOrigin, _ := makeOrigin(t, "active")
	referenceOrigin, pinned := makeOrigin(t, "reference")
	cfg := &config.Config{Root: root}
	dir, err := createSessionWithRoles(cfg, "Role test", "", "", "feat", []RepoSelection{
		{Repo: github.Repo{Name: "active", Org: "org", SSHURL: activeOrigin, DefaultBranch: "main"}, Role: RepoRoleActive},
		{Repo: github.Repo{Name: "reference", Org: "org", SSHURL: referenceOrigin, DefaultBranch: "main"}, Role: RepoRoleReference},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.SchemaVersion != manifestSchemaVersion {
		t.Fatalf("schema version = %d", m.SchemaVersion)
	}
	if got := m.Repos[0]; got.Role != RepoRoleActive || got.Branch != "feat/role-test" || got.Revision != "" {
		t.Fatalf("active manifest repo = %+v", got)
	}
	if got := m.Repos[1]; got.Role != RepoRoleReference || got.Branch != "" || got.Revision != pinned {
		t.Fatalf("reference manifest repo = %+v", got)
	}
	if gitOK("-C", filepath.Join(dir, "src", "reference"), "symbolic-ref", "-q", "HEAD") {
		t.Fatal("reference checkout is not detached")
	}

	// Advance the remote, remove the worktree, and prove resume uses the recorded SHA.
	os.WriteFile(filepath.Join(referenceOrigin, "version"), []byte("two"), 0o644)
	run(t, referenceOrigin, "add", ".")
	run(t, referenceOrigin, "commit", "-m", "advance")
	os.RemoveAll(filepath.Join(dir, "src", "reference"))
	if err := EnsureWorktrees(cfg, m); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", filepath.Join(dir, "src", "reference"), "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != pinned {
		t.Fatalf("resumed reference at %s, want pinned %s", got, pinned)
	}
}

func TestMissingManifestRoleResumesAsActive(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	origin, _ := makeOrigin(t, "legacy")
	if err := ensureMirror(root, "org", "legacy", origin); err != nil {
		t.Fatal(err)
	}
	m := Manifest{SchemaVersion: 1, Slug: "old", Repos: []ManifestRepo{{
		Name: "legacy", Org: "org", Branch: "feat/old", DefaultBranch: "main", WorktreePath: "src/legacy",
	}}}
	if err := EnsureWorktrees(&config.Config{Root: root}, m); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(root, "old", "src", "legacy")
	out, err := exec.Command("git", "-C", wt, "branch", "--show-current").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "feat/old" {
		t.Fatalf("legacy checkout branch = %q", got)
	}
}

func TestCreateFailureCleansNewSessionDirectoryAndAllowsRetry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "retry")
	repo := github.Repo{Name: "retry", Org: "org", SSHURL: origin, DefaultBranch: "main"}
	bad := []RepoSelection{{Repo: repo, Role: RepoRole("invalid")}}
	if _, err := createSessionWithRoles(&config.Config{Root: root}, "Retry me", "", "", "feat", bad); err == nil {
		t.Fatal("invalid role unexpectedly succeeded")
	}
	dir := filepath.Join(root, "retry-me")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("failed session directory was not cleaned up: %v", err)
	}
	if _, err := createSessionWithRoles(&config.Config{Root: root}, "Retry me", "", "", "feat",
		[]RepoSelection{{Repo: repo, Role: RepoRoleActive}}); err != nil {
		t.Fatal("retry failed:", err)
	}
}

func TestSessionProgressReportsAssemblyOperations(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "progress")
	var events []Progress
	_, err := Create(&config.Config{Root: root}, "Progress", "", "", "feat",
		[]RepoSelection{{Repo: github.Repo{Name: "progress", Org: "org", SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleActive}},
		func(event Progress) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		step   ProgressStep
		status ProgressStatus
	}{
		{ProgressMirror, ProgressStarted}, {ProgressMirror, ProgressCompleted},
		{ProgressWorktree, ProgressStarted}, {ProgressWorktree, ProgressCompleted},
		{ProgressScaffold, ProgressStarted}, {ProgressScaffold, ProgressCompleted},
		{ProgressManifest, ProgressStarted}, {ProgressManifest, ProgressCompleted},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i := range want {
		if events[i].Step != want[i].step || events[i].Status != want[i].status {
			t.Fatalf("event %d = %+v, want step=%s status=%s", i, events[i], want[i].step, want[i].status)
		}
	}
	if events[0].Repo == nil || events[0].Repo.Name != "progress" || events[0].Role != RepoRoleActive {
		t.Fatalf("repo progress lacks context: %+v", events[0])
	}
}
