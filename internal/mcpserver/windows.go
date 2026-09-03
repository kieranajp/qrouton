package mcpserver

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// pickerPollInterval and pickerTimeout govern awaitPickerOutcome; they are vars
// so tests can shrink them instead of waiting out the real ceiling.
var (
	// ponytail: pickerTimeout is the poll's ceiling — a picker left open longer
	// than this reports back as still-open instead of blocking the agent's tool
	// call forever.
	pickerTimeout        = 30 * time.Minute
	pickerPollInterval   = 2 * time.Second
	viewportWaitTimeout  = 750 * time.Millisecond
	viewportPollInterval = 25 * time.Millisecond
)

// windowManager owns qrouton's slice of the workbench: the editor window plus
// whatever the agent opens. Its registry maps a logical name the agent chooses
// to the workbench's own window id, and reusing a name replaces the window.
type windowManager struct {
	root    string
	editor  launch.EditorCommand
	host    workbench.WindowHost
	mu      sync.Mutex
	claims  uint64
	windows map[string]windowEntry
}

// windowEntry is a registered window, or — while id is empty — an open still in
// flight. claim orders opens racing on one name.
type windowEntry struct {
	id    string
	claim uint64
}

func newWindowManager(root string, editor launch.EditorCommand, host workbench.WindowHost) *windowManager {
	return &windowManager{root: root, editor: editor, host: host, windows: map[string]windowEntry{}}
}

// open replaces any window registered under name with a fresh one, running the
// host round trips off m.mu so one slow open cannot stall every other window
// tool. Opens racing on one name resolve to the later claim; the loser's window
// is closed rather than left behind.
func (m *windowManager) open(ctx context.Context, name string, opts workbench.WindowOptions) (string, error) {
	previous, claim := m.claim(name)
	if previous != "" {
		_ = m.host.Close(ctx, previous)
	}
	id, err := m.host.Open(ctx, opts)
	if err != nil {
		m.release(name, claim)
		return "", err
	}
	if !m.commit(name, claim, id) {
		_ = m.host.Close(ctx, id)
	}
	return id, nil
}

func (m *windowManager) claim(name string) (previous string, claim uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.claims++
	displaced := m.windows[name]
	m.windows[name] = windowEntry{claim: m.claims}
	return displaced.id, m.claims
}

// commit registers an opened window, reporting false if a later claim on the
// same name has already overtaken it.
func (m *windowManager) commit(name string, claim uint64, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.windows[name].claim != claim {
		return false
	}
	m.windows[name] = windowEntry{id: id, claim: claim}
	return true
}

func (m *windowManager) release(name string, claim uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.windows[name]; ok && entry.claim == claim {
		delete(m.windows, name)
	}
}

func (m *windowManager) openFile(ctx context.Context, input openFileInput) (string, *workbench.DocumentViewport, error) {
	span := workbench.LineSpan{Line: input.Line, Through: input.Through}
	opts, err := launch.DocumentWindow(m.root, input.Path, m.editor, span)
	if err != nil {
		return "", nil, err
	}
	opts.Select = resolveForeground(input.Foreground, thoughtsSource(opts.Source))
	id, err := m.open(ctx, editorWindowName, opts)
	if err != nil {
		return "", nil, fmt.Errorf("open file window: %w", err)
	}
	first, last, focused := opts.Span.Bounds()
	if opts.Kind == workbench.KindDocument {
		viewport, err := m.openedViewport(ctx, id, opts.Source, opts.Select)
		if err != nil {
			return "", nil, fmt.Errorf("read opened file viewport: %w", err)
		}
		if !focused {
			return fmt.Sprintf(renderedFileFormat, opts.Source, viewportSummary(viewport)), viewport, nil
		}
		if intersects(viewport, first, last) {
			return fmt.Sprintf(renderedSpanVisibleFormat, opts.Source, lineRange(first, last), viewportSummary(viewport)), viewport, nil
		}
		return fmt.Sprintf(renderedSpanUnverifiedFormat, opts.Source, lineRange(first, last), viewportSummary(viewport)), viewport, nil
	}
	line, _, ok := span.Bounds()
	if !ok {
		line = 1
	}
	return fmt.Sprintf(openedFileFormat, opts.Source, line), nil, nil
}

func resolveForeground(foreground *bool, defaultSelect bool) bool {
	if foreground != nil {
		return *foreground
	}
	return defaultSelect
}

