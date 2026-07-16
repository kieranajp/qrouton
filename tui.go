package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen uint8

const (
	landingScreen screen = iota
	loadingScreen
	newScreen
	runnerScreen
	assemblyScreen
	errorScreen
)

type repoRole uint8

const (
	excluded repoRole = iota
	active
	reference
)

type launchRequest struct {
	dir    string
	runner Runner
	resume bool
}

type reposLoadedMsg struct {
	repos []Repo
	err   error
}

type refreshReadyMsg struct {
	gen   int
	token string
	err   error
}

type refreshEventMsg struct {
	gen   int
	event repoRefreshMsg
}

type assembledMsg struct {
	dir string
	err error
}

type assemblyEvent struct {
	progress *SessionProgress
	done     *assembledMsg
}
type assemblyEventMsg struct{ event assemblyEvent }
type failedRetryMsg struct {
	repos   []Repo
	results map[string]error
}

type formState struct {
	name, search, description, ticket string
	owner, prefix                     int
	focus, cursor                     int
	roles                             map[string]repoRole
}

type appModel struct {
	cfg             *Config
	sessions        []Manifest
	repos           []Repo
	runners         []Runner
	requestedRunner string
	screen, back    screen
	landingCursor   int
	runnerCursor    int
	width, height   int
	refreshing      bool
	refresh         <-chan repoRefreshMsg
	refreshGen      int
	refreshCancel   context.CancelFunc
	ownerStatus     map[string]string
	ownerErrors     map[string]error
	cacheAt         time.Time
	form            formState
	err             error
	result          *launchRequest
	resume          *Manifest
	assembly        <-chan assemblyEvent
	assemblySteps   []SessionProgress
	assemblyFailed  bool
}

var (
	accent = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	good   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	bad    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	card   = lipgloss.NewStyle().Padding(0, 1).BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).BorderForeground(lipgloss.Color("238"))
	picked = card.Copy().BorderForeground(lipgloss.Color("39")).Background(lipgloss.Color("236"))
)

const fullLogo = `              __________
             /  ·  *   /|
            / *   ·   / |
           /_________/  |
           |  ·   *  |  |
           | *     · |  /
           |  ·   *  | /
           |_________|/

              qrouton`

const compactLogo = `  ____
 /· *_/|
|_* ·|/  qrouton`

func runOnboarding(cfg *Config, sessions []Manifest, requestedRunner string, forceRefresh bool) (*launchRequest, error) {
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

func newAppModel(cfg *Config, sessions []Manifest, requested string) appModel {
	repos, fetched, _ := cachedRepos(cfg.Orgs)
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].CreatedAt.After(sessions[j].CreatedAt) })
	return appModel{cfg: cfg, sessions: sessions, repos: repos, requestedRunner: requested,
		runners: availableRunners(cfg), screen: landingScreen, refreshing: true, cacheAt: fetched,
		ownerStatus: make(map[string]string), ownerErrors: make(map[string]error), refreshGen: 1,
		form: formState{prefix: 0, roles: make(map[string]repoRole)}}
}

func (m appModel) Init() tea.Cmd { return refreshTokenCmd(m.refreshGen) }

func refreshTokenCmd(gen int) tea.Cmd {
	return func() tea.Msg {
		token, err := githubToken()
		return refreshReadyMsg{gen: gen, token: token, err: err}
	}
}

func awaitRefresh(gen int, ch <-chan repoRefreshMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return refreshEventMsg{gen: gen, event: repoRefreshMsg{State: repoRefreshComplete}}
		}
		return refreshEventMsg{gen: gen, event: msg}
	}
}

func (m *appModel) beginRefresh() tea.Cmd {
	if m.refreshCancel != nil {
		m.refreshCancel()
	}
	m.refreshGen++
	m.refreshing = true
	return refreshTokenCmd(m.refreshGen)
}

