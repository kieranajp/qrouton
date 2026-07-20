package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

func testApp() appModel {
	return appModel{
		cfg: &config.Config{Orgs: []string{"acme", "other"}, Root: "/tmp/sessions"},
		repos: []github.Repo{
			{Org: "acme", Name: "api", DefaultBranch: "main", PushedAt: time.Now()},
			{Org: "other", Name: "web", DefaultBranch: "trunk", PushedAt: time.Now().Add(-time.Hour)},
		},
		ownerStatus: make(map[string]string), ownerErrors: make(map[string]error),
		form: formState{roles: make(map[string]repoRole), owners: selectedOwners([]string{"acme", "other"})},
	}
}

func TestRepositoryInclusionAndRoles(t *testing.T) {
	m := testApp()
	m.cycleRepoRole()
	if got := m.form.roles["acme/api"]; got != active {
		t.Fatalf("first cycle role = %v, want active", got)
	}
	m.cycleRepoRole()
	if got := m.form.roles["acme/api"]; got != reference {
		t.Fatalf("second cycle role = %v, want reference", got)
	}
	m.cycleRepoRole()
	if _, ok := m.form.roles["acme/api"]; ok {
		t.Fatal("third cycle did not exclude repository")
	}
	m.form.cursor = 1
	m.cycleRepoRole()
	if got := m.form.roles["other/web"]; got != active {
		t.Fatalf("each repository should independently cycle to active, got %v", got)
	}
}

func TestModeFieldTogglesAndRenders(t *testing.T) {
	m := testApp()
	m.screen = newScreen
	m.form.mode = session.ModeRPI
	m.form.focus = 6

	view := m.viewForm()
	if !strings.Contains(view, "RPI (default)") || !strings.Contains(view, "Research → Plan → Implement") {
		t.Fatalf("form should render the RPI mode field:\n%s", view)
	}

	// space, left, and right all cycle the two-option mode field.
	for _, key := range []tea.KeyMsg{{Type: tea.KeySpace}, {Type: tea.KeyLeft}, {Type: tea.KeyRight}} {
		before := m.form.mode
		updated, _ := m.updateForm(key)
		m = updated.(appModel)
		if m.form.mode == before {
			t.Fatalf("%v did not toggle mode from %q", key, before)
		}
	}
	// Odd number of toggles from RPI lands on Assistant.
	if m.form.mode != session.ModeAssistant {
		t.Fatalf("mode after three toggles = %q, want assistant", m.form.mode)
	}
	if v := m.viewForm(); !strings.Contains(v, "Assistant") || !strings.Contains(v, "escalate to RPI") {
		t.Fatalf("form should render the assistant mode field:\n%s", v)
	}
}

func TestRepositoryFiltersPreserveActivityOrder(t *testing.T) {
	m := testApp()
	m.form.owner = 1
	m.form.search = "AP"
	got := m.filteredRepos()
	if len(got) != 1 || got[0].ID() != "acme/api" {
		t.Fatalf("filtered repositories = %#v", got)
	}
	if len(m.repos) != 2 || m.repos[0].ID() != "acme/api" {
		t.Fatal("filter mutated the source activity ordering")
	}
}

func TestOwnerSwitchClampsRepoCursorWithoutPanic(t *testing.T) {
	m := testApp()
	m.repos = append(m.repos,
		github.Repo{Org: "acme", Name: "api2", DefaultBranch: "main", PushedAt: time.Now()},
		github.Repo{Org: "acme", Name: "api3", DefaultBranch: "main", PushedAt: time.Now()},
	)
	m.screen = newScreen
	m.form.focus = 4
	m.form.cursor = 3 // valid while all four repositories are listed

	// Unselect acme, leaving only the other owner and its single repository.
	m.form.focus = 3
	m.form.owner = 0
	updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(appModel)

	if got := len(m.filteredRepos()); got != 1 {
		t.Fatalf("precondition: narrowed owner should list 1 repo, got %d", got)
	}
	if m.form.cursor >= len(m.filteredRepos()) {
		t.Fatalf("cursor %d not clamped into %d filtered repos", m.form.cursor, len(m.filteredRepos()))
	}

	// Cycling a role must act on the visible repo, not panic on a stale index.
	m.form.focus = 4
	updated, _ = m.updateForm(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(appModel)
	if m.form.roles["other/web"] != active {
		t.Fatalf("space did not cycle the narrowed repo to active: %v", m.form.roles)
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
	m.form.focus = 1
	m.form.name = "API"
	updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(appModel).form.name
	if got != "API " {
		t.Fatalf("name after space = %q", got)
	}
	m.form.focus = 2
	m.form.description = "multi"
	updated, _ = m.updateForm(tea.KeyMsg{Type: tea.KeySpace})
	got = updated.(appModel).form.description
	if got != "multi " {
		t.Fatalf("description after space = %q", got)
	}
}

func TestNavigationLettersAreEnteredInTextFields(t *testing.T) {
	m := testApp()
	m.screen = newScreen
	m.form.focus = 1
	for _, letter := range []rune("hello") {
		updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{letter}})
		m = updated.(appModel)
	}
	if m.form.name != "hello" {
		t.Fatalf("name = %q, want navigation letters preserved", m.form.name)
	}
}