func thoughtsSource(source string) bool {
	clean := filepath.Clean(source)
	if filepath.IsAbs(clean) {
		return false
	}
	parts := strings.Split(clean, string(filepath.Separator))
	return len(parts) > 1 && parts[0] == sessionpaths.ThoughtsDirName
}

func (m *windowManager) openedViewport(ctx context.Context, id, source string, selected bool) (*workbench.DocumentViewport, error) {
	if selected {
		return m.awaitViewport(ctx, id, source)
	}
	viewport, err := m.host.Viewport(ctx, id)
	if err != nil {
		return nil, err
	}
	if viewport == nil {
		return &workbench.DocumentViewport{Source: source, Intervals: []workbench.LineInterval{}}, nil
	}
	copy := *viewport
	copy.Available = false
	copy.Selected = false
	copy.Intervals = []workbench.LineInterval{}
	return &copy, nil
}

func (m *windowManager) awaitViewport(ctx context.Context, id, source string) (*workbench.DocumentViewport, error) {
	last := &workbench.DocumentViewport{Source: source, Intervals: []workbench.LineInterval{}}
	deadline := time.NewTimer(viewportWaitTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(viewportPollInterval)
	defer ticker.Stop()
	for {
		viewport, err := m.host.Viewport(ctx, id)
		if err != nil {
			return nil, err
		}
		if viewport != nil {
			last = viewport
			if last.Intervals == nil {
				last.Intervals = []workbench.LineInterval{}
			}
			if viewport.Selected && viewport.Available {
				return viewport, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return last, nil
		case <-ticker.C:
		}
	}
}

func intersects(viewport *workbench.DocumentViewport, first, last int) bool {
	if viewport == nil || !viewport.Available || !viewport.Selected {
		return false
	}
	for _, interval := range viewport.Intervals {
		if interval.Line <= last && interval.To >= first {
			return true
		}
	}
	return false
}

func lineRange(first, last int) string {
	if first == last {
		return fmt.Sprintf(singleLineFormat, first)
	}
	return fmt.Sprintf(lineRangeFormat, first, last)
}

func (m *windowManager) run(ctx context.Context, input runCommandInput) (string, error) {
	if strings.TrimSpace(input.Command) == "" {
		return "", ErrCommandRequired
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = defaultCommandWindowName
	}
	if name == editorWindowName {
		return "", ErrReservedWindowName
	}
	cwd := m.root
	if trimmed := strings.TrimSpace(input.Cwd); trimmed != "" {
		dir, err := launch.ResolveSessionDir(m.root, trimmed)
		if err != nil {
			return "", err
		}
		cwd = dir
	}
	if _, err := m.open(ctx, name, workbench.WindowOptions{
		Kind:        workbench.KindTerminal,
		Label:       commandWindowLabel + name,
		Cwd:         cwd,
		Command:     []string{shellBin, shellLoginFlag, input.Command},
		CloseOnExit: true,
		Select:      resolveForeground(input.Foreground, false),
	}); err != nil {
		return "", fmt.Errorf("run command: %w", err)
	}
	where := sessionRootScope
	if rel, err := filepath.Rel(m.root, cwd); err == nil && rel != currentDir {
		where = rel
	}
	return fmt.Sprintf(runningFormat, name, where, name, name), nil
}

func (m *windowManager) read(ctx context.Context, input readWindowInput) (string, *workbench.DocumentViewport, error) {
	name := strings.TrimSpace(input.Name)
	id, err := m.liveWindow(ctx, name)
	if err != nil {
		return "", nil, err
	}
	out, err := m.host.Read(ctx, id, input.Full)
	if err != nil {
		return "", nil, fmt.Errorf("read window %q: %w", name, err)
	}
	viewport, err := m.host.Viewport(ctx, id)
	if err != nil {
		return "", nil, fmt.Errorf("read window %q viewport: %w", name, err)
	}
	text := strings.TrimRight(out, "\n")
	if strings.TrimSpace(text) == "" {
		text = fmt.Sprintf(noOutputFormat, name)
	}
	if len(text) > readWindowLimit {
		text = truncatedPrefix + text[len(text)-readWindowLimit:]
	}
	if viewport != nil {
		if viewport.Intervals == nil {
			viewport.Intervals = []workbench.LineInterval{}
		}
		text += "\n\n" + viewportSummary(viewport)
	}
	return text, viewport, nil
}

func viewportSummary(viewport *workbench.DocumentViewport) string {
	if viewport == nil || !viewport.Selected {
		return viewportUnavailableUnselected
	}
	if !viewport.Available {
		return viewportUnavailableSelected
	}
	if len(viewport.Intervals) == 0 {
		return viewportMeasuredEmpty
	}
	ranges := make([]string, 0, len(viewport.Intervals))
	for _, interval := range viewport.Intervals {
		ranges = append(ranges, lineRange(interval.Line, interval.To))
	}
	return fmt.Sprintf(viewportMeasuredFormat, strings.Join(ranges, viewportRangeJoiner))
}

func (m *windowManager) closeWindow(ctx context.Context, input windowNameInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	id, err := m.liveWindow(ctx, name)
	if err != nil {
		return "", err
	}
	m.forget(name, id)
	_ = m.host.Close(ctx, id)
	return fmt.Sprintf(closedFormat, name), nil
}

// liveWindow resolves a registered name to a window that is still open, pruning
// the entry and saying so if it is not. Nothing in the registry learns that the
// user closed a window by hand, or that a command finished and took its window
// with it; without this the agent's next read reaches a dead id and surfaces a
// transport failure instead of a reason.
func (m *windowManager) liveWindow(ctx context.Context, name string) (string, error) {
	m.mu.Lock()
	entry := m.windows[name]
	m.mu.Unlock()
	if entry.id == "" {
		return "", noSuchWindow(name)
	}
	// A failing check is not evidence of absence: keep the window and let the
	// caller's own action report whatever is actually wrong.
	if live, err := m.host.Exists(ctx, entry.id); err == nil && !live {
		m.forget(name, entry.id)
		return "", windowGone(name)
	}
	return entry.id, nil
}

func (m *windowManager) forget(name, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current, exists := m.windows[name]; exists && current.id == id {
		delete(m.windows, name)
	}
}

func (m *windowManager) showDiff(ctx context.Context, input showDiffInput) (string, error) {
	repoAbs, scope, label := "", allReposScope, diffWindowName
	if repo := strings.TrimSpace(input.Repo); repo != "" {
		dir, err := launch.ResolveSessionDir(m.root, repo)
		if err != nil {
			return "", err
		}
		repoAbs, scope, label = dir, repo, diffWindowName+diffWindowSeparator+filepath.Base(dir)
	}
	text := shellOutput(ctx, m.root, diffCommand(repoAbs, strings.TrimSpace(input.Base), input.Staged))
	if strings.TrimSpace(text) == "" {
		text = fmt.Sprintf(emptyDiffFormat, scope)
	}
	if _, err := m.open(ctx, label, workbench.WindowOptions{
		Kind:    workbench.KindDocument,
		Label:   diffWindowLabel + label,
		Content: text,
		Format:  workbench.FormatDiff,
		Select:  resolveForeground(input.Foreground, false),
	}); err != nil {
		return "", fmt.Errorf("show diff: %w", err)
	}
	return fmt.Sprintf(showingDiffFormat, scope, label), nil
}

func (m *windowManager) notify(ctx context.Context, input notifyInput) (string, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return "", ErrMessageRequired
	}
	playSound(sessionpaths.NotifyScript(m.root))
	if _, err := m.open(ctx, notifyWindowName, workbench.WindowOptions{
		Kind:      workbench.KindDocument,
		Label:     notifyWindowLabel,
		Content:   fmt.Sprintf(toastFormat, message),
		Attention: true,
		Select:    resolveForeground(input.Foreground, false),
	}); err != nil {
		return "", fmt.Errorf("notify: %w", err)
	}
	return fmt.Sprintf(notifiedFormat, message), nil
}

// escalate opens the picker pre-filled with name, keeping keyboard focus on it —
// the deliberate exception to the conversation keeping focus, since no agent is
// waiting for the keyboard back once the picker is up. It then blocks until the
// manifest records an escalation outcome newer than the spawn.
//
// On confirm, the agent supervisor kills and relaunches this MCP server's parent
// process, so usually the handoff is the caller disappearing and this call never
// returns. It is a race, not a guarantee: if the relaunch is slow or never
// happens, the poll observes the confirmed outcome and the caller reads the
// message below. The escalation holds either way — the fresh context is owed by
// a marker on disk, not by this process dying.
func (m *windowManager) escalate(ctx context.Context, input escalateInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return "", ErrNameRequired
	}
	spawnedAt := time.Now()
	if err := m.host.Picker(ctx, workbench.PickerRequest{
		SessionRoot: m.root,
		Kind:        workbench.PickerKindEscalate,
		Name:        name,
		Prefix:      strings.TrimSpace(input.BranchPrefix),
		Deadline:    spawnedAt.Add(pickerTimeout),
	}); err != nil {
		return "", fmt.Errorf("escalate: %w", err)
	}
	answer, err := awaitPickerOutcome(ctx, m.root, spawnedAt, workbench.PickerKindEscalate)
	switch {
	case err != nil:
		return "", err
	case !answer.answered:
		return escalationTimeoutMessage, nil
	case answer.status == session.PickerConfirmed:
		return escalationConfirmedMessage, nil
	}
	return escalationCancelledMessage, nil
}

