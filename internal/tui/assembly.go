package tui

// Session assembly runs in a goroutine so the TUI stays responsive: progress
// events stream over a channel and the final assembledMsg carries the session
// directory (or the failure). The picker's branch lives here too: confirm
// assembles into the live session, escalating it if that is what was asked for.

import (
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
)

// assemblyState is one assembly run as the progress screen sees it: the event
// channel, the steps recorded so far, and whether it ended in a failure the
// error screen can offer to retry.
type assemblyState struct {
	ch     <-chan assemblyEvent
	steps  []session.Progress
	failed bool
}

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
	m.assembly.steps = nil
	ch := make(chan assemblyEvent, 128)
	m.assembly.ch = ch
	if m.picker.manifest != nil {
		cfg, dir, escalate := m.cfg, m.picker.dir, m.picker.escalate
		details := assemblyDetails{name: m.form.name, description: m.form.description,
			ticket: m.form.ticket, branch: m.picker.branch(branchPrefixes[m.form.prefix], m.form.name)}
		selected := m.selectedRepos()
		go func() {
			defer close(ch)
			err := confirmPicker(cfg, dir, selected, details, escalate,
				func(p session.Progress) { copy := p; ch <- assemblyEvent{progress: &copy} })
			ch <- assemblyEvent{done: &assembledMsg{dir: dir, err: err}}
		}()
		return awaitAssembly(ch)
	}
	go func() {
		defer close(ch)
		if m.resume != nil {
			s := *m.resume
			err := session.EnsureWorktrees(m.cfg, s, func(p session.Progress) { copy := p; ch <- assemblyEvent{progress: &copy} })
			ch <- assemblyEvent{done: &assembledMsg{dir: filepath.Join(m.cfg.Root, s.Slug), err: err}}
			return
		}
		dir, err := session.Create(m.cfg, m.form.name, m.form.description, m.form.ticket, branchPrefixes[m.form.prefix], m.form.mode, m.selectedRepos(), func(p session.Progress) { copy := p; ch <- assemblyEvent{progress: &copy} })
		ch <- assemblyEvent{done: &assembledMsg{dir: dir, err: err}}
	}()
	return awaitAssembly(ch)
}

// recordStep folds a progress event into the step list. Clone and fetch report
// repeatedly for the same step, and the view only ever draws the latest of
// each, so a repeated update overwrites its predecessor instead of growing a
// slice the render walks every frame.
func recordStep(steps []session.Progress, p session.Progress) []session.Progress {
	if n := len(steps); n > 0 && p.Status == session.ProgressAdvanced && sameStep(steps[n-1], p) {
		steps[n-1] = p
		return steps
	}
	return append(steps, p)
}

// sameStep reports whether two events describe one operation on one repository.
func sameStep(a, b session.Progress) bool {
	if a.Step != b.Step || a.Status != b.Status {
		return false
	}
	if a.Repo == nil || b.Repo == nil {
		return a.Repo == b.Repo
	}
	return a.Repo.ID() == b.Repo.ID()
}

// selectedRepos translates the form's roles into session selections, in the order
// they were picked. The manifest keeps that order and so does the rail.
func (m *appModel) selectedRepos() []session.RepoSelection {
	byID := make(map[string]github.Repo, len(m.repos))
	for _, r := range m.repos {
		byID[r.ID()] = r
	}
	var selected []session.RepoSelection
	for _, id := range m.form.picked {
		r, ok := byID[id]
		if !ok {
			continue
		}
		switch m.form.roles[id] {
		case active:
			selected = append(selected, session.RepoSelection{Repo: r, Role: session.RepoRoleActive})
		case reference:
			selected = append(selected, session.RepoSelection{Repo: r, Role: session.RepoRoleReference})
		}
	}
	return selected
}

// assemblyDetails is what the picker's form contributes to the manifest.
// Grouped so the write below cannot quietly drop one of them, which is how the
// description and ticket came to be fields that collected input and discarded it.
type assemblyDetails struct {
	name        string
	description string
	ticket      string
	branch      string
}

