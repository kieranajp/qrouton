package tui

// Session assembly runs in a goroutine so the TUI stays responsive: progress
// events stream over a channel and the final assembledMsg carries the session
// directory (or the failure).

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/session"
)

type assembledMsg struct {
	dir string
	err error
}

type assemblyEvent struct {
	progress *session.Progress
	done     *assembledMsg
}

type assemblyEventMsg struct{ event assemblyEvent }

func (m *appModel) startAssembly() tea.Cmd {
	m.assemblySteps = nil
	ch := make(chan assemblyEvent, 128)
	m.assembly = ch
	go func() {
		defer close(ch)
		if m.resume != nil {
			s := *m.resume
			err := session.EnsureWorktrees(m.cfg, s)
			ch <- assemblyEvent{done: &assembledMsg{dir: filepath.Join(m.cfg.Root, s.Slug), err: err}}
			return
		}
		var selected []session.RepoSelection
		for _, r := range m.repos {
			role := m.form.roles[r.ID()]
			if role == active {
				selected = append(selected, session.RepoSelection{Repo: r, Role: session.RepoRoleActive})
			} else if role == reference {
				selected = append(selected, session.RepoSelection{Repo: r, Role: session.RepoRoleReference})
			}
		}
		dir, err := session.Create(m.cfg, m.form.name, m.form.description, m.form.ticket, branchPrefixes[m.form.prefix], m.form.mode, selected, func(p session.Progress) { copy := p; ch <- assemblyEvent{progress: &copy} })
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