// requestRepos asks the user for repositories the session does not hold, or for
// one it only reads to be taken up for editing. It queues the ordinary picker
// pre-ticked with the request and blocks on the same stanza escalate waits for,
// then hands back the whole resulting set: the user is free to change a role,
// drop something asked for, or add something never mentioned, so the answer is
// the set itself rather than a yes.
func (m *windowManager) requestRepos(ctx context.Context, input requestReposInput) (string, []repoRow, error) {
	requested, reason, err := repoRequest(input)
	if err != nil {
		return "", nil, err
	}
	spawnedAt := time.Now()
	playSound(sessionpaths.NotifyScript(m.root))
	if err := m.host.Picker(ctx, workbench.PickerRequest{
		SessionRoot: m.root,
		Kind:        workbench.PickerKindRepos,
		Requested:   requested,
		Reason:      reason,
		Deadline:    spawnedAt.Add(pickerTimeout),
	}); err != nil {
		return "", nil, fmt.Errorf("request repos: %w", err)
	}
	answer, err := awaitPickerOutcome(ctx, m.root, spawnedAt, workbench.PickerKindRepos)
	if err != nil {
		return "", nil, err
	}
	if !answer.answered {
		rows, err := sessionRepos(m.root)
		if err != nil {
			return "", nil, err
		}
		return reposAnswer(reposStillOpenFormat, requested, rows), rows, nil
	}
	// From the manifest the poll read: the confirm wrote the repositories and the
	// stanza together, so this is the set the user answered with.
	rows := reposFrom(answer.manifest)
	if answer.status == session.PickerConfirmed {
		return reposAnswer(reposConfirmedFormat, requested, rows), rows, nil
	}
	return reposAnswer(reposCancelledFormat, requested, rows), rows, nil
}