func TestTypingInRepositoryListFiltersAndBackspaceClears(t *testing.T) {
	m := testApp()
	m.screen = newScreen
	m.form.focus = 4
	for _, letter := range []rune("web") {
		updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{letter}})
		m = updated.(appModel)
	}
	if got := m.filteredRepos(); len(got) != 1 || got[0].ID() != "other/web" {
		t.Fatalf("type-to-filter repositories = %#v", got)
	}
	updated, _ := m.updateForm(tea.KeyMsg{Type: tea.KeyBackspace})
	m = updated.(appModel)
	if m.form.search != "we" {
		t.Fatalf("repository filter after backspace = %q", m.form.search)
	}
}

func TestTicketResultPopulatesNameAndDescription(t *testing.T) {
	m := testApp()
	m.form.ticket = "https://linear.app/acme/issue/API-42/fix-retries"
	updated, _ := m.Update(ticketLoadedMsg{url: m.form.ticket, ticket: ticket.Ticket{Title: "Fix retries", Body: "Retry failed requests"}})
	got := updated.(appModel)
	if got.form.name != "Fix retries" || got.form.description != "Retry failed requests" || got.form.ticketStatus != "ticket loaded" {
		t.Fatalf("ticket did not populate form: %#v", got.form)
	}
}

func TestResumeRequiresRunnerSelection(t *testing.T) {
	m := testApp()
	m.sessions = []session.Manifest{{Slug: "existing"}}
	m.landingCursor = 1
	m.runners = []launch.Runner{{ID: "codex", Label: "Codex"}}
	updated, cmd := m.updateLanding(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(appModel)
	if got.screen != runnerScreen || got.resume == nil {
		t.Fatalf("resume state = screen %v manifest %v", got.screen, got.resume)
	}
	if cmd != nil {
		t.Fatal("resume began assembly before runner selection")
	}
}

func TestResumedAssemblyProducesResumeLaunchRequest(t *testing.T) {
	m := testApp()
	existingSession := session.Manifest{Slug: "existing"}
	m.resume = &existingSession
	m.runners = []launch.Runner{{ID: "codex", Label: "Codex", Path: "/bin/codex", Command: []string{"codex"}}}
	updated, _ := m.Update(assembledMsg{dir: "/tmp/existing"})
	got := updated.(appModel)
	if got.result == nil || !got.result.Resume {
		t.Fatalf("resume assembly launch request = %#v", got.result)
	}
}

func TestStaleRefreshEventIsIgnored(t *testing.T) {
	m := testApp()
	m.refreshGen = 2
	updated, _ := m.Update(refreshEventMsg{gen: 1, event: github.RefreshMsg{State: github.RefreshSucceeded, Repos: []github.Repo{{Org: "stale", Name: "repo"}}}})
	got := updated.(appModel)
	if got.repos[0].ID() != "acme/api" {
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
	p := session.Progress{Step: session.ProgressMirror, Status: session.ProgressCompleted, Repo: &repo}
	updated, _ := m.Update(assemblyEventMsg{event: assemblyEvent{progress: &p}})
	view := updated.(appModel).viewAssembly()
	if !strings.Contains(view, "✓ acme/api mirror") {
		t.Fatalf("progress missing from view:\n%s", view)
	}
}

func TestLandingCardIncludesDescriptionAndRepositories(t *testing.T) {
	m := testApp()
	m.sessions = []session.Manifest{{Slug: "fix-login", Description: "Refresh expired sessions", CreatedAt: time.Now(), Repos: []session.ManifestRepo{{Org: "acme", Name: "api"}, {Org: "other", Name: "web"}}}}
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

func TestStaleFailedRetryIsIgnored(t *testing.T) {
	m := testApp()
	m.refreshGen = 2
	updated, _ := m.Update(failedRetryMsg{gen: 1, repos: []github.Repo{{Org: "stale", Name: "repo"}}})
	got := updated.(appModel)
	if got.repos[0].ID() != "acme/api" {
		t.Fatalf("stale retry replaced repositories: %#v", got.repos)
	}
}

func TestFailedRetryOnLoadingScreenShowsError(t *testing.T) {
	m := testApp()
	m.screen = loadingScreen
	updated, _ := m.Update(failedRetryMsg{gen: m.refreshGen, repos: m.repos,
		results: map[string]error{"acme": fmt.Errorf("still unavailable")}})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != landingScreen {
		t.Fatalf("failed retry left screen=%v back=%v, want error screen", got.screen, got.back)
	}
}

func TestTokenFailureOnLoadingScreenWithCachedReposShowsError(t *testing.T) {
	m := testApp()
	m.screen = loadingScreen
	updated, _ := m.Update(refreshReadyMsg{gen: m.refreshGen, err: fmt.Errorf("no token")})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != landingScreen {
		t.Fatalf("token failure left screen=%v back=%v, want error screen", got.screen, got.back)
	}
}

func TestAllOwnerFailureLeavesLoadingForActionableError(t *testing.T) {
	m := testApp()
	m.repos = nil
	m.screen = loadingScreen
	m.ownerErrors["acme"] = fmt.Errorf("unavailable")
	updated, _ := m.Update(refreshEventMsg{gen: m.refreshGen, event: github.RefreshMsg{State: github.RefreshComplete}})
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
