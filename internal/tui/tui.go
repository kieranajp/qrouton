// Package tui is the fullscreen Bubble Tea onboarding flow: pick or create a
// session, choose a runner, watch assembly, then hand a LaunchRequest back to
// main. The model and screen dispatch live here; the form, background refresh,
// assembly, and rendering live in their own files.
package tui

import (
	"context"
	"fmt"
	"net/http"
	"sort"
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
	refreshing      bool
	refresh         <-chan github.RefreshMsg
	refreshGen      int
	refreshCancel   context.CancelFunc
	ownerStatus     map[string]string
	ownerErrors     map[string]error
	cacheAt         time.Time
	form            formState
	err             error
	result          *LaunchRequest
	resume          *session.Manifest
	assembly        <-chan assemblyEvent
	assemblySteps   []session.Progress
	assemblyFailed  bool
	deleteTarget    *session.Manifest
	deleteDirty     []string
}

func Run(cfg *config.Config, sessions []session.Manifest, requestedRunner string, forceRefresh bool) (*LaunchRequest, error) {
	m := newAppModel(cfg, sessions, requestedRunner)
	if requestedRunner != "" {
		if _, err := m.selectedRunner(); err != nil {
			return nil, err
		}
	}
	if forceRefresh {
		m.repos = nil
		m.cacheAt = time.Time{}
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return nil, err
	}
	out := final.(appModel)
	return out.result, nil
}

func newAppModel(cfg *config.Config, sessions []session.Manifest, requested string) appModel {
	repos, fetched, _ := github.CachedRepos(cfg.Orgs)
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].CreatedAt.After(sessions[j].CreatedAt) })
	return appModel{cfg: cfg, sessions: sessions, repos: repos, requestedRunner: requested,
		runners: availableRunners(cfg), screen: landingScreen, refreshing: true, cacheAt: fetched,
		ownerStatus: make(map[string]string), ownerErrors: make(map[string]error), refreshGen: 1,
		form: formState{prefix: 0, mode: session.ModeRPI, roles: make(map[string]repoRole), owners: selectedOwners(cfg.Orgs)}}
}

func (m appModel) Init() tea.Cmd { return refreshTokenCmd(m.refreshGen) }