// reposAnswer is the resulting set, and then whatever the request asked for that
// the set does not account for. The agent could diff the two itself, but naming
// the shortfall is what stops it asking again for a repository the user dropped
// or a name that matched nothing.
func reposAnswer(format string, requested []workbench.RequestedRepo, rows []repoRow) string {
	message := fmt.Sprintf(format, reposMessage(rows))
	if short := shortfall(requested, rows); len(short) > 0 {
		message += fmt.Sprintf(reposShortfallFormat, strings.Join(short, repoShortfallJoiner))
	}
	return message
}

// shortfall reads the request against the resulting set. Absent covers both a
// name nothing matched and a row the user dropped, which are one fact from here:
// the session does not hold it and asking again unchanged will not help. A
// repository held in a lesser role than the one asked for is the other case,
// because the request was answered but not granted.
func shortfall(requested []workbench.RequestedRepo, rows []repoRow) []string {
	roles := make(map[string]string, len(rows))
	for _, row := range rows {
		roles[strings.ToLower(row.Org+repoIDSeparator+row.Name)] = row.Role
	}
	short := make([]string, 0, len(requested))
	for _, want := range requested {
		id := strings.TrimSpace(want.ID)
		role, held := roles[strings.ToLower(id)]
		switch {
		case !held:
			short = append(short, fmt.Sprintf(shortfallAbsentFormat, id))
		case want.Role == string(session.RepoRoleEditing) && role != string(session.RepoRoleEditing):
			short = append(short, fmt.Sprintf(shortfallRoleFormat, id, role))
		}
	}
	return short
}

