// Package tui is the fullscreen Bubble Tea onboarding flow: pick or create a
// session, choose a runner, watch assembly, then hand a LaunchRequest back to
// main. The model and screen dispatch live here; the form, background refresh,
// assembly, and rendering live in their own files.
package tui

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
)

type screen uint8

const (
	landingScreen screen = iota
	loadingScreen
	newScreen
	runnerScreen
	assemblyScreen
	deleteScreen
	errorScreen
)

type repoRole uint8

const (
	excluded repoRole = iota
	active
	reference
)

type LaunchRequest struct {
	Dir    string
	Runner launch.Runner
	Resume bool
}

// appModel is the whole onboarding flow's state. Each screen's own working set
// is grouped into a named sub-struct, so what a screen may touch is visible
// from the field it reaches through rather than from a flat list.
type appModel struct {
	cfg             *config.Config
	sessions        []session.Manifest
	repos           []github.Repo
	runners         []launch.Runner
	requestedRunner string
	screen, back    screen
	landingCursor   int
	runnerCursor    int
	width, height   int
	err             error
	result          *LaunchRequest
	resume          *session.Manifest

	form          formState
	refresh       refreshState
	assembly      assemblyState
	pendingDelete deleteState
	picker        pickerState
}

// deleteState is the delete confirmation's target and the uncommitted work that
// would go with it, held while the confirmation screen is up.
type deleteState struct {
	target *session.Manifest
	dirty  []string
}

// pickerState is picker mode (qrouton pick): a non-nil manifest scopes the
// model to the form alone — confirming assembles into the live session at dir
// instead of producing a LaunchRequest.
type pickerState struct {
	dir      string
	manifest *session.Manifest
}

func Run(cfg *config.Config, sessions []session.Manifest, requestedRunner string, forceRefresh bool) (*LaunchRequest, error) {
	// The first repository search is where owners stop being optional, so this
	// is where they are asked for rather than at config load.
	if err := config.EnsureOrgs(cfg); err != nil {
		return nil, err
	}
	runners, err := launch.Runners(cfg)
	if err != nil {
		return nil, err
	}
	m := newAppModel(cfg, sessions, requestedRunner, installed(runners))
	if requestedRunner != "" {
		if _, err := m.selectedRunner(); err != nil {
			return nil, err
		}
	}
	if forceRefresh {
		m.repos = nil
		m.refresh.cacheAt = time.Time{}
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	out := final.(appModel)
	return out.result, nil
}

// RunPicker runs just the new-session form against a live session: name and
// prefix arrive pre-filled and editable, repositories are chosen fresh, and
// confirming assembles into the session — one atomic manifest write carrying
// repos, mode, name, and the escalation stanza — rather than returning a
// LaunchRequest. Esc records a cancelled outcome and leaves the session as it
// was.
func RunPicker(cfg *config.Config, dir, name, prefix string) error {
	if err := config.EnsureOrgs(cfg); err != nil {
		return err
	}
	manifest, err := session.Load(dir)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(newPickerModel(cfg, dir, manifest, name, prefix), tea.WithAltScreen()).Run()
	return err
}

// newPickerModel seeds the shared appModel at the form screen, in picker mode:
// the landing, runner, and delete screens are unreachable from it. Everything
// the session already knows is filled in — a session that has been worked in
// should not be asked what it is called or which repositories it holds.
func newPickerModel(cfg *config.Config, dir string, manifest session.Manifest, name, prefix string) appModel {
	m := newAppModel(cfg, nil, "", nil)
	m.screen = newScreen
	m.picker.dir = dir
	m.picker.manifest = &manifest
	// The keybinding route passes no name; fall back to the session's own so a
	// bare Alt-e adds repositories without renaming the work.
	m.form.name = name
	if strings.TrimSpace(name) == "" {
		m.form.name = manifest.Name
	}
	m.form.description, m.form.ticket = manifest.Description, manifest.TicketURL
	// Repositories already in the session show with the role they actually
	// have. Without this the list rendered every one of them as excluded,
	// including the repo being worked in, so selecting it again looked like the
	// only sensible move — which is what produced a second checkout of it.
	for _, r := range manifest.Repos {
		role := active
		if r.Role == session.RepoRoleReference {
			role = reference
		}
		// github.Repo.ID owns the key format the roles map is keyed by.
		m.form.roles[(github.Repo{Org: r.Org, Name: r.Name}).ID()] = role
	}
	m.form.focus = focusName
	for i, p := range branchPrefixes {
		if p == prefix {
			m.form.prefix = i
		}
	}
	return m
}

func newAppModel(cfg *config.Config, sessions []session.Manifest, requested string, runners []launch.Runner) appModel {
	repos, fetched, _ := github.CachedRepos(cfg.Orgs)
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].CreatedAt.After(sessions[j].CreatedAt) })
	return appModel{cfg: cfg, sessions: sessions, repos: repos, requestedRunner: requested,
		runners: runners, screen: landingScreen,
		refresh: refreshState{active: true, gen: 1, cacheAt: fetched,
			status: make(map[string]string), errs: make(map[string]error)},
		form: formState{prefix: 0, mode: session.ModeRPI, roles: make(map[string]repoRole), owners: selectedOwners(cfg.Orgs)}}
}

