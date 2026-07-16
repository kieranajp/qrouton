package tui

// The new-session form: name/description fields, the role-cycling repository
// picker, branch prefix, and validation. Focus indices 0–6 map to the field
// order rendered by viewForm.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
)

type formState struct {
	name, search, description, ticket string
	owner, prefix                     int
	focus, cursor                     int
	roles                             map[string]repoRole
}

var branchPrefixes = []string{"feat", "fix", "chore", "refactor", "docs", "test"}

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
			m.clampRepoCursor()
		}
		if f.focus == 5 {
			f.prefix = (f.prefix + 5) % 6
		}
		return m, nil
	case "right":
		if f.focus == 2 {
			f.owner = (f.owner + 1) % (len(m.cfg.Orgs) + 1)
			m.clampRepoCursor()
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

func (m appModel) filteredRepos() []github.Repo {
	var out []github.Repo
	q := strings.ToLower(m.form.search)
	for _, r := range m.repos {
		if m.form.owner > 0 && r.Org != m.cfg.Orgs[m.form.owner-1] {
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
	slug := session.Slugify(m.form.name)
	if slug == "" {
		return fmt.Errorf("session name is required")
	}
	// An abandoned half-assembly (interrupted run) doesn't block the name;
	// session.Create reclaims it.
	if dir := filepath.Join(m.cfg.Root, slug); !session.Abandoned(dir) {
		if _, err := os.Stat(dir); err == nil {
			return fmt.Errorf("session %q already exists", slug)
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
	return fmt.Errorf("include at least one active repository")
}
