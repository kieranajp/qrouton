package tui

// Session assembly runs in a goroutine so the TUI stays responsive: progress
// events stream over a channel and the final assembledMsg carries the session
// directory (or the failure). The picker's escalation branch lives here too:
// confirm assembles into the live session, cancel records the outcome alone.

import (
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/config"
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
	if m.pickerManifest != nil {
		cfg, dir, manifest := m.cfg, m.pickerDir, *m.pickerManifest
		name, prefix := m.form.name, branchPrefixes[m.form.prefix]
		selected := m.selectedRepos()
		go func() {
			defer close(ch)
			err := confirmEscalation(cfg, dir, manifest, selected, name, prefix,
				func(p session.Progress) { copy := p; ch <- assemblyEvent{progress: &copy} })
			ch <- assemblyEvent{done: &assembledMsg{dir: dir, err: err}}
		}()
		return awaitAssembly(ch)
	}
	go func() {
		defer close(ch)
		if m.resume != nil {
			s := *m.resume
			err := session.EnsureWorktrees(m.cfg, s)
			ch <- assemblyEvent{done: &assembledMsg{dir: filepath.Join(m.cfg.Root, s.Slug), err: err}}
			return
		}
		dir, err := session.Create(m.cfg, m.form.name, m.form.description, m.form.ticket, branchPrefixes[m.form.prefix], m.form.mode, m.selectedRepos(), func(p session.Progress) { copy := p; ch <- assemblyEvent{progress: &copy} })
		ch <- assemblyEvent{done: &assembledMsg{dir: dir, err: err}}
	}()
	return awaitAssembly(ch)
}

// selectedRepos translates the form's role map into session selections.
func (m *appModel) selectedRepos() []session.RepoSelection {
	var selected []session.RepoSelection
	for _, r := range m.repos {
		switch m.form.roles[r.ID()] {
		case active:
			selected = append(selected, session.RepoSelection{Repo: r, Role: session.RepoRoleActive})
		case reference:
			selected = append(selected, session.RepoSelection{Repo: r, Role: session.RepoRoleReference})
		}
	}
	return selected
}

// confirmEscalation is the picker's confirm path: the composed repositories,
// the work's name, RPI mode, and the confirmed stanza land in one atomic
// manifest write, so a polling reader never sees repos added while the mode
// still says assistant. Active repositories are cut on <prefix>/<slug-of-name>.
func confirmEscalation(cfg *config.Config, dir string, m session.Manifest, sels []session.RepoSelection, name, prefix string, progress session.ProgressFunc) error {
	branch := fmt.Sprintf(branchFormat, prefix, session.Slugify(name))
	out, err := session.ComposeRepos(cfg, m, sels, branch, progress)
	if err != nil {
		return err
	}
	out.Name = name
	out.Mode = session.ModeRPI
	out.Escalation = &session.EscalationOutcome{Status: session.EscalationConfirmed, At: time.Now()}
	return session.WriteManifest(dir, out)
}

// cancelPicker records the cancelled outcome — the stanza alone, mode and
// repositories untouched — and closes the picker.
func (m appModel) cancelPicker() (tea.Model, tea.Cmd) {
	out := *m.pickerManifest
	out.Escalation = &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()}
	if err := session.WriteManifest(m.pickerDir, out); err != nil {
		m.err, m.back, m.screen = err, newScreen, errorScreen
		return m, nil
	}
	if m.refreshCancel != nil {
		m.refreshCancel()
	}
	return m, tea.Quit
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
