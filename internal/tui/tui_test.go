package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
		refresh: refreshState{status: make(map[string]string), errs: make(map[string]error)},
		form:    formState{roles: make(map[string]repoRole), owners: selectedOwners([]string{"acme", "other"})},
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
	m.width = 100
	m.screen = newScreen
	m.form.mode = session.ModeRPI
	m.form.focus = 6

	view := m.viewForm()
	if !strings.Contains(view, "RPI") || !strings.Contains(view, "Research → Plan → Implement") {
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
	m.width = 100
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
	m.refresh.gen = 2
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
	m.assembly.ch = ch
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

func TestLandingDescriptionIsSingleLineAndTruncated(t *testing.T) {
	got := landingDescription("A long Linear ticket\n\nwith several paragraphs and extra detail", 30)
	if strings.Contains(got, "\n") {
		t.Fatalf("landing description contains a newline: %q", got)
	}
	if got != "A long Linear ticket with sev…" {
		t.Fatalf("landing description = %q", got)
	}
}

func TestLandingDescriptionIsDimmed(t *testing.T) {
	m := testApp()
	m.sessions = []session.Manifest{{Slug: "fix-login", Description: "Refresh expired sessions"}}

	view := m.viewLanding()
	if !strings.Contains(view, dim.Render("Refresh expired sessions")) {
		t.Fatalf("landing description is not dimmed:\n%s", view)
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

func TestLandingSessionListScrollsWithinTerminalHeight(t *testing.T) {
	m := testApp()
	m.screen = landingScreen
	m.width, m.height = 80, 24
	for i := 1; i <= 6; i++ {
		m.sessions = append(m.sessions, session.Manifest{Slug: fmt.Sprintf("session-%d", i)})
	}

	view := m.View()
	if !strings.Contains(view, "session-1") || strings.Contains(view, "session-6") {
		t.Fatalf("initial landing window does not show the first page:\n%s", view)
	}
	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("landing view is %d lines in a %d-line terminal", got, m.height)
	}

	for range 6 {
		updated, _ := m.updateLanding(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(appModel)
	}
	if m.landingOffset == 0 {
		t.Fatal("landing window did not scroll as the cursor moved")
	}
	view = m.View()
	if !strings.Contains(view, "session-6") || strings.Contains(view, "session-1") {
		t.Fatalf("landing window did not follow the cursor:\n%s", view)
	}
	if !strings.Contains(view, "↑ 5 earlier sessions") {
		t.Fatalf("landing window is missing its scroll indicator:\n%s", view)
	}
}

func TestLandingPageKeysMoveByVisibleWindow(t *testing.T) {
	m := testApp()
	m.screen = landingScreen
	m.width, m.height = 80, 40
	for i := 1; i <= 10; i++ {
		m.sessions = append(m.sessions, session.Manifest{Slug: fmt.Sprintf("session-%d", i)})
	}
	window := m.landingSessionWindow()

	updated, _ := m.updateLanding(tea.KeyMsg{Type: tea.KeyPgDown})
	m = updated.(appModel)
	if m.landingCursor != window {
		t.Fatalf("page down cursor = %d, want %d", m.landingCursor, window)
	}
	updated, _ = m.updateLanding(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(appModel)
	if m.landingCursor != 0 || m.landingOffset != 0 {
		t.Fatalf("page up did not return to new session: cursor=%d offset=%d", m.landingCursor, m.landingOffset)
	}
}

func TestStaleFailedRetryIsIgnored(t *testing.T) {
	m := testApp()
	m.refresh.gen = 2
	updated, _ := m.Update(failedRetryMsg{gen: 1, repos: []github.Repo{{Org: "stale", Name: "repo"}}})
	got := updated.(appModel)
	if got.repos[0].ID() != "acme/api" {
		t.Fatalf("stale retry replaced repositories: %#v", got.repos)
	}
}

func TestFailedRetryOnLoadingScreenShowsError(t *testing.T) {
	m := testApp()
	m.screen = loadingScreen
	updated, _ := m.Update(failedRetryMsg{gen: m.refresh.gen, repos: m.repos,
		results: map[string]error{"acme": fmt.Errorf("still unavailable")}})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != landingScreen {
		t.Fatalf("failed retry left screen=%v back=%v, want error screen", got.screen, got.back)
	}
}

func TestTokenFailureOnLoadingScreenWithCachedReposShowsError(t *testing.T) {
	m := testApp()
	m.screen = loadingScreen
	updated, _ := m.Update(refreshReadyMsg{gen: m.refresh.gen, err: fmt.Errorf("no token")})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != landingScreen {
		t.Fatalf("token failure left screen=%v back=%v, want error screen", got.screen, got.back)
	}
}

func TestAllOwnerFailureLeavesLoadingForActionableError(t *testing.T) {
	m := testApp()
	m.repos = nil
	m.screen = loadingScreen
	m.refresh.errs["acme"] = fmt.Errorf("unavailable")
	updated, _ := m.Update(refreshEventMsg{gen: m.refresh.gen, event: github.RefreshMsg{State: github.RefreshComplete}})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != landingScreen {
		t.Fatalf("all-owner failure screen=%v back=%v", got.screen, got.back)
	}
}

func TestPickerPrefillsNameAndPrefix(t *testing.T) {
	cfg := &config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}
	m := newPickerModel(cfg, "/tmp/session", session.Manifest{Slug: "session"}, "Webhook retry backoff", "fix")
	if m.screen != newScreen {
		t.Fatalf("picker starts on screen %v, want the form", m.screen)
	}
	if m.form.name != "Webhook retry backoff" {
		t.Fatalf("picker name = %q", m.form.name)
	}
	if branchPrefixes[m.form.prefix] != "fix" {
		t.Fatalf("picker prefix = %q, want fix", branchPrefixes[m.form.prefix])
	}
	if len(m.form.roles) != 0 {
		t.Fatalf("picker pre-selected repositories: %v", m.form.roles)
	}
}

func TestPickerCancelWritesCancelledStanzaOnly(t *testing.T) {
	dir := t.TempDir()
	manifest := session.Manifest{Slug: "scratch", Mode: session.ModeAssistant}
	if err := session.WriteManifest(dir, manifest); err != nil {
		t.Fatal(err)
	}
	m := newPickerModel(&config.Config{Orgs: []string{"acme"}, Root: t.TempDir()}, dir, manifest, "", "")
	_, cmd := m.updateForm(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("cancel did not quit the picker")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("cancel command produced %T, want quit", cmd())
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation == nil || got.Escalation.Status != session.EscalationCancelled || got.Escalation.At.IsZero() {
		t.Fatalf("cancelled stanza = %+v", got.Escalation)
	}
	if got.EffectiveMode() != session.ModeAssistant || len(got.Repos) != 0 {
		t.Fatalf("cancel touched the session beyond the stanza: %+v", got)
	}
}

func TestPickerConfirmWritesReposModeAndStanzaTogether(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Orgs: []string{"org"}, Root: root}
	dir, err := session.Create(cfg, "scratch", "", "", "", session.ModeAssistant, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sels := []session.RepoSelection{{
		Repo: github.Repo{Name: "svc", Org: "org", SSHURL: makeTestOrigin(t), DefaultBranch: "main"},
		Role: session.RepoRoleActive,
	}}
	if err := confirmEscalation(cfg, dir, manifest, sels, escalationDetails{name: "Webhook retry backoff", prefix: "fix"}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != session.ModeRPI || got.Name != "Webhook retry backoff" {
		t.Fatalf("escalated manifest = mode %q name %q", got.Mode, got.Name)
	}
	if len(got.Repos) != 1 || got.Repos[0].Branch != "fix/webhook-retry-backoff" {
		t.Fatalf("escalated repos = %+v", got.Repos)
	}
	if got.Escalation == nil || got.Escalation.Status != session.EscalationConfirmed || got.Escalation.At.IsZero() {
		t.Fatalf("confirmed stanza = %+v", got.Escalation)
	}
}

func makeTestOrigin(t *testing.T) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "svc")
	for _, args := range [][]string{
		{"init", "-b", "main", origin},
		{"-C", origin, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "initial"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return origin
}

func TestAssemblyFailureBackTargetIsUsable(t *testing.T) {
	m := testApp()
	m.screen = assemblyScreen
	updated, _ := m.Update(assembledMsg{err: fmt.Errorf("git failed")})
	got := updated.(appModel)
	if got.screen != errorScreen || got.back != runnerScreen || !got.assembly.failed {
		t.Fatalf("assembly failure screen=%v back=%v failed=%v", got.screen, got.back, got.assembly.failed)
	}
}

// Clone and fetch report repeatedly for one step, and the view draws only the
// latest of each; without collapsing them the slice the render walks every
// frame would grow for the whole clone.
func TestRecordStepCollapsesRepeatedProgressPerRepo(t *testing.T) {
	svc := github.Repo{Name: "svc", Org: "org"}
	web := github.Repo{Name: "web", Org: "org"}
	var steps []session.Progress

	steps = recordStep(steps, session.Progress{Step: session.ProgressMirror, Status: session.ProgressStarted, Repo: &svc})
	for percent := 10; percent <= 90; percent += 10 {
		steps = recordStep(steps, session.Progress{Step: session.ProgressMirror, Status: session.ProgressAdvanced,
			Repo: &svc, Phase: "Receiving objects", Percent: percent})
	}
	if len(steps) != 2 {
		t.Fatalf("nine updates for one repo produced %d steps, want 2", len(steps))
	}
	if last := steps[1]; last.Percent != 90 {
		t.Fatalf("collapsed step kept %d%%, want the newest (90%%)", last.Percent)
	}

	// A different repository is a different row, not an overwrite.
	steps = recordStep(steps, session.Progress{Step: session.ProgressMirror, Status: session.ProgressAdvanced,
		Repo: &web, Phase: "Receiving objects", Percent: 5})
	if len(steps) != 3 {
		t.Fatalf("a second repository was collapsed into the first: %d steps", len(steps))
	}
	// Outcomes are never collapsed away — they are what the view colours.
	steps = recordStep(steps, session.Progress{Step: session.ProgressMirror, Status: session.ProgressCompleted, Repo: &web})
	if len(steps) != 4 {
		t.Fatalf("completion overwrote a progress row: %d steps", len(steps))
	}
}

func TestProgressBarClampsAndFills(t *testing.T) {
	for _, tc := range []struct{ percent, wantFull int }{
		{-5, 0}, {0, 0}, {50, progressBarWidth / 2}, {100, progressBarWidth}, {150, progressBarWidth},
	} {
		got := strings.Count(progressBar(tc.percent), progressBarFull)
		if got != tc.wantFull {
			t.Fatalf("progressBar(%d) filled %d cells, want %d", tc.percent, got, tc.wantFull)
		}
	}
}

// Escalating a session that has already been worked in must not disturb the
// checkout the work is in. Selecting the repo you are working on is the obvious
// move in the picker, and it used to produce a second clone of it on a second
// branch, leaving the original's commits behind on the old one.
func TestEscalationLeavesAnAlreadyPresentRepoAlone(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Root: root}
	repo := github.Repo{Name: "repo123", Org: "kieranajp", SSHURL: makeTestOrigin(t), DefaultBranch: "main"}
	dir, err := session.Create(cfg, "repo123", "", "", "feat", session.ModeAssistant,
		[]session.RepoSelection{{Repo: repo, Role: session.RepoRoleActive}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Uncommitted work in the original checkout.
	stub := filepath.Join(dir, "src", "repo123", "stub.go")
	if err := os.WriteFile(stub, []byte("package stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sels := []session.RepoSelection{{Repo: repo, Role: session.RepoRoleActive}}
	if err := confirmEscalation(cfg, dir, manifest, sels,
		escalationDetails{name: "Webhook retry backoff", prefix: "fix"}, nil); err != nil {
		t.Fatal(err)
	}

	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repository duplicated on escalation: %+v", got.Repos)
	}
	if r := got.Repos[0]; r.Branch != "feat/repo123" || r.WorktreePath != "src/repo123" {
		t.Fatalf("existing checkout was moved: branch=%q worktree=%q", r.Branch, r.WorktreePath)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "src")); err != nil || len(entries) != 1 {
		t.Fatalf("src/ holds %d checkouts, want 1 (err %v)", len(entries), err)
	}
	if _, err := os.Stat(stub); err != nil {
		t.Fatal("uncommitted work lost from the original checkout:", err)
	}
	if got.EffectiveMode() != session.ModeRPI {
		t.Fatalf("mode = %q, want rpi", got.Mode)
	}
}

// The picker used to render every repository as excluded, including ones the
// session already held — which is what made selecting the worked-in repo look
// necessary. It also asked again for a name and description it already had.
func TestPickerSeedsFormFromTheSessionItIsEscalating(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Orgs: []string{"kieranajp"}}
	manifest := session.Manifest{Name: "Existing work", Description: "already known",
		TicketURL: "https://linear.app/x/issue/ABC-1", Repos: []session.ManifestRepo{
			{Name: "repo123", Org: "kieranajp", Role: session.RepoRoleActive},
			{Name: "shared", Org: "kieranajp", Role: session.RepoRoleReference},
		}}

	m := newPickerModel(cfg, "/tmp/session", manifest, "", "")

	if m.form.name != "Existing work" || m.form.description != "already known" {
		t.Fatalf("form not seeded: name=%q description=%q", m.form.name, m.form.description)
	}
	if m.form.ticket != manifest.TicketURL {
		t.Fatalf("ticket not seeded: %q", m.form.ticket)
	}
	if got := m.form.roles["kieranajp/repo123"]; got != active {
		t.Fatalf("present active repo seeded as %v, want active", got)
	}
	if got := m.form.roles["kieranajp/shared"]; got != reference {
		t.Fatalf("present reference repo seeded as %v, want reference", got)
	}
	// Escalation is the move to RPI, so the mode selector is not offered.
	if m.lastFormField() != focusPrefix {
		t.Fatalf("mode field reachable in picker mode (last = %d)", m.lastFormField())
	}
}

// The role glyph alone could not distinguish "you selected this" from "the
// session already has this", and the second cannot be changed: removing a repo
// from a live session, or re-roling one, is not implemented.
func TestPickerMarksAndLocksRepositoriesAlreadyInTheSession(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Orgs: []string{"kieranajp"}}
	manifest := session.Manifest{Name: "Work", Repos: []session.ManifestRepo{
		{Name: "repo123", Org: "kieranajp", Role: session.RepoRoleActive},
	}}
	m := newPickerModel(cfg, "/tmp/session", manifest, "", "")
	m.width = 200
	m.repos = []github.Repo{
		{Name: "repo123", Org: "kieranajp", DefaultBranch: "main"},
		{Name: "other", Org: "kieranajp", DefaultBranch: "main"},
	}
	m.form.focus = focusRepos

	rendered := m.viewForm()
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "kieranajp/other") && strings.Contains(line, "in session") {
			t.Fatalf("a repo not in the session was marked as in it: %q", line)
		}
	}
	if !strings.Contains(rendered, "in session") {
		t.Fatalf("no in-session marker rendered:\n%s", rendered)
	}

	// Space on the in-session row does nothing; on a free row it still cycles.
	m.form.cursor = 0
	m.cycleRepoRole()
	if got := m.form.roles["kieranajp/repo123"]; got != active {
		t.Fatalf("in-session row cycled to %v; it cannot be changed", got)
	}
	m.form.cursor = 1
	m.cycleRepoRole()
	if got := m.form.roles["kieranajp/other"]; got != active {
		t.Fatalf("free row did not cycle: %v", got)
	}

	// A plain new-session form locks nothing.
	if newAppModel(cfg, nil, "", nil).inSession("kieranajp/repo123") {
		t.Fatal("non-picker form reports a repo as in-session")
	}
}
