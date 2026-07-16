package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func testApp() appModel {
	return appModel{
		cfg: &Config{Orgs: []string{"acme", "other"}, Root: "/tmp/sessions"},
		repos: []Repo{
			{Org: "acme", Name: "api", DefaultBranch: "main", PushedAt: time.Now()},
			{Org: "other", Name: "web", DefaultBranch: "trunk", PushedAt: time.Now().Add(-time.Hour)},
		},
		ownerStatus: make(map[string]string), ownerErrors: make(map[string]error),
		form: formState{roles: make(map[string]repoRole)},
	}
}

func TestRepositoryInclusionAndRoles(t *testing.T) {
	m := testApp()
	m.toggleIncluded()
	if got := m.form.roles["acme/api"]; got != active {
		t.Fatalf("first included repository role = %v, want active", got)
	}
	m.form.cursor = 1
	m.toggleIncluded()
	if got := m.form.roles["other/web"]; got != reference {
		t.Fatalf("additional repository role = %v, want reference", got)
	}
	m.toggleRole()
	if got := m.form.roles["other/web"]; got != active {
		t.Fatalf("toggled repository role = %v, want active", got)
	}
	m.toggleIncluded()
	if _, ok := m.form.roles["other/web"]; ok {
		t.Fatal("excluded repository retained a role")
	}
}

func TestRepositoryFiltersPreserveActivityOrder(t *testing.T) {
	m := testApp()
	m.form.owner = 1
	m.form.search = "AP"
	got := m.filteredRepos()
	if len(got) != 1 || repoID(got[0]) != "acme/api" {
		t.Fatalf("filtered repositories = %#v", got)
	}
	if len(m.repos) != 2 || repoID(m.repos[0]) != "acme/api" {
		t.Fatal("filter mutated the source activity ordering")
	}
}

func TestFormRendersLiveBranchAndReferencePreview(t *testing.T) {
	m := testApp()
	m.screen = newScreen
	m.form.name = "API Cleanup!"
	m.form.prefix = 1
	m.form.roles["acme/api"] = active
	m.form.roles["other/web"] = reference
	view := m.viewForm()
	for _, want := range []string{"api-cleanup", "fix/api-cleanup", "trunk · reference", "2 included · 1 active"} {
		if !strings.Contains(view, want) {
			t.Errorf("form preview missing %q\n%s", want, view)
		}
	}
	m.form.description = "Coordinate the cleanup"
	view = m.viewForm()
	if description, repositories := strings.Index(view, "Coordinate the cleanup"), strings.Index(view, "Repositories"); description < 0 || description > repositories {
		t.Fatalf("description should render directly under the name and before repositories:\n%s", view)
	}
}

func TestValidateFormRequiresActiveRepository(t *testing.T) {
	m := testApp()
	m.cfg.Root = t.TempDir()
	m.form.name = "Research"
	m.form.roles["acme/api"] = reference
	if err := m.validateForm(); err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("validateForm error = %v, want active repository error", err)
	}
	m.form.roles["acme/api"] = active
	if err := m.validateForm(); err != nil {
		t.Fatalf("valid form rejected: %v", err)
	}
}

func TestValidateFormRejectsActiveRepoRemovedByRefresh(t *testing.T) {
	m := testApp()
	m.cfg.Root = t.TempDir()
	m.form.name = "Cleanup"
	m.form.roles["acme/api"] = active
	m.repos = m.repos[1:]
	if err := m.validateForm(); err == nil || !strings.Contains(err.Error(), "active") {
		t.Fatalf("validation after refresh = %v", err)
	}
}

func TestSpaceIsEnteredInTextFields(t *testing.T) {
	m := testApp()
	m.screen = newScreen
	m.form.focus = 0
	m.form.name = "API"
	updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(appModel).form.name
	if got != "API " {
		t.Fatalf("name after space = %q", got)
	}
	m.form.focus = 1
	m.form.description = "multi"
	updated, _ = m.updateForm(tea.KeyMsg{Type: tea.KeySpace})
	got = updated.(appModel).form.description
	if got != "multi " {
		t.Fatalf("description after space = %q", got)
	}
}

