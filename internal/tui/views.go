package tui

// All rendering: the screen bodies, styles, logos, and small text helpers.
// Views are pure functions of the model — no state changes here.

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kieranajp/qrouton/internal/session"
)

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
	slug := session.Slugify(f.name)
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
		role := f.roles[r.ID()]
		marker, label := "○", "excluded"
		detail := ""
		if role == active {
			marker, label, detail = "●", "active", " → "+branchPrefixes[f.prefix]+"/"+slug
		}
		if role == reference {
			marker, label, detail = "◐", "reference", " → "+r.DefaultBranch+" · reference"
		}
		line := fmt.Sprintf("%s %-10s %-36s pushed %s%s", marker, label, r.ID(), relativeTime(r.PushedAt), detail)
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
	name := session.Slugify(m.form.name)
	if m.resume != nil {
		name = m.resume.Slug
	}
	lines := []string{"Creating " + accent.Render(name), "", good.Render("✓ Session configuration")}
	if m.resume != nil && len(m.assemblySteps) == 0 {
		lines = append(lines, "◌ Restore missing worktrees")
	}
	latest := make(map[string]session.Progress)
	var order []string
	for _, p := range m.assemblySteps {
		key := string(p.Step)
		if p.Repo != nil {
			key += "/" + p.Repo.ID()
		}
		if _, ok := latest[key]; !ok {
			order = append(order, key)
		}
		latest[key] = p
	}
	for _, key := range order {
		p := latest[key]
		symbol := "◌"
		if p.Status == session.ProgressCompleted {
			symbol = "✓"
		} else if p.Status == session.ProgressFailed {
			symbol = "✗"
		}
		label := string(p.Step)
		if p.Repo != nil {
			label = p.Repo.ID() + " " + label
		}
		line := symbol + " " + label
		if p.Err != nil {
			line += " — " + p.Err.Error()
		}
		if p.Status == session.ProgressCompleted {
			line = good.Render(line)
		} else if p.Status == session.ProgressFailed {
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
