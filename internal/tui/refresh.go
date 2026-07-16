package tui

// The GitHub repo list refreshes in the background: a token lookup, then one
// concurrent fetch per configured owner. Every async message carries the
// generation it belongs to, so a cancelled or superseded refresh can never
// clobber a newer one's results.

import (
	"context"
	"net/http"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/github"
)

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
	m.refreshGen++
	gen := m.refreshGen
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
