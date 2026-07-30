package tui

// The GitHub repo list refreshes in the background: a token lookup, then one
// concurrent fetch per configured owner. Every async message carries the
// generation it belongs to, so a cancelled or superseded refresh can never
// clobber a newer one's results.

import (
	"context"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/github"
)

// refreshState is the background refresh's own state. gen is the live
// generation — every async message carries the one it belongs to, and a message
// from an older generation is dropped rather than applied.
type refreshState struct {
	active  bool
	ch      <-chan github.RefreshMsg
	gen     int
	cancel  context.CancelFunc
	status  map[string]string
	errs    map[string]error
	cacheAt time.Time
}

type refreshReadyMsg struct {
	gen   int
	token string
	err   error
}

type refreshEventMsg struct {
	gen   int
	event github.RefreshMsg
}

type failedRetryMsg struct {
	gen     int
	repos   []github.Repo
	results map[string]error
}

func refreshTokenCmd(gen int) tea.Cmd {
	return func() tea.Msg {
		token, err := github.Token()
		return refreshReadyMsg{gen: gen, token: token, err: err}
	}
}

func awaitRefresh(gen int, ch <-chan github.RefreshMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return refreshEventMsg{gen: gen, event: github.RefreshMsg{State: github.RefreshComplete}}
		}
		return refreshEventMsg{gen: gen, event: msg}
	}
}

func (m *appModel) beginRefresh() tea.Cmd {
	if m.refresh.cancel != nil {
		m.refresh.cancel()
	}
	m.refresh.gen++
	m.refresh.active = true
	return refreshTokenCmd(m.refresh.gen)
}

func (m *appModel) retryFailed() tea.Cmd {
	if m.refresh.cancel != nil {
		m.refresh.cancel()
	}
	m.refresh.gen++
	gen := m.refresh.gen
	ctx, cancel := context.WithCancel(context.Background())
	m.refresh.cancel = cancel
	m.refresh.active = true
	var owners []string
	for _, owner := range m.cfg.Orgs {
		if m.refresh.errs[owner] != nil {
			owners = append(owners, owner)
			m.refresh.status[owner] = statusFetching
		}
	}
	cached := append([]github.Repo(nil), m.repos...)
	return func() tea.Msg {
		token, err := github.Token()
		results := make(map[string]error)
		if err != nil {
			for _, o := range owners {
				results[o] = err
			}
			return failedRetryMsg{gen: gen, repos: cached, results: results}
		}
		merged := cached
		for _, o := range owners {
			repos, e := github.RefreshOwnerRepos(ctx, http.DefaultClient, token, o)
			results[o] = e
			if e == nil {
				merged = github.ReplaceOwnerRepos(merged, o, repos)
			}
		}
		github.SortReposByActivity(merged)
		return failedRetryMsg{gen: gen, repos: merged, results: results}
	}
}

// onRefreshReady turns a token lookup into the concurrent per-owner fetch.
func (m appModel) onRefreshReady(v refreshReadyMsg) (tea.Model, tea.Cmd) {
	if v.gen != m.refresh.gen {
		return m, nil
	}
	if v.err != nil {
		m.refresh.active = false
		m.err = v.err
		// The loading screen has no failure rendering of its own; without
		// this it would sit on "Loading repositories…" forever.
		if len(m.repos) == 0 || m.screen == loadingScreen {
			m.back, m.screen = landingScreen, errorScreen
			if m.picker.manifest != nil {
				m.back = newScreen // the picker has no landing to go back to
			}
		}
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.refresh.cancel = cancel
	m.refresh.ch = github.RefreshRepos(ctx, http.DefaultClient, v.token, m.cfg.Orgs, m.repos)
	return m, awaitRefresh(v.gen, m.refresh.ch)
}

// onRefreshEvent folds one owner's progress into the model and waits for the
// next, until RefreshComplete closes the run out.
func (m appModel) onRefreshEvent(v refreshEventMsg) (tea.Model, tea.Cmd) {
	if v.gen != m.refresh.gen {
		return m, nil
	}
	event := v.event
	switch event.State {
	case github.RefreshStarted:
		m.refresh.status[event.Owner] = statusFetching
	case github.RefreshSucceeded:
		m.refresh.status[event.Owner] = statusUpdated
		delete(m.refresh.errs, event.Owner)
		if event.Repos != nil {
			m.repos = event.Repos
			m.clampRepoCursor()
		}
	case github.RefreshFailed:
		m.refresh.status[event.Owner] = statusFailed
		m.refresh.errs[event.Owner] = event.Err
		m.err = event.Err
	case github.RefreshComplete:
		m.refresh.active = false
		if len(m.refresh.errs) == 0 {
			m.err = nil
		}
		if event.Repos != nil {
			m.repos = event.Repos
		}
		m.refresh.cacheAt = time.Now()
		if m.screen == loadingScreen && len(m.repos) > 0 {
			m.screen = newScreen
		} else if m.screen == loadingScreen && len(m.refresh.errs) > 0 {
			m.back, m.screen = landingScreen, errorScreen
		}
		return m, nil
	}
	return m, awaitRefresh(v.gen, m.refresh.ch)
}

// onFailedRetry applies a retry of only the owners that had failed, and caches
// the merged list once every one of them has come back clean.
func (m appModel) onFailedRetry(v failedRetryMsg) (tea.Model, tea.Cmd) {
	if v.gen != m.refresh.gen {
		return m, nil
	}
	m.refresh.active = false
	m.repos = v.repos
	for owner, err := range v.results {
		if err != nil {
			m.refresh.errs[owner] = err
			m.refresh.status[owner] = statusFailed
			m.err = err
		} else {
			delete(m.refresh.errs, owner)
			m.refresh.status[owner] = statusUpdated
		}
	}
	if len(m.refresh.errs) == 0 {
		m.err = nil
		github.WriteRepoCache(m.cfg.Orgs, m.repos)
		if m.screen == loadingScreen {
			m.screen = landingScreen
		}
	} else if m.screen == loadingScreen {
		m.back, m.screen = landingScreen, errorScreen
	}
	return m, nil
}