func (m *appModel) retryFailed() tea.Cmd {
	if m.refreshCancel != nil {
		m.refreshCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.refreshCancel = cancel
	m.refreshing = true
	var owners []string
	for _, owner := range m.cfg.Orgs {
		if m.ownerErrors[owner] != nil {
			owners = append(owners, owner)
			m.ownerStatus[owner] = "fetching…"
		}
	}
	cached := append([]Repo(nil), m.repos...)
	return func() tea.Msg {
		token, err := githubToken()
		results := make(map[string]error)
		if err != nil {
			for _, o := range owners {
				results[o] = err
			}
			return failedRetryMsg{repos: cached, results: results}
		}
		merged := cached
		for _, o := range owners {
			repos, e := refreshOwnerRepos(ctx, http.DefaultClient, token, o)
			results[o] = e
			if e == nil {
				merged = replaceOwnerRepos(merged, o, repos)
			}
		}
		sortReposByActivity(merged)
		return failedRetryMsg{repos: merged, results: results}
	}
}

func availableRunners(cfg *Config) []Runner {
	var out []Runner
	for _, r := range runners(cfg) {
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
			if len(m.repos) == 0 {
				m.back, m.screen = landingScreen, errorScreen
			}
			return m, nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		m.refreshCancel = cancel
		m.refresh = refreshRepos(ctx, http.DefaultClient, v.token, m.cfg.Orgs, m.repos)
		return m, awaitRefresh(v.gen, m.refresh)
	case refreshEventMsg:
		if v.gen != m.refreshGen {
			return m, nil
		}
		event := v.event
		switch event.State {
		case repoRefreshStarted:
			m.ownerStatus[event.Owner] = "fetching…"
		case repoRefreshSucceeded:
			m.ownerStatus[event.Owner] = "updated"
			delete(m.ownerErrors, event.Owner)
			if event.Repos != nil {
				m.repos = event.Repos
				m.clampRepoCursor()
			}
		case repoRefreshFailed:
			m.ownerStatus[event.Owner] = "failed"
			m.ownerErrors[event.Owner] = event.Err
			m.err = event.Err
		case repoRefreshComplete:
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
	case reposLoadedMsg:
		m.refreshing = false
		if v.err != nil {
			m.err = v.err
			if len(m.repos) == 0 {
				m.back, m.screen = landingScreen, errorScreen
			}
		} else {
			m.repos, m.cacheAt, m.err = v.repos, time.Now(), nil
			m.clampRepoCursor()
			if m.screen == loadingScreen {
				m.screen = newScreen
			}
		}
		return m, nil
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
		m.result = &launchRequest{dir: v.dir, runner: r, resume: m.resume != nil}
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
			writeRepoCache(m.cfg.Orgs, m.repos)
			if m.screen == loadingScreen {
				m.screen = landingScreen
			}
		}
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

func (m appModel) updateForm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.form
	switch k.String() {
	case "esc":
		m.screen = landingScreen
		return m, nil
	case "up":
		if f.focus == 4 && f.cursor > 0 {
			f.cursor--
		} else if f.focus > 0 {
			f.focus--
		}
		return m, nil
	case "down":
		if f.focus == 4 && f.cursor+1 < len(m.filteredRepos()) {
			f.cursor++
		} else if f.focus < 6 {
			f.focus++
		}
		return m, nil
	case "shift+tab":
		if f.focus > 0 {
			f.focus--
		} else {
			f.focus = 6
		}
		return m, nil
	case "tab":
		f.focus = (f.focus + 1) % 7
		return m, nil
	case " ":
		if f.focus == 4 {
			m.cycleRepoRole()
			return m, nil
		}
		m.editField(false, " ")
		return m, nil
	case "left":
		if f.focus == 2 {
			f.owner = (f.owner + len(m.cfg.Orgs)) % (len(m.cfg.Orgs) + 1)
		}
		if f.focus == 5 {
			f.prefix = (f.prefix + 5) % 6
		}
		return m, nil
	case "right":
		if f.focus == 2 {
			f.owner = (f.owner + 1) % (len(m.cfg.Orgs) + 1)
		}
		if f.focus == 5 {
			f.prefix = (f.prefix + 1) % 6
		}
		return m, nil
	case "enter":
		if f.focus < 6 {
			f.focus++
			return m, nil
		}
		if err := m.validateForm(); err != nil {
			m.err, m.back, m.screen = err, newScreen, errorScreen
			return m, nil
		}
		if m.requestedRunner != "" {
			m.screen = assemblyScreen
			return m, m.startAssembly()
		}
		if len(m.runners) == 0 {
			m.err, m.back, m.screen = fmt.Errorf("no supported coding agent is installed"), newScreen, errorScreen
			return m, nil
		}
		m.screen = runnerScreen
		return m, nil
	case "backspace":
		m.editField(true, "")
		return m, nil
	}
	if k.Type == tea.KeyRunes {
		m.editField(false, string(k.Runes))
		m.clampRepoCursor()
	}
	return m, nil
}

func (m *appModel) editField(backspace bool, text string) {
	var p *string
	switch m.form.focus {
	case 0:
		p = &m.form.name
	case 1:
		p = &m.form.description
	case 3:
		p = &m.form.search
	case 6:
		p = &m.form.ticket
	}
	if p == nil {
		return
	}
	if backspace {
		r := []rune(*p)
		if len(r) > 0 {
			*p = string(r[:len(r)-1])
		}
	} else {
		*p += text
	}
}

func (m *appModel) clampRepoCursor() {
	n := len(m.filteredRepos())
	if n == 0 {
		m.form.cursor = 0
	} else if m.form.cursor >= n {
		m.form.cursor = n - 1
	}
}

func (m appModel) filteredRepos() []Repo {
	var out []Repo
	q := strings.ToLower(m.form.search)
	for _, r := range m.repos {
		if m.form.owner > 0 && r.Org != m.cfg.Orgs[m.form.owner-1] {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(repoID(r)), q) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (m *appModel) cycleRepoRole() {
	rs := m.filteredRepos()
	if len(rs) == 0 {
		return
	}
	id := repoID(rs[m.form.cursor])
	switch m.form.roles[id] {
	case excluded:
		m.form.roles[id] = active
	case active:
		m.form.roles[id] = reference
	case reference:
		delete(m.form.roles, id)
	}
}

func (m appModel) validateForm() error {
	slug := slugify(m.form.name)
	if slug == "" {
		return fmt.Errorf("session name is required")
	}
	if _, err := os.Stat(filepath.Join(m.cfg.Root, slug)); err == nil {
		return fmt.Errorf("session %q already exists", slug)
	}
	available := make(map[string]bool, len(m.repos))
	for _, r := range m.repos {
		available[repoID(r)] = true
	}
	for id, role := range m.form.roles {
		if role == active && available[id] {
			return nil
		}
	}
	return fmt.Errorf("include at least one active repository")
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

func (m appModel) selectedRunner() (Runner, error) {
	if m.requestedRunner != "" {
		for _, r := range runners(m.cfg) {
			if (r.ID == m.requestedRunner || r.Command[0] == m.requestedRunner) && r.Path != "" {
				return r, nil
			}
		}
		return Runner{}, fmt.Errorf("runner %q is unavailable", m.requestedRunner)
	}
	if len(m.runners) == 0 || m.runnerCursor >= len(m.runners) {
		return Runner{}, fmt.Errorf("no runner selected")
	}
	return m.runners[m.runnerCursor], nil
}

func (m *appModel) startAssembly() tea.Cmd {
	m.assemblySteps = nil
	ch := make(chan assemblyEvent, 128)
	m.assembly = ch
	go func() {
		defer close(ch)
		if m.resume != nil {
			s := *m.resume
			err := ensureWorktrees(m.cfg, s)
			ch <- assemblyEvent{done: &assembledMsg{dir: filepath.Join(m.cfg.Root, s.Slug), err: err}}
			return
		}
		var selected []RepoSelection
		for _, r := range m.repos {
			role := m.form.roles[repoID(r)]
			if role == active {
				selected = append(selected, RepoSelection{Repo: r, Role: RepoRoleActive})
			} else if role == reference {
				selected = append(selected, RepoSelection{Repo: r, Role: RepoRoleReference})
			}
		}
		dir, err := createSessionWithRolesProgress(m.cfg, m.form.name, m.form.description, m.form.ticket, branchPrefixes[m.form.prefix], selected, func(p SessionProgress) { copy := p; ch <- assemblyEvent{progress: &copy} })
		ch <- assemblyEvent{done: &assembledMsg{dir: dir, err: err}}
	}()
	return awaitAssembly(ch)
}

func awaitAssembly(ch <-chan assemblyEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return assemblyEventMsg{}
		}
		return assemblyEventMsg{event: event}
	}
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

var branchPrefixes = []string{"feat", "fix", "chore", "refactor", "docs", "test"}

func (m appModel) View() string {
	var body string
	switch m.screen {
	case landingScreen:
		body = m.viewLanding()
	case loadingScreen:
		body = "Loading repositories…\n\nFetching configured GitHub owners in the background.\n\nesc back"
	case newScreen:
		body = m.viewForm()
	case runnerScreen:
		body = m.viewRunners()
	case assemblyScreen:
		body = m.viewAssembly()
	case errorScreen:
		retry := "retry GitHub"
		if m.assemblyFailed {
			retry = "retry assembly"
		}
		body = bad.Render("Something needs attention") + "\n\n" + m.err.Error() + "\n\n[b] back  [r] " + retry + "  [q] quit"
	}
	w := m.width - 6
	if w < 50 {
		w = 50
	}
	if w > 100 {
		w = 100
	}
	header := accent.Render("qrouton")
	if m.screen == landingScreen {
		logo := compactLogo
		if m.height >= 30 {
			logo = fullLogo
		}
		header = lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(accent.Render(logo))
	}
	return lipgloss.NewStyle().Width(w).Padding(1, 2).Render(header + "\n\n" + body)
}

func (m appModel) viewLanding() string {
	status := "GitHub: "
	if m.refreshing {
		status += "refreshing…"
	} else if m.err != nil {
		status += "cached · refresh failed"
	} else {
		status += "connected"
	}
	status += fmt.Sprintf(" · %d repositories · %d owners", len(m.repos), len(m.cfg.Orgs))
	if !m.cacheAt.IsZero() {
		status += " · updated " + relativeTime(m.cacheAt)
	}
	lines := []string{dim.Render(status), ""}
	if m.refreshing || len(m.ownerErrors) > 0 {
		var statuses []string
		for _, org := range m.cfg.Orgs {
			if s := m.ownerStatus[org]; s != "" {
				entry := org + " " + s
				if err := m.ownerErrors[org]; err != nil {
					entry += " (" + err.Error() + ")"
				}
				statuses = append(statuses, entry)
			}
		}
		if len(statuses) > 0 {
			lines = append(lines, dim.Render(strings.Join(statuses, " · ")), "")
		}
	}
	label := "  New session"
	if m.landingCursor == 0 {
		label = accent.Render("› New session")
	}
	lines = append(lines, label, "")
	for i, s := range m.sessions {
		repos := make([]string, 0, len(s.Repos))
		for _, r := range s.Repos {
			repos = append(repos, r.Org+"/"+r.Name)
		}
		title := fmt.Sprintf("%-42s %s", s.Slug, relativeTime(s.CreatedAt))
		content := title + "\n" + emptyFallback(s.Description, "No description") + "\n" + strings.Join(repos, " · ")
		st := card
		if m.landingCursor == i+1 {
			st = picked
		}
		lines = append(lines, st.Render(content), "")
	}
	lines = append(lines, dim.Render("↑↓ navigate   enter select   r refresh   q quit"))
	return strings.Join(lines, "\n")
}

func (m appModel) viewForm() string {
	f := m.form
	slug := slugify(f.name)
	owner := "All organizations"
	if f.owner > 0 {
		owner = m.cfg.Orgs[f.owner-1]
	}
	included, activeN := 0, 0
	for _, r := range f.roles {
		if r != excluded {
			included++
		}
		if r == active {
			activeN++
		}
	}
	rows := []string{fieldLine(f.focus == 0, "Name", f.name), fieldLine(f.focus == 1, "Description", f.description), "  Session slug   " + dim.Render(emptyFallback(slug, "—")), fieldLine(f.focus == 2, "Organization", owner), fieldLine(f.focus == 3, "Search", f.search), fmt.Sprintf("\n  Repositories   %d included · %d active", included, activeN)}
	rs := m.filteredRepos()
	start := 0
	if f.cursor > 6 {
		start = f.cursor - 6
	}
	end := min(len(rs), start+8)
	for i := start; i < end; i++ {
		r := rs[i]
		role := f.roles[repoID(r)]
		marker, label := "○", "excluded"
		detail := ""
		if role == active {
			marker, label, detail = "●", "active", " → "+branchPrefixes[f.prefix]+"/"+slug
		}
		if role == reference {
			marker, label, detail = "◐", "reference", " → "+r.DefaultBranch+" · reference"
		}
		line := fmt.Sprintf("%s %-10s %-36s pushed %s%s", marker, label, repoID(r), relativeTime(r.PushedAt), detail)
		if f.focus == 4 && i == f.cursor {
			line = accent.Render("› " + line)
		} else {
			line = "  " + line
		}
		rows = append(rows, line)
	}
	rows = append(rows, "", fieldLine(f.focus == 5, "Branch prefix", branchPrefixes[f.prefix]), "  Branch preview "+branchPrefixes[f.prefix]+"/"+emptyFallback(slug, "—")+dim.Render("  active repos only"), fieldLine(f.focus == 6, "Ticket URL", f.ticket), "", dim.Render("↑↓ fields/repos   space cycle role   tab next field   ←→ choice   enter continue   esc back"))
	return strings.Join(rows, "\n")
}

func (m appModel) viewRunners() string {
	lines := []string{"Choose a coding agent", ""}
	for i, r := range m.runners {
		p := "  "
		if i == m.runnerCursor {
			p = "› "
			lines = append(lines, accent.Render(p+r.Label))
			continue
		}
		lines = append(lines, p+r.Label)
	}
	lines = append(lines, "", dim.Render("↑↓ navigate   enter create   esc back"))
	return strings.Join(lines, "\n")
}
func (m appModel) viewAssembly() string {
	name := slugify(m.form.name)
	if m.resume != nil {
		name = m.resume.Slug
	}
	lines := []string{"Creating " + accent.Render(name), "", good.Render("✓ Session configuration")}
	if m.resume != nil && len(m.assemblySteps) == 0 {
		lines = append(lines, "◌ Restore missing worktrees")
	}
	latest := make(map[string]SessionProgress)
	var order []string
	for _, p := range m.assemblySteps {
		key := string(p.Step)
		if p.Repo != nil {
			key += "/" + repoID(*p.Repo)
		}
		if _, ok := latest[key]; !ok {
			order = append(order, key)
		}
		latest[key] = p
	}
	for _, key := range order {
		p := latest[key]
		symbol := "◌"
		if p.Status == SessionProgressCompleted {
			symbol = "✓"
		} else if p.Status == SessionProgressFailed {
			symbol = "✗"
		}
		label := string(p.Step)
		if p.Repo != nil {
			label = repoID(*p.Repo) + " " + label
		}
		line := symbol + " " + label
		if p.Err != nil {
			line += " — " + p.Err.Error()
		}
		if p.Status == SessionProgressCompleted {
			line = good.Render(line)
		} else if p.Status == SessionProgressFailed {
			line = bad.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", dim.Render("Mirrors and worktrees are being assembled…"))
	return strings.Join(lines, "\n")
}

func fieldLine(focused bool, label, value string) string {
	p := "  "
	if focused {
		p = "› "
	}
	value = emptyFallback(value, "—")
	line := fmt.Sprintf("%s%-15s %s", p, label, value)
	if focused {
		return accent.Render(line)
	}
	return line
}
func emptyFallback(s, f string) string {
	if strings.TrimSpace(s) == "" {
		return f
	}
	return s
}
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < 0 {
		return "just now"
	}
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "yesterday"
	}
	if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	}
	return t.Format("2006-01-02")
}