func availableRunners(cfg *config.Config) []launch.Runner {
	var out []launch.Runner
	for _, r := range launch.Runners(cfg) {
		if r.Path != "" {
			out = append(out, r)
		}
	}
	return out
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		return m, nil
	case refreshReadyMsg:
		if v.gen != m.refreshGen {
			return m, nil
		}
		if v.err != nil {
			m.refreshing = false
			m.err = v.err
			// The loading screen has no failure rendering of its own; without
			// this it would sit on "Loading repositories…" forever.
			if len(m.repos) == 0 || m.screen == loadingScreen {
				m.back, m.screen = landingScreen, errorScreen
			}
			return m, nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.refreshCancel = cancel
		m.refresh = github.RefreshRepos(ctx, http.DefaultClient, v.token, m.cfg.Orgs, m.repos)
		return m, awaitRefresh(v.gen, m.refresh)
	case refreshEventMsg:
		if v.gen != m.refreshGen {
			return m, nil
		}
		event := v.event
		switch event.State {
		case github.RefreshStarted:
			m.ownerStatus[event.Owner] = "fetching…"
		case github.RefreshSucceeded:
			m.ownerStatus[event.Owner] = "updated"
			delete(m.ownerErrors, event.Owner)
			if event.Repos != nil {
				m.repos = event.Repos
				m.clampRepoCursor()
			}
		case github.RefreshFailed:
			m.ownerStatus[event.Owner] = "failed"
			m.ownerErrors[event.Owner] = event.Err
			m.err = event.Err
		case github.RefreshComplete:
			m.refreshing = false
			if len(m.ownerErrors) == 0 {
				m.err = nil
			}
			if event.Repos != nil {
				m.repos = event.Repos
			}
			m.cacheAt = time.Now()
			if m.screen == loadingScreen && len(m.repos) > 0 {
				m.screen = newScreen
			} else if m.screen == loadingScreen && len(m.ownerErrors) > 0 {
				m.back, m.screen = landingScreen, errorScreen
			}
			return m, nil
		}
		return m, awaitRefresh(v.gen, m.refresh)
	case assembledMsg:
		if v.err != nil {
			back := newScreen
			if m.requestedRunner == "" {
				back = runnerScreen
			} else if m.resume != nil {
				back = landingScreen
			}
			m.err, m.assemblyFailed, m.back, m.screen = v.err, true, back, errorScreen
			return m, nil
		}
		m.assemblyFailed = false
		r, err := m.selectedRunner()
		if err != nil {
			m.err, m.back, m.screen = err, runnerScreen, errorScreen
			return m, nil
		}
		m.result = &LaunchRequest{Dir: v.dir, Runner: r, Resume: m.resume != nil}
		if m.refreshCancel != nil {
			m.refreshCancel()
		}
		return m, tea.Quit
	case assemblyEventMsg:
		if v.event.progress != nil {
			m.assemblySteps = append(m.assemblySteps, *v.event.progress)
			return m, awaitAssembly(m.assembly)
		}
		if v.event.done != nil {
			return m.Update(*v.event.done)
		}
	case failedRetryMsg:
		if v.gen != m.refreshGen {
			return m, nil
		}
		m.refreshing = false
		m.repos = v.repos
		for owner, err := range v.results {
			if err != nil {
				m.ownerErrors[owner] = err
				m.ownerStatus[owner] = "failed"
				m.err = err
			} else {
				delete(m.ownerErrors, owner)
				m.ownerStatus[owner] = "updated"
			}
		}
		if len(m.ownerErrors) == 0 {
			m.err = nil
			github.WriteRepoCache(m.cfg.Orgs, m.repos)
			if m.screen == loadingScreen {
				m.screen = landingScreen
			}
		} else if m.screen == loadingScreen {
			m.back, m.screen = landingScreen, errorScreen
		}
		return m, nil
	case ticketLoadedMsg:
		if v.url != m.form.ticket {
			return m, nil
		}
		if v.err != nil {
			m.form.ticketStatus = v.err.Error()
			return m, nil
		}
		m.form.name = v.ticket.Title
		m.form.description = v.ticket.Body
		m.form.ticketStatus = "ticket loaded"
		return m, nil
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if k.String() == "ctrl+c" {
		if m.refreshCancel != nil {
			m.refreshCancel()
		}
		return m, tea.Quit
	}
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
		if m.refreshCancel != nil {
			m.refreshCancel()
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
		m.deleteTarget, m.deleteDirty, m.screen = &s, dirty, deleteScreen
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
				m.err, m.back, m.screen = fmt.Errorf("no supported coding agent is installed"), landingScreen, errorScreen
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
		m.deleteTarget, m.deleteDirty, m.screen = nil, nil, landingScreen
	case "enter", "y":
		if m.deleteTarget == nil {
			m.screen = landingScreen
			return m, nil
		}
		if err := session.Delete(m.cfg.Root, *m.deleteTarget); err != nil {
			m.err, m.back, m.screen = err, deleteScreen, errorScreen
			return m, nil
		}
		deleted := m.deleteTarget.Slug
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
		m.deleteTarget, m.deleteDirty, m.screen = nil, nil, landingScreen
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
		if m.assemblyFailed {
			m.screen = assemblyScreen
			m.assemblyFailed = false
			return m, m.startAssembly()
		}
		m.refreshing = true
		m.screen = loadingScreen
		if len(m.ownerErrors) > 0 {
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
		for _, r := range launch.Runners(m.cfg) {
			if (r.ID == m.requestedRunner || r.Command[0] == m.requestedRunner) && r.Path != "" {
				return r, nil
			}
		}
		return launch.Runner{}, fmt.Errorf("runner %q is unavailable", m.requestedRunner)
	}
	if len(m.runners) == 0 || m.runnerCursor >= len(m.runners) {
		return launch.Runner{}, fmt.Errorf("no runner selected")
	}
	return m.runners[m.runnerCursor], nil
}
