package assembly

import (
	"slices"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
)

func TestPreviewDerivesTheBranchFromThePrefixAndTheSlug(t *testing.T) {
	if got := Preview(Draft{Name: "API Cleanup!", Prefix: "fix"}); got != "fix/api-cleanup" {
		t.Fatalf("preview = %q", got)
	}
	if got := Preview(Draft{Name: "API Cleanup!", Entropy: "4f3a", Prefix: "fix"}); got != "fix/api-cleanup-4f3a" {
		t.Fatalf("entropic preview = %q", got)
	}
}

// An existing session keeps its branch; only a first assembly derives one.
func TestBranchForKeepsTheSessionsOwnBranch(t *testing.T) {
	m := session.Manifest{Repos: []session.ManifestRepo{
		{Name: "svc", Org: "org", Role: session.RepoRoleEditing, Branch: "fix/webhook-retry"},
	}}
	d := Draft{Name: "Webhook retry", Prefix: "feat"}
	if got := branchFor(m, d); got != "fix/webhook-retry" {
		t.Fatalf("added repos would go on %q", got)
	}
	if got := branchFor(session.Manifest{}, d); got != "feat/webhook-retry" {
		t.Fatalf("a session with no branch derived %q", got)
	}
}

// The MCP escalate tool describes this list in a struct tag, which cannot
// interpolate a slice — so the two are held together here instead.
func TestPrefixesLeadWithFeat(t *testing.T) {
	got := Prefixes()
	if len(got) == 0 || got[0] != "feat" {
		t.Fatalf("prefixes = %v", got)
	}
	if slices.Contains(got, "") {
		t.Fatalf("prefixes carry a blank: %v", got)
	}
}

func TestInstalledKeepsOnlyRunnersWithAResolvedPath(t *testing.T) {
	got := Installed([]Runner{
		{ID: "claude", Label: "Claude Code", Installed: true},
		{ID: "codex", Label: "Codex CLI"},
		{ID: "opencode", Label: "OpenCode", Installed: true},
	})
	if len(got) != 2 || got[0].ID != "claude" || got[1].ID != "opencode" {
		t.Fatalf("installed runners = %+v", got)
	}
}
