package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
)

// sessionDir writes a session directory with the given manifest under a fake
// qrouton root and returns it — the strip only ever reads.
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

func TestStatusLinesScratchAssistant(t *testing.T) {
	dir := sessionDir(t, t.TempDir(), session.Manifest{Slug: "lifesum-4f3a", Mode: session.ModeAssistant})
	lines := statusLines(dir)
	if len(lines) != 1 {
		t.Fatalf("strip = %d lines, want one:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for _, want := range []string{"ASSISTANT", "scratch", "lifesum-4f3a", "Alt-e escalate", "Alt-g terminal"} {
		if !strings.Contains(lines[0], want) {
			t.Fatalf("strip missing %q: %q", want, lines[0])
		}
	}
	if strings.Contains(lines[0], "Alt-n") {
		t.Fatalf("assistant strip advertises de-escalation: %q", lines[0])
	}
}

func TestStatusLinesEscalatedShowsPhaseNameAndBranch(t *testing.T) {
	root := t.TempDir()
	m := session.Manifest{Slug: "webhook", Name: "webhook retry backoff", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{
			{Name: "consumer", Org: "org", Role: session.RepoRoleReference, Revision: "41c3", WorktreePath: "src/consumer"},
			{Name: "svc", Org: "org", Branch: "fix/webhook-retry-backoff", WorktreePath: "src/svc"},
		}}
	dir := sessionDir(t, root, m)

	// No documents yet: freshly escalated means Research.
	line := statusLines(dir)[0]
	for _, want := range []string{"RPI", "Research", "webhook retry backoff", "fix/webhook-retry-backoff",
		"Alt-n de-escalate", "Alt-e add repos"} {
		if !strings.Contains(line, want) {
			t.Fatalf("strip missing %q: %q", want, line)
		}
	}
	// Alt-e stays bound in RPI, but the work is already assembled: it must not
	// read as an offer to escalate again.
	if strings.Contains(line, "Alt-e escalate") {
		t.Fatalf("escalated strip still advertises escalation: %q", line)
	}

	// A research document moves the session into planning.
	research := filepath.Join(dir, "thoughts", "shared", "research")
	if err := os.MkdirAll(research, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(research, "r1-findings.md"), []byte("# findings"), 0o644); err != nil {
		t.Fatal(err)
	}
	if line := statusLines(dir)[0]; !strings.Contains(line, "Plan") {
		t.Fatalf("strip did not advance to Plan: %q", line)
	}

	// A plan document moves it into implementation.
	plans := filepath.Join(dir, "thoughts", "shared", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "p1.md"), []byte("- [ ] step"), 0o644); err != nil {
		t.Fatal(err)
	}
	if line := statusLines(dir)[0]; !strings.Contains(line, "Implement") {
		t.Fatalf("strip did not advance to Implement: %q", line)
	}
}

func TestStatusLinesWithoutManifest(t *testing.T) {
	lines := statusLines(t.TempDir())
	if len(lines) != 1 || !strings.Contains(lines[0], "no session manifest") {
		t.Fatalf("missing-manifest strip = %#v", lines)
	}
}
