package tui

// The new-session form: name/description fields, the role-cycling repository
// picker, branch prefix, and validation. Focus indices 0–6 map to the field
// order rendered by viewForm.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

type formState struct {
	name, search, description, ticket string
	owner, prefix                     int
	focus, cursor                     int
	mode                              session.SessionMode
	roles                             map[string]repoRole
	owners                            map[string]bool
	ticketStatus                      string
}

// Focus indices, in the order viewForm renders the fields. lastField bounds
// navigation, so adding a field means adding it here and nowhere else.
const (
	focusTicket = iota
	focusName
	focusDescription
	focusOwners
	focusRepos
	focusPrefix
	focusMode

	lastField  = focusMode
	fieldCount = lastField + 1
)

// sessionModes are the mode field's cycle order; RPI leads so it is the default.
var sessionModes = []session.SessionMode{session.ModeRPI, session.ModeAssistant}

func (f *formState) cycleMode() {
	current := 0
	for i, m := range sessionModes {
		if m == f.mode {
			current = i
		}
	}
	f.mode = sessionModes[(current+1)%len(sessionModes)]
}

type ticketLoadedMsg struct {
	url    string
	ticket ticket.Ticket
	err    error
}

func selectedOwners(owners []string) map[string]bool {
	selected := make(map[string]bool, len(owners))
	for _, owner := range owners {
		selected[owner] = true
	}
	return selected
}

var branchPrefixes = []string{"feat", "fix", "chore", "refactor", "docs", "test"}

func (m appModel) updateForm(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := &m.form
	switch k.String() {
	case "esc":
		if m.pickerManifest != nil {
			return m.cancelPicker()
		}
		m.screen = landingScreen
		return m, nil
	case "up":
		if f.focus == focusRepos && f.cursor > 0 {
			f.cursor--
		} else if f.focus > focusTicket {
			f.focus--
		}
		return m, nil
	case "down":
		previous := f.focus
		if f.focus == focusRepos && f.cursor+1 < len(m.filteredRepos()) {
			f.cursor++
		} else if f.focus < lastField {
			f.focus++
		}
		if previous == focusTicket && f.focus != focusTicket {
			return m, m.loadTicket()
		}
		return m, nil
	case "shift+tab":
		if f.focus > focusTicket {
			f.focus--
		} else {
			f.focus = lastField
		}
		return m, nil
	case "tab":
		previous := f.focus
		f.focus = (f.focus + 1) % fieldCount
		if previous == focusTicket {
			return m, m.loadTicket()
		}
		return m, nil
	case " ":
		switch f.focus {
		case focusRepos:
			m.cycleRepoRole()
			return m, nil
		case focusOwners:
			if len(m.cfg.Orgs) > 0 {
				owner := m.cfg.Orgs[f.owner]
				f.owners[owner] = !f.owners[owner]
				m.clampRepoCursor()
				return m, nil
			}
		case focusMode:
			f.cycleMode()
			return m, nil
		}
		m.editField(false, " ")
		return m, nil
	case "left":
		switch f.focus {
		case focusOwners:
			if f.owner > 0 {
				f.owner--
			}
		case focusPrefix:
			f.prefix = (f.prefix + len(branchPrefixes) - 1) % len(branchPrefixes)
		case focusMode:
			f.cycleMode()
		}
		return m, nil
	case "right":
		switch f.focus {
		case focusOwners:
			if f.owner+1 < len(m.cfg.Orgs) {
				f.owner++
			}
		case focusPrefix:
			f.prefix = (f.prefix + 1) % len(branchPrefixes)
		case focusMode:
			f.cycleMode()
		}
		return m, nil
	case "enter":
		if f.focus < lastField {
			previous := f.focus
			f.focus++
			if previous == focusTicket {
				return m, m.loadTicket()
			}
			return m, nil
		}
		if err := m.validateForm(); err != nil {
			m.err, m.back, m.screen = err, newScreen, errorScreen
			return m, nil
		}
		if m.pickerManifest != nil || m.requestedRunner != "" {
			m.screen = assemblyScreen
			return m, m.startAssembly()
		}
		if len(m.runners) == 0 {
			m.err, m.back, m.screen = launch.ErrNoRunnerInstalled, newScreen, errorScreen
			return m, nil
		}
		m.screen = runnerScreen
		return m, nil
	case "backspace":
		m.editField(true, "")
		return m, nil
	}
	if k.Type == tea.KeyRunes {
		if f.focus == focusRepos {
			f.search += string(k.Runes)
		} else {
			m.editField(false, string(k.Runes))
		}
		m.clampRepoCursor()
	}
	return m, nil
}

func (m *appModel) editField(backspace bool, text string) {
	var p *string
	switch m.form.focus {
	case focusTicket:
		p = &m.form.ticket
	case focusName:
		p = &m.form.name
	case focusDescription:
		p = &m.form.description
	case focusRepos:
		p = &m.form.search
	}
	if p == nil {
		return
	}
	if m.form.focus == focusTicket {
		m.form.ticketStatus = ""
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

func (m *appModel) loadTicket() tea.Cmd {
	url := strings.TrimSpace(m.form.ticket)
	if url == "" {
		return nil
	}
	m.form.ticketStatus = "loading ticket…"
	return func() tea.Msg {
		loaded, err := ticket.Fetch(context.Background(), http.DefaultClient, url)
		return ticketLoadedMsg{url: url, ticket: loaded, err: err}
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

func (m appModel) filteredRepos() []github.Repo {
	var out []github.Repo
	q := strings.ToLower(m.form.search)
	for _, r := range m.repos {
		if !m.form.owners[r.Org] {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(r.ID()), q) {
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
	id := rs[m.form.cursor].ID()
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
	if strings.TrimSpace(m.form.ticket) != "" {
		if _, err := ticket.ParseURL(m.form.ticket); err != nil {
			return err
		}
	}
	slug := session.Slugify(m.form.name)
	if slug == "" {
		return errSessionNameEmpty
	}
	// An abandoned half-assembly (interrupted run) doesn't block the name;
	// session.Create reclaims it. The picker names work inside an existing
	// session — its slug only shapes the branch — so no directory check there.
	if m.pickerManifest == nil {
		if dir := filepath.Join(m.cfg.Root, slug); !session.Abandoned(dir) {
			if _, err := os.Stat(dir); err == nil {
				return fmt.Errorf("%w: %q", errSessionExists, slug)
			}
		}
	}
	available := make(map[string]bool, len(m.repos))
	for _, r := range m.repos {
		available[r.ID()] = true
	}
	for id, role := range m.form.roles {
		if role == active && available[id] {
			return nil
		}
	}
	return errNoActiveRepo
}