func (m appModel) Init() tea.Cmd { return refreshTokenCmd(m.refresh.gen) }

// installed keeps the runners the picker can actually offer.
func installed(runners []launch.Runner) []launch.Runner {
	var out []launch.Runner
	for _, runner := range runners {
		if runner.Path != "" {
			out = append(out, runner)
		}
	}
	return out
}

// Update routes async messages to the handler that owns the machinery behind
// them, then keys to the focused screen.
func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		return m, nil
	case refreshReadyMsg:
		return m.onRefreshReady(v)
	case refreshEventMsg:
		return m.onRefreshEvent(v)
	case assembledMsg:
		return m.onAssembled(v)
	case assemblyEventMsg:
		return m.onAssemblyEvent(v)
	case failedRetryMsg:
		return m.onFailedRetry(v)
	case ticketLoadedMsg:
		return m.onTicketLoaded(v)
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if k.String() == "ctrl+c" {
		if m.refresh.cancel != nil {
			m.refresh.cancel()
		}
		return m, tea.Quit
	}
	return m.updateScreen(k)
}

// updateScreen hands a key to the focused screen. assemblyScreen is absent
// deliberately: assembly is not interruptible, so it ignores input.
func (m appModel) updateScreen(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case landingScreen:
		return m.updateLanding(k)
	case loadingScreen:
		if k.String() == "esc" {
			m.screen = landingScreen
		}
		return m, nil
	case newScreen:
		return m.updateForm(k)
	case runnerScreen:
		return m.updateRunner(k)
	case deleteScreen:
		return m.updateDelete(k)
	case errorScreen:
		return m.updateError(k)
	}
	return m, nil
}

func (m appModel) updateLanding(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "q", "esc":
		if m.refresh.cancel != nil {
			m.refresh.cancel()
		}
		return m, tea.Quit
	case "up", "k":
		if m.landingCursor > 0 {
			m.landingCursor--
		}
	case "down", "j":
		if m.landingCursor < len(m.sessions) {
			m.landingCursor++
		}
	case "r":
		return m, m.beginRefresh()
	case "d":
		if m.landingCursor == 0 {
			return m, nil
		}
		s := m.sessions[m.landingCursor-1]
		dirty, err := session.DirtyWorktrees(m.cfg.Root, s)
		if err != nil {
			m.err, m.back, m.screen = err, landingScreen, errorScreen
			return m, nil
		}
		m.pendingDelete.target, m.pendingDelete.dirty, m.screen = &s, dirty, deleteScreen
		return m, nil
	case "enter":
		if m.landingCursor == 0 {
			if len(m.repos) == 0 {
				m.screen = loadingScreen
			} else {
				m.screen = newScreen
			}
			return m, nil
		}
		s := m.sessions[m.landingCursor-1]
		m.resume = &s
		if m.requestedRunner == "" {
			if len(m.runners) == 0 {
				m.err, m.back, m.screen = launch.ErrNoRunnerInstalled, landingScreen, errorScreen
				return m, nil
			}
			m.screen = runnerScreen
			return m, nil
		}
		m.screen = assemblyScreen
		return m, m.startAssembly()
	}
	return m, nil
}

func (m appModel) updateDelete(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "n":
		m.pendingDelete.target, m.pendingDelete.dirty, m.screen = nil, nil, landingScreen
	case "enter", "y":
		if m.pendingDelete.target == nil {
			m.screen = landingScreen
			return m, nil
		}
		if err := session.Delete(m.cfg.Root, *m.pendingDelete.target); err != nil {
			m.err, m.back, m.screen = err, deleteScreen, errorScreen
			return m, nil
		}
		deleted := m.pendingDelete.target.Slug
		kept := m.sessions[:0]
		for _, s := range m.sessions {
			if s.Slug != deleted {
				kept = append(kept, s)
			}
		}
		m.sessions = kept
		if m.landingCursor > len(m.sessions) {
			m.landingCursor = len(m.sessions)
		}
		m.pendingDelete.target, m.pendingDelete.dirty, m.screen = nil, nil, landingScreen
	}
	return m, nil
}

func (m appModel) updateRunner(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		if m.resume != nil {
			m.resume = nil
			m.screen = landingScreen
		} else {
			m.screen = newScreen
		}
	case "up", "k":
		if m.runnerCursor > 0 {
			m.runnerCursor--
		}
	case "down", "j":
		if m.runnerCursor+1 < len(m.runners) {
			m.runnerCursor++
		}
	case "enter":
		m.screen = assemblyScreen
		return m, m.startAssembly()
	}
	return m, nil
}

func (m appModel) updateError(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc", "b":
		m.screen = m.back
	case "r":
		if m.assembly.failed {
			m.screen = assemblyScreen
			m.assembly.failed = false
			return m, m.startAssembly()
		}
		m.refresh.active = true
		m.screen = loadingScreen
		if len(m.refresh.errs) > 0 {
			return m, m.retryFailed()
		}
		return m, m.beginRefresh()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m appModel) selectedRunner() (launch.Runner, error) {
	if m.requestedRunner != "" {
		return launch.ByID(m.cfg, m.requestedRunner)
	}
	if len(m.runners) == 0 || m.runnerCursor >= len(m.runners) {
		return launch.Runner{}, errNoRunnerSelected
	}
	return m.runners[m.runnerCursor], nil
}
