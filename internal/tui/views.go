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
		body = loadingBody
	case newScreen:
		body = m.viewForm()
	case runnerScreen:
		body = m.viewRunners()
	case assemblyScreen:
		body = m.viewAssembly()
	case deleteScreen:
		body = m.viewDelete()
	case errorScreen:
		retry := errorRetryGitHub
		if m.assemblyFailed {
			retry = errorRetryAssemb
		}
		body = bad.Render(errorTitle) + "\n\n" + m.err.Error() + "\n\n" + errorKeyHints + retry + errorKeyQuit
	}
	w := m.bodyWidth()
	header := accent.Render(appName)
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
	status := githubStatusPrefix
	if m.refreshing {
		status += githubStatusRefreshing
	} else if m.err != nil {
		status += githubStatusStale
	} else {
		status += githubStatusConnected
	}
	status += fmt.Sprintf(repoOwnerCountFormat, len(m.repos), len(m.cfg.Orgs))
	if !m.cacheAt.IsZero() {
		status += updatedPrefix + relativeTime(m.cacheAt)
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
	label := "  " + newSessionLabel
	if m.landingCursor == 0 {
		label = accent.Render(glyphSelected + " " + newSessionLabel)
	}
	lines = append(lines, label, "")
	for i, s := range m.sessions {
		repos := make([]string, 0, len(s.Repos))
		for _, r := range s.Repos {
			repos = append(repos, r.Org+"/"+r.Name)
		}
		title := fmt.Sprintf(sessionTitleFormat, s.Slug, relativeTime(s.CreatedAt))
		content := title + "\n" + emptyFallback(s.Description, noDescriptionLabel) + "\n" + strings.Join(repos, " · ") + "\n" + workflowLine(session.Status(m.cfg.Root, s))
		st := card
		if m.landingCursor == i+1 {
			st = picked
		}
		lines = append(lines, st.Width(m.formWidth()).Render(content), "")
	}
	lines = append(lines, dim.Render(landingKeyHints))
	return strings.Join(lines, "\n")
}

func workflowLine(s session.WorkflowStatus) string {
	mark := func(done bool) string {
		if done {
			return good.Render(glyphDone)
		}
		return bad.Render(glyphFailed)
	}
	return fmt.Sprintf(workflowLineFormat, mark(s.Research), mark(s.Plan), mark(s.Implement))
}