func TestResumeRequiresRunnerSelection(t *testing.T) {
	m := testApp()
	m.sessions = []Manifest{{Slug: "existing"}}
	m.landingCursor = 1
	m.runners = []Runner{{ID: "codex", Label: "Codex"}}
	updated, cmd := m.updateLanding(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(appModel)
	if got.screen != runnerScreen || got.resume == nil {
		t.Fatalf("resume state = screen %v manifest %v", got.screen, got.resume)
	}
	if cmd != nil {
		t.Fatal("resume began assembly before runner selection")
	}
}

func TestStaleRefreshEventIsIgnored(t *testing.T) {
	m := testApp()
	m.refreshGen = 2
	updated, _ := m.Update(refreshEventMsg{gen: 1, event: repoRefreshMsg{State: repoRefreshSucceeded, Repos: []Repo{{Org: "stale", Name: "repo"}}}})
	got := updated.(appModel)
	if repoID(got.repos[0]) != "acme/api" {
		t.Fatalf("stale refresh replaced repositories: %#v", got.repos)
	}
}

func TestAssemblyProgressUpdatesVisibleSteps(t *testing.T) {
	m := testApp()
	m.screen = assemblyScreen
	m.form.name = "Cleanup"
	ch := make(chan assemblyEvent)
	m.assembly = ch
	repo := m.repos[0]
	p := SessionProgress{Step: SessionProgressMirror, Status: SessionProgressCompleted, Repo: &repo}
	updated, _ := m.Update(assemblyEventMsg{event: assemblyEvent{progress: &p}})
	view := updated.(appModel).viewAssembly()
	if !strings.Contains(view, "✓ acme/api mirror") {
		t.Fatalf("progress missing from view:\n%s", view)
	}
}

func TestLandingCardIncludesDescriptionAndRepositories(t *testing.T) {
	m := testApp()
	m.sessions = []Manifest{{Slug: "fix-login", Description: "Refresh expired sessions", CreatedAt: time.Now(), Repos: []ManifestRepo{{Org: "acme", Name: "api"}, {Org: "other", Name: "web"}}}}
	view := m.viewLanding()
	for _, want := range []string{"fix-login", "Refresh expired sessions", "acme/api · other/web"} {
		if !strings.Contains(view, want) {
			t.Errorf("landing card missing %q", want)
		}
	}
}

func TestLandingUsesResponsiveCroutonLogo(t *testing.T) {
	m := testApp()
	m.screen = landingScreen
	m.width, m.height = 80, 40
	if view := m.View(); !strings.Contains(view, "__________") || !strings.Contains(view, "|  ·   *  |") {
		t.Fatalf("tall landing view missing full crouton cube:\n%s", view)
	}
	m.height = 24
	if view := m.View(); !strings.Contains(view, "/· *_/|") || strings.Contains(view, "__________") {
		t.Fatalf("short landing view should use compact cube:\n%s", view)
	}
}

func TestAllOwnerFailureLeavesLoadingForActionableError(t *testing.T) {
	m := testApp()
	m.repos = nil
	m.screen = loadingScreen
	m.ownerErrors["acme"] = fmt.Errorf("unavailable")
	updated, _ := m.Update(refreshEventMsg{gen: m.refreshGen, event: repoRefreshMsg{State: repoRefreshComplete}})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != landingScreen {
		t.Fatalf("all-owner failure screen=%v back=%v", got.screen, got.back)
	}
}

func TestAssemblyFailureBackTargetIsUsable(t *testing.T) {
	m := testApp()
	m.screen = assemblyScreen
	updated, _ := m.Update(assembledMsg{err: fmt.Errorf("git failed")})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != runnerScreen || !got.assemblyFailed {
		t.Fatalf("assembly failure screen=%v back=%v failed=%v", got.screen, got.back, got.assemblyFailed)
	}
}
