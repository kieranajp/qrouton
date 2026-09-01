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

func TestLinearBranchUsesTheTicketKeyAndEditableDescription(t *testing.T) {
	d := Draft{Name: "Stage 2 blocker: Verifier 401s the Gympass partner integration",
		BranchDescription: "verifier-401s-gympass-partner", Entropy: "5756",
		Ticket: "https://linear.app/lifesum/issue/LIF-2841/title", Prefix: "feat"}
	if got := Preview(d); got != "feat/lif-2841-verifier-401s-gympass-partner" {
		t.Fatalf("preview = %q", got)
	}

	d.BranchDescription = ""
	if got := Preview(d); got != "feat/lif-2841" {
		t.Fatalf("key-only preview = %q", got)
	}
}

func TestTicketlessBranchDescriptionKeepsItsEntropy(t *testing.T) {
	d := Draft{Name: "A deliberately long session name", BranchDescription: "short name",
		Entropy: "5756", Prefix: "feat"}
	if got := Preview(d); got != "feat/short-name-5756" {
		t.Fatalf("preview = %q", got)
	}
}

func TestSuggestBranchDescriptionUsesTheSpecificTitleClause(t *testing.T) {
	for title, want := range map[string]string{
		"Stage 2 blocker: Verifier 401s the Gympass partner integration":  "verifier-401s-gympass-partner",
		"Stage 2 blocker — Verifier 401s the Gympass partner integration": "verifier-401s-gympass-partner",
		"Stage 2 blocker - Verifier 401s the Gympass partner integration": "verifier-401s-gympass-partner",
		"Stage 2 blocker verifier 401s the Gympass partner integration":   "stage-2-blocker-verifier",
	} {
		if got := SuggestBranchDescription(title); got != want {
			t.Errorf("SuggestBranchDescription(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestSuggestBranchDescriptionRespectsItsLengthBudget(t *testing.T) {
	if got := SuggestBranchDescription("Authentication authorization configuration cleanup"); got != "authentication-authorization" {
		t.Fatalf("suggestion = %q", got)
	}
	if got := SuggestBranchDescription("Pneumonoultramicroscopicsilicovolcanoconiosis"); len(got) != branchDescriptionMaxLength {
		t.Fatalf("single-word suggestion = %q", got)
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