func (m appModel) viewDelete() string {
	if m.deleteTarget == nil {
		return deleteNoTarget
	}
	lines := []string{bad.Render(fmt.Sprintf(deleteTitleFormat, m.deleteTarget.Slug)), "", deleteBody}
	if len(m.deleteDirty) > 0 {
		lines = append(lines, "", bad.Render(deleteDirtyBody))
		for _, repo := range m.deleteDirty {
			lines = append(lines, "  • "+repo)
		}
	}
	lines = append(lines, "", dim.Render(deleteKeyHints))
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

	ticket := []string{fieldValue(f.focus == focusTicket, f.ticket)}
	if f.ticketStatus != "" {
		ticket = append(ticket, dim.Render(f.ticketStatus))
	}
	boxes = append(boxes, labeledBox(f.focus == focusTicket, labelTicket, w, ticket...))

	boxes = append(boxes, labeledBox(f.focus == focusName, labelName, w,
		fieldValue(f.focus == focusName, f.name),
		dim.Render(slugPrefix+emptyFallback(slug, emptyFieldLabel))))

	boxes = append(boxes, labeledBox(f.focus == focusDescription, labelDescription, w,
		fieldValue(f.focus == focusDescription, f.description)))

	ownerChips := make([]string, 0, len(m.cfg.Orgs))
	for i, owner := range m.cfg.Orgs {
		token := chip(owner, f.owners[owner])
		if f.focus == focusOwners && i == f.owner {
			token = accent.Render(glyphCursor) + token
		}
		ownerChips = append(ownerChips, token)
	}
	boxes = append(boxes, labeledBox(f.focus == focusOwners, labelOwners, w, strings.Join(ownerChips, " ")))

	repoLines := []string{fmt.Sprintf(repoCountFormat, included, activeN)}
	if f.search != "" {
		repoLines = append(repoLines, dim.Render(filterPrefix)+accent.Render(f.search))
	}
	rs := m.filteredRepos()
	start := 0
	if f.cursor > repoListLead {
		start = f.cursor - repoListLead
	}
	end := min(len(rs), start+repoListWindow)
	for i := start; i < end; i++ {
		r := rs[i]
		role := f.roles[r.ID()]
		marker, label, detail, style := glyphExcluded, roleLabelExcluded, "", dim
		if role == active {
			marker, label, style = glyphActive, roleLabelActive, good
		}
		if role == reference {
			marker, label, detail, style = glyphReference, roleLabelReference,
				fmt.Sprintf(referenceDetailFormat, r.DefaultBranch), accent
		}
		head := style.Render(fmt.Sprintf(roleColumnFormat, marker, label))
		tail := fmt.Sprintf(repoColumnFormat, r.ID(), dim.Render(bulletPrefix+relativeTime(r.PushedAt)+detail))
		row := "  " + head + " " + tail
		if f.focus == focusRepos && i == f.cursor {
			row = accent.Render(glyphCursor+" ") + head + " " + tail
		}
		repoLines = append(repoLines, row)
	}
	boxes = append(boxes, labeledBox(f.focus == focusRepos, labelRepos, w, repoLines...))

	prefixChips := make([]string, len(branchPrefixes))
	for i, p := range branchPrefixes {
		prefixChips[i] = chip(p, i == f.prefix)
	}
	boxes = append(boxes, labeledBox(f.focus == focusPrefix, labelPrefix, w,
		strings.Join(prefixChips, " "),
		dim.Render(fmt.Sprintf(branchPreviewFormat, branchPrefixes[f.prefix], emptyFallback(slug, emptyFieldLabel)))))

	modeChips := []string{
		chip(modeLabelRPI, f.mode != session.ModeAssistant),
		chip(modeLabelAssistant, f.mode == session.ModeAssistant),
	}
	boxes = append(boxes, labeledBox(f.focus == focusMode, labelMode, w,
		strings.Join(modeChips, " "),
		dim.Render(modeHint(f.mode))))

	footer := dim.Render(formKeyHints)
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
		return dim.Render(emptyFieldLabel)
	}
	if focused {
		return accent.Render(value)
	}
	return body.Render(value)
}

func (m appModel) viewRunners() string {
	lines := []string{accent.Render(runnerTitle), ""}
	w := m.formWidth()
	for i, r := range m.runners {
		st := card
		label := body.Render(r.Label)
		if i == m.runnerCursor {
			st = picked
			label = accent.Render(glyphCursor + " " + r.Label)
		}
		lines = append(lines, st.Width(w).Render(label))
	}
	lines = append(lines, "", dim.Render(runnerKeyHints))
	return strings.Join(lines, "\n")
}

func (m appModel) viewAssembly() string {
	name := session.Slugify(m.form.name)
	if m.resume != nil {
		name = m.resume.Slug
	}
	lines := []string{assemblyCreatingPrefix + accent.Render(name), "", good.Render(assemblyConfigured)}
	if m.resume != nil && len(m.assemblySteps) == 0 {
		lines = append(lines, assemblyRestoring)
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
		symbol := glyphPending
		if p.Status == session.ProgressCompleted {
			symbol = glyphDone
		} else if p.Status == session.ProgressFailed {
			symbol = glyphFailed
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
	lines = append(lines, "", dim.Render(assemblyFooter))
	return strings.Join(lines, "\n")
}

func modeHint(mode session.SessionMode) string {
	if mode == session.ModeAssistant {
		return modeHintAssistant
	}
	return modeHintRPI
}

func emptyFallback(s, f string) string {
	if strings.TrimSpace(s) == "" {
		return f
	}
	return s
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return timeUnknown
	}
	d := time.Since(t)
	if d < 0 {
		return "just now"
	}
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf(minutesAgoFormat, int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf(hoursAgoFormat, int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return timeYesterday
	}
	if days < 30 {
		return fmt.Sprintf(daysAgoFormat, days)
	}
	return t.Format(timeDateLayout)
}
