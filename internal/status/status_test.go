package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
)

// sessionDir writes a session directory with the given manifest under a fake
// qrouton root and returns it — the chrome only ever reads.
func sessionDir(t *testing.T, root string, m session.Manifest) string {
	t.Helper()
	dir := filepath.Join(root, m.Slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qrouton.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReadScratchAssistant(t *testing.T) {
	dir := sessionDir(t, t.TempDir(), session.Manifest{Slug: "lifesum-4f3a", Mode: session.ModeAssistant})
	got, ok := Read(dir)
	if !ok {
		t.Fatal("Read reported no manifest")
	}
	want := Fields{Mode: "ASSISTANT", Phase: "scratch", Identity: "lifesum-4f3a"}
	if got != want {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
}

func TestReadEscalatedShowsPhaseNameAndBranch(t *testing.T) {
	root := t.TempDir()
	m := session.Manifest{Slug: "webhook", Name: "webhook retry backoff", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{
			{Name: "consumer", Org: "org", Role: session.RepoRoleReference, Revision: "41c3", WorktreePath: "src/consumer"},
			{Name: "svc", Org: "org", Branch: "fix/webhook-retry-backoff", WorktreePath: "src/svc"},
		}}
	dir := sessionDir(t, root, m)

	// No documents yet: freshly escalated means Research.
	got, ok := Read(dir)
	if !ok {
		t.Fatal("Read reported no manifest")
	}
	want := Fields{Mode: "RPI", Phase: "Research", Identity: "webhook retry backoff · fix/webhook-retry-backoff"}
	if got != want {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}

	// A research document moves the session into planning.
	research := filepath.Join(dir, "thoughts", "shared", "research")
	if err := os.MkdirAll(research, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(research, "r1-findings.md"), []byte("# findings"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Read(dir); got.Phase != "Plan" {
		t.Fatalf("phase did not advance to Plan: %q", got.Phase)
	}

	// A plan document moves it into implementation.
	plans := filepath.Join(dir, "thoughts", "shared", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "p1.md"), []byte("- [ ] step"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Read(dir); got.Phase != "Implement" {
		t.Fatalf("phase did not advance to Implement: %q", got.Phase)
	}
}

func TestReadWithoutManifest(t *testing.T) {
	got, ok := Read(t.TempDir())
	if ok || got != (Fields{}) {
		t.Fatalf("missing manifest = %#v, %v", got, ok)
	}
}
