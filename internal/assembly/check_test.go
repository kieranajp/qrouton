package assembly

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
)

func repoNamed(name string) github.Repo {
	return github.Repo{Org: "acme", Name: name, DefaultBranch: "main"}
}

func problemOn(t *testing.T, problems []Problem, field Field) Problem {
	t.Helper()
	for _, p := range problems {
		if p.Field == field {
			if p.Message == "" {
				t.Fatalf("problem on %q carries no copy", field)
			}
			return p
		}
	}
	t.Fatalf("no problem on %q: %+v", field, problems)
	return Problem{}
}

func TestCheckRequiresAnEditingRepository(t *testing.T) {
	d := Draft{Name: "Research", Prefix: "feat",
		Repos: []session.RepoSelection{{Repo: repoNamed("api"), Role: session.RepoRoleReference}}}
	problemOn(t, Check(d), FieldRepos)

	d.Repos[0].Role = session.RepoRoleEditing
	if problems := Check(d); len(problems) != 0 {
		t.Fatalf("valid draft rejected: %+v", problems)
	}
}

// A refresh can drop a repository between picking it and advancing. Resolving
// the selection against the live list is what leaves the draft short of one.
func TestCheckRejectsADraftWhoseOnlyEditingRepoHasGone(t *testing.T) {
	d := Draft{Name: "Cleanup", Prefix: "feat"}
	problemOn(t, Check(d), FieldRepos)
}

// The rows a picker opens holding are the session's, not the visit's: an
// escalation over a session already being worked in adds reference repositories
// alone, or confirms adding nothing at all.
func TestAdditionsLeanOnTheEditingRepositoryTheSessionAlreadyHolds(t *testing.T) {
	working := session.Manifest{Repos: []session.ManifestRepo{
		{Org: "acme", Name: "api", Role: session.RepoRoleEditing},
	}}
	reference := Draft{Name: "Research", Prefix: "feat",
		Repos: []session.RepoSelection{{Repo: repoNamed("docs"), Role: session.RepoRoleReference}}}

	if problems := CheckAdditions(working, reference); len(problems) != 0 {
		t.Fatalf("addition to a session holding an editing repo refused: %+v", problems)
	}
	if problems := CheckAdditions(working, Draft{Name: "Research", Prefix: "feat"}); len(problems) != 0 {
		t.Fatalf("confirming no additions refused: %+v", problems)
	}
	problemOn(t, CheckAdditions(session.Manifest{}, reference), FieldRepos)
}

func TestCheckRejectsAnUnparseableTicketURLAndAnEmptyName(t *testing.T) {
	editingAPI := []session.RepoSelection{{Repo: repoNamed("api"), Role: session.RepoRoleEditing}}
	d := Draft{Name: "Cleanup", Prefix: "feat", Ticket: "not a ticket", Repos: editingAPI}
	problemOn(t, Check(d), FieldTicket)

	d = Draft{Name: "!!!", Prefix: "feat", Repos: editingAPI}
	problemOn(t, Check(d), FieldName)

	d.Ticket = "  "
	d.Name = "Cleanup"
	if problems := Check(d); len(problems) != 0 {
		t.Fatalf("blank ticket treated as a URL: %+v", problems)
	}
}

func TestCheckSlugRefusesAnOccupiedDirectoryAndReclaimsAnAbandonedOne(t *testing.T) {
	root := t.TempDir()
	a := Assembler{Cfg: &config.Config{Root: root}}
	d := Draft{Name: "Cleanup", Prefix: "feat"}
	if problems := a.CheckSlug(d); len(problems) != 0 {
		t.Fatalf("free slug rejected: %+v", problems)
	}

	dir := filepath.Join(root, "cleanup")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(dir, session.Manifest{Slug: "cleanup"}); err != nil {
		t.Fatal(err)
	}
	problemOn(t, a.CheckSlug(d), FieldName)

	// A half-assembly left behind by an interrupted run is reclaimed, not a clash.
	if err := os.Remove(filepath.Join(dir, "qrouton.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".qrouton-assembling"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if problems := a.CheckSlug(d); len(problems) != 0 {
		t.Fatalf("abandoned half-assembly blocked the name: %+v", problems)
	}
}
