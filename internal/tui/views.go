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
	case deleteScreen:
		body = m.viewDelete()
	case errorScreen:
		retry := "retry GitHub"
		if m.assemblyFailed {
			retry = "retry assembly"
		}
		body = bad.Render("Something needs attention") + "\n\n" + m.err.Error() + "\n\n[b] back  [r] " + retry + "  [q] quit"
	}
	w := m.bodyWidth()
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
		content := title + "\n" + emptyFallback(s.Description, "No description") + "\n" + strings.Join(repos, " · ") + "\n" + workflowLine(session.Status(m.cfg.Root, s))
		st := card
		if m.landingCursor == i+1 {
			st = picked
		}
		lines = append(lines, st.Width(m.formWidth()).Render(content), "")
	}
	lines = append(lines, dim.Render("↑↓ navigate   enter select   d delete   r refresh   q quit"))
	return strings.Join(lines, "\n")
}

func workflowLine(s session.WorkflowStatus) string {
	mark := func(done bool) string {
		if done {
			return good.Render("✓")
		}
		return bad.Render("✗")
	}
	return fmt.Sprintf("R %s   P %s   I %s", mark(s.Research), mark(s.Plan), mark(s.Implement))
}

func (m appModel) viewDelete() string {
	if m.deleteTarget == nil {
		return "No session selected.\n\n[esc] back"
	}
	lines := []string{bad.Render("Delete " + m.deleteTarget.Slug + "?"), "", "This removes its worktrees and session files. Shared mirrors are kept."}
	if len(m.deleteDirty) > 0 {
		lines = append(lines, "", bad.Render("Uncommitted files will be lost in:"))
		for _, repo := range m.deleteDirty {
			lines = append(lines, "  • "+repo)
		}
	}
	lines = append(lines, "", dim.Render("enter/y delete   esc/n cancel"))
	return strings.Join(lines, "\n")
}

func (m appModel) viewForm() string {
	f := m.form
	slug := session.Slugify(f.name)
	w := m.formWidth()

	included, activeN := 0, 0
	for _, r := range f.roles {
		if r != excluded {
			included++
		}
		if r == active {
			activeN++
		}
	}

	var boxes []string

	ticket := []string{fieldValue(f.focus == 0, f.ticket)}
	if f.ticketStatus != "" {
		ticket = append(ticket, dim.Render(f.ticketStatus))
	}
	boxes = append(boxes, labeledBox(f.focus == 0, "Ticket URL", w, ticket...))

	boxes = append(boxes, labeledBox(f.focus == 1, "Name", w,
		fieldValue(f.focus == 1, f.name),
		dim.Render("slug · "+emptyFallback(slug, "—"))))

	boxes = append(boxes, labeledBox(f.focus == 2, "Description", w,
		fieldValue(f.focus == 2, f.description)))

	ownerChips := make([]string, 0, len(m.cfg.Orgs))
	for i, owner := range m.cfg.Orgs {
		token := chip(owner, f.owners[owner])
		if f.focus == 3 && i == f.owner {
			token = accent.Render("▸") + token
		}
		ownerChips = append(ownerChips, token)
	}
	boxes = append(boxes, labeledBox(f.focus == 3, "GitHub owners", w, strings.Join(ownerChips, " ")))

	repoLines := []string{fmt.Sprintf("%d included · %d active", included, activeN)}
	if f.search != "" {
		repoLines = append(repoLines, dim.Render("filter · ")+accent.Render(f.search))
	}
	rs := m.filteredRepos()
	start := 0
	if f.cursor > 6 {
		start = f.cursor - 6
	}
	end := min(len(rs), start+8)
	for i := start; i < end; i++ {
		r := rs[i]
		role := f.roles[r.ID()]
		marker, label, detail, style := "○", "excluded", "", dim
		if role == active {
			marker, label, style = "●", "active", good
		}
		if role == reference {
			marker, label, detail, style = "◐", "reference", " → "+r.DefaultBranch+" · reference", accent
		}
		head := style.Render(fmt.Sprintf("%s %-9s", marker, label))
		tail := fmt.Sprintf("%-26s %s", r.ID(), dim.Render("· "+relativeTime(r.PushedAt)+detail))
		row := "  " + head + " " + tail
		if f.focus == 4 && i == f.cursor {
			row = accent.Render("▸ ") + head + " " + tail
		}
		repoLines = append(repoLines, row)
	}
	boxes = append(boxes, labeledBox(f.focus == 4, "Repositories", w, repoLines...))

	prefixChips := make([]string, len(branchPrefixes))
	for i, p := range branchPrefixes {
		prefixChips[i] = chip(p, i == f.prefix)
	}
	boxes = append(boxes, labeledBox(f.focus == 5, "Branch prefix", w,
		strings.Join(prefixChips, " "),
		dim.Render("preview · "+branchPrefixes[f.prefix]+"/"+emptyFallback(slug, "—")+"  (active repos only)")))

	modeChips := []string{
		chip("RPI", f.mode != session.ModeAssistant),
		chip("Assistant", f.mode == session.ModeAssistant),
	}
	boxes = append(boxes, labeledBox(f.focus == 6, "Mode", w,
		strings.Join(modeChips, " "),
		dim.Render(modeHint(f.mode))))

	footer := dim.Render("type in the repository list to filter · backspace clears it\n↑↓ move · space select/cycle · tab next field · ←→ choice · enter continue · esc back")
	return strings.Join(boxes, "\n") + "\n\n" + footer
}

// bodyWidth is the width View() wraps the whole body to, clamped so a narrow
// terminal stays readable and a wide one does not stretch into unreadable lines.
func (m appModel) bodyWidth() int {
	w := m.width - bodyWidthInset
	if w < minBodyWidth {
		return minBodyWidth
	}
	if w > maxBodyWidth {
		return maxBodyWidth
	}
	return w
}

// formWidth is the width passed to each box. View() wraps the body to its Width
// minus horizontal padding (w-4); a lipgloss box renders 2 wider than its Width
// (the border sits outside it), so w-6 makes a box total exactly w-4 and fill
// the content area without overflowing into a wrap.
func (m appModel) formWidth() int {
	return m.bodyWidth() - boxWidthInset
}

// labeledBox draws one form field as a titled cube: the label heads the box
// (accent when focused), the value lines sit below.
func labeledBox(focused bool, label string, width int, lines ...string) string {
	title := dim.Render(label)
	if focused {
		title = accent.Render(label)
	}
	content := append([]string{title}, lines...)
	return box(focused).Width(width).Render(strings.Join(content, "\n"))
}

// fieldValue renders a text field's value: a dim placeholder when empty, accent
// when the field has focus, quiet body text otherwise.
func fieldValue(focused bool, value string) string {
	if strings.TrimSpace(value) == "" {
		return dim.Render("—")
	}
	if focused {
		return accent.Render(value)
	}
	return body.Render(value)
}

func (m appModel) viewRunners() string {
	lines := []string{accent.Render("Choose a coding agent"), ""}
	w := m.formWidth()
	for i, r := range m.runners {
		st := card
		label := body.Render(r.Label)
		if i == m.runnerCursor {
			st = picked
			label = accent.Render("▸ " + r.Label)
		}
		lines = append(lines, st.Width(w).Render(label))
	}
	lines = append(lines, "", dim.Render("↑↓ navigate · enter create · esc back"))
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

func modeHint(mode session.SessionMode) string {
	if mode == session.ModeAssistant {
		return "open-ended coding session · escalate to RPI anytime"
	}
	return "orchestrated Research → Plan → Implement workflow"
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