// repoRequest validates what the agent asked for. An omitted role reads as
// reference: asking to read is the smaller ask, and a promotion the user did not
// intend costs them a checkout.
func repoRequest(input requestReposInput) ([]workbench.RequestedRepo, string, error) {
	reason := strings.TrimSpace(input.Reason)
	if len(input.Repos) == 0 {
		return nil, "", ErrReposRequired
	}
	if reason == "" {
		return nil, "", ErrReasonRequired
	}
	requested := make([]workbench.RequestedRepo, 0, len(input.Repos))
	for _, want := range input.Repos {
		repo := strings.TrimSpace(want.Repo)
		if repo == "" {
			return nil, "", ErrReposRequired
		}
		role := strings.TrimSpace(want.Role)
		switch session.RepoRole(role) {
		case "":
			role = string(session.RepoRoleReference)
		case session.RepoRoleEditing, session.RepoRoleReference:
		default:
			return nil, "", invalidRequestedRole(role)
		}
		requested = append(requested, workbench.RequestedRepo{ID: repo, Role: role})
	}
	return requested, reason, nil
}

// pickerAnswer is what a poll came back with. answered false is the ceiling:
// nobody confirmed or cancelled, and the manifest is not the user's answer.
type pickerAnswer struct {
	manifest session.Manifest
	status   session.PickerStatus
	answered bool
}

// awaitPickerOutcome polls the manifest for a picker outcome newer than
// spawnedAt, without holding m.mu — a blocking poll must not stall every other
// MCP tool the agent might call while the picker is open.
func awaitPickerOutcome(ctx context.Context, root string, spawnedAt time.Time, kind string) (pickerAnswer, error) {
	ticker := time.NewTicker(pickerPollInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(pickerTimeout)
	defer timeout.Stop()
	for {
		if answer, done := pickerOutcome(root, spawnedAt, kind); done {
			return answer, nil
		}
		select {
		case <-ctx.Done():
			return pickerAnswer{}, ctx.Err()
		case <-timeout.C:
			return pickerAnswer{}, nil
		case <-ticker.C:
		}
	}
}

// pickerOutcome takes only the answer to the kind of picker this caller opened.
// Two can be pending at once — a repository request replaces a waiting
// escalation — and the one stanza they share would otherwise tell the escalate
// poller that a mode change it never got had been confirmed.
func pickerOutcome(root string, spawnedAt time.Time, kind string) (pickerAnswer, bool) {
	m, err := session.Load(root)
	if err != nil || m.Picker == nil || !m.Picker.At.After(spawnedAt) {
		return pickerAnswer{}, false
	}
	// An escalation also answers to the empty kind, which is every stanza written
	// before the field existed; a repository request only ever answers to its own.
	if m.Picker.Kind != kind && !(kind == workbench.PickerKindEscalate && m.Picker.Kind == "") {
		return pickerAnswer{}, false
	}
	return pickerAnswer{manifest: m, status: m.Picker.Status, answered: true}, true
}

// list names the windows still open, dropping any the user has closed. An
// unreadable window list is not evidence of absence, and neither is a window
// opened after that list was taken, so both stand.
func (m *windowManager) list(ctx context.Context) []string {
	m.mu.Lock()
	asked := make(map[string]bool, len(m.windows))
	for _, entry := range m.windows {
		asked[entry.id] = true
	}
	m.mu.Unlock()

	live, err := m.host.List(ctx)
	open := make(map[string]bool, len(live))
	for _, id := range live {
		open[id] = true
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.windows))
	for name, entry := range m.windows {
		if entry.id == "" {
			continue
		}
		if err != nil || open[entry.id] || !asked[entry.id] {
			names = append(names, name)
			continue
		}
		delete(m.windows, name)
	}
	sort.Strings(names)
	return names
}

// diffCommand builds the diff show_diff renders into a document window. Git's
// pager stays off; the all-repos form walks the src/* worktrees.
func diffCommand(repoAbs, base string, staged bool) string {
	flags := ""
	if staged {
		flags += stagedFlag
	}
	if base != "" {
		flags += " " + launch.ShellQuote(base)
	}
	if repoAbs == "" {
		return fmt.Sprintf(allReposDiffFormat, sessionpaths.SrcDirName, flags)
	}
	return fmt.Sprintf(singleRepoDiffFormat, launch.ShellQuote(repoAbs), flags)
}

// shellOutput runs a non-interactive command and returns everything it printed,
// stderr included: a failed git invocation is the diff window's content.
func shellOutput(ctx context.Context, dir, command string) string {
	cmd := exec.CommandContext(ctx, shellBin, shellLoginFlag, command)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// playSound rings the session's attention sound without waiting for it.
var playSound = func(script string) {
	cmd := exec.Command(script)
	if err := cmd.Start(); err != nil {
		return
	}
	go func() { _ = cmd.Wait() }()
}