// confirmPicker is the picker's confirm path: the composed repositories and the
// work's details land in one atomic manifest write. Escalating adds RPI mode and
// the confirmed stanza to that same write, so a polling reader never sees repos
// added while the mode still says assistant.
//
// The branch applies only to repositories being newly added. Anything already in
// the session keeps its worktree and its branch, uncommitted work included: this
// is how work that started small acquires more of the codebase, so the checkout
// it started in is the last thing that should move.
func confirmPicker(cfg *config.Config, dir string, sels []session.RepoSelection, d assemblyDetails, escalate bool, progress session.ProgressFunc) error {
	// Loaded here, not carried in: a picker can sit open for half an hour while
	// the workbench keeps rewriting the manifest underneath it.
	m, err := session.Load(dir)
	if err != nil {
		return err
	}
	composed, err := session.ComposeRepos(cfg, m, sels, d.branch, progress)
	if err != nil {
		return err
	}
	out, err := session.Load(dir)
	if err != nil {
		return err
	}
	out = session.MergeRepos(out, composed.Repos)
	out.Name, out.Description, out.TicketURL = d.name, d.description, d.ticket
	if !escalate {
		// Adding repositories is not a mode change, and the running agent reads
		// the manifest for itself — relaunching it would cost the user a
		// conversation to tell it something it can already see.
		return session.WriteManifest(dir, out)
	}
	out.Mode = session.ModeRPI
	out.Escalation = &session.EscalationOutcome{Status: session.EscalationConfirmed, At: time.Now()}
	if err := session.WriteManifest(dir, out); err != nil {
		return err
	}
	// Best-effort: the supervisor replaces the assistant with a fresh
	// orchestrator; with no supervisor, the mode takes effect next launch.
	launch.SignalSupervisor(dir)
	return nil
}

// cancelPicker records the cancelled outcome — the stanza alone, mode and
// repositories untouched — and closes the picker. Only an escalation has a
// caller waiting on that stanza; the button's cancel is nobody's business.
func (m appModel) cancelPicker() (tea.Model, tea.Cmd) {
	if !m.picker.escalate {
		if m.refresh.cancel != nil {
			m.refresh.cancel()
		}
		return m, tea.Quit
	}
	out, err := session.Load(m.picker.dir)
	if err != nil {
		m.err, m.back, m.screen = err, newScreen, errorScreen
		return m, nil
	}
	out.Escalation = &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()}
	if err := session.WriteManifest(m.picker.dir, out); err != nil {
		m.err, m.back, m.screen = err, newScreen, errorScreen
		return m, nil
	}
	if m.refresh.cancel != nil {
		m.refresh.cancel()
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

// onAssembled ends an assembly run: the picker's write is already the whole
// job, every other path hands a LaunchRequest back to main.
func (m appModel) onAssembled(v assembledMsg) (tea.Model, tea.Cmd) {
	if v.err != nil {
		back := newScreen
		if m.picker.manifest == nil {
			if m.requestedRunner == "" {
				back = runnerScreen
			} else if m.resume != nil {
				back = landingScreen
			}
		}
		m.err, m.assembly.failed, m.back, m.screen = v.err, true, back, errorScreen
		return m, nil
	}
	m.assembly.failed = false
	if m.picker.manifest != nil {
		// The picker's single write is done; there is nothing to launch —
		// the session is already live.
		if m.refresh.cancel != nil {
			m.refresh.cancel()
		}
		return m, tea.Quit
	}
	r, err := m.selectedRunner()
	if err != nil {
		m.err, m.back, m.screen = err, runnerScreen, errorScreen
		return m, nil
	}
	m.result = &LaunchRequest{Dir: v.dir, Runner: r, Resume: m.resume != nil}
	if m.refresh.cancel != nil {
		m.refresh.cancel()
	}
	return m, tea.Quit
}

// onAssemblyEvent records a progress step, or finishes the run when the event
// carries the terminal message. A zero event means the channel closed without
// one, which leaves the model untouched.
func (m appModel) onAssemblyEvent(v assemblyEventMsg) (tea.Model, tea.Cmd) {
	if v.event.progress != nil {
		m.assembly.steps = recordStep(m.assembly.steps, *v.event.progress)
		return m, awaitAssembly(m.assembly.ch)
	}
	if v.event.done != nil {
		return m.onAssembled(*v.event.done)
	}
	return m, nil
}
