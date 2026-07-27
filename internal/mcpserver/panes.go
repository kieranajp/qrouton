package mcpserver

// paneManager is the engine behind the MCP tools: it owns qrouton's slice of
// the multiplexer session — the editor pane plus any command panes the agent
// opens. Panes stay visible while the user keeps typing to the agent, and
// every open returns focus to the agent pane (the mux.PaneHost contract).
// The registry maps a logical name to the live backend pane id.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

var (
	editorGeometry  = mux.Geometry{X: "66%", Y: "3%", Width: "33%", Height: "94%"}
	commandGeometry = mux.Geometry{X: "40%", Y: "8%", Width: "58%", Height: "84%"}
	toastGeometry   = mux.Geometry{X: "25%", Y: "5%", Width: "50%", Height: "18%"}

	// pickerGeometry mirrors the Alt-e keybinding's floating picker exactly, so
	// the tool-driven and keyboard-driven routes look identical to the user.
	pickerGeometry = mux.Geometry{X: "20%", Y: "3%", Width: "60%", Height: "94%"}
)

// escalatePollInterval and escalateTimeout govern awaitEscalation; they are
// vars (not consts) so tests can shrink them instead of waiting out the real
// ceiling.
var (
	// ponytail: escalateTimeout is the poll's ceiling — a picker left open
	// longer than this reports back as still-open instead of blocking the
	// agent's tool call forever.
	escalateTimeout      = 30 * time.Minute
	escalatePollInterval = 2 * time.Second
)

type paneManager struct {
	root   string
	editor launch.EditorCommand
	host   mux.PaneHost
	mu     sync.Mutex
	panes  map[string]string
}

func newPaneManager(root string, editor launch.EditorCommand, host mux.PaneHost) *paneManager {
	return &paneManager{root: root, editor: editor, host: host, panes: map[string]string{}}
}

// spawn replaces any pane registered under name with a fresh pane running
// command; the host leaves focus on the agent pane. Callers hold m.mu.
func (m *paneManager) spawn(ctx context.Context, name, label, cwd string, geom mux.Geometry, closeOnExit bool, command []string) (string, error) {
	return m.spawnFocus(ctx, name, label, cwd, geom, closeOnExit, false, command)
}

// spawnFocus is spawn with control over which pane keeps keyboard focus.
// escalate is the one caller that asks for focus=true: no agent is waiting
// for the terminal back once the picker floats. Callers hold m.mu.
func (m *paneManager) spawnFocus(ctx context.Context, name, label, cwd string, geom mux.Geometry, closeOnExit, focus bool, command []string) (string, error) {
	m.closeLocked(ctx, name)
	id, err := m.host.Spawn(ctx, mux.SpawnOptions{Label: label, Cwd: cwd, Geometry: geom, CloseOnExit: closeOnExit, Focus: focus, Command: command})
	if err != nil {
		return "", err
	}
	m.panes[name] = id
	return id, nil
}

// closeLocked closes and forgets the pane registered under name, if any. Callers hold m.mu.
func (m *paneManager) closeLocked(ctx context.Context, name string) {
	if id := m.panes[name]; id != "" {
		_ = m.host.Close(ctx, id)
		delete(m.panes, name)
	}
}

func (m *paneManager) openFile(ctx context.Context, input openFileInput) (string, error) {
	path, err := launch.ResolveSessionFile(m.root, input.Path)
	if err != nil {
		return "", err
	}
	if input.Line < 1 {
		input.Line = 1
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.spawn(ctx, editorPaneName, editorPaneLabel, m.root, editorGeometry, true, m.editor.Args(path, input.Line)); err != nil {
		return "", fmt.Errorf("open editor pane: %w", err)
	}
	rel, _ := filepath.Rel(m.root, path)
	return fmt.Sprintf(openedFileFormat, rel, input.Line), nil
}

func (m *paneManager) run(ctx context.Context, input runCommandInput) (string, error) {
	if strings.TrimSpace(input.Command) == "" {
		return "", ErrCommandRequired
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = defaultCommandPaneName
	}
	if name == editorPaneName {
		return "", ErrReservedPaneName
	}
	cwd := m.root
	if trimmed := strings.TrimSpace(input.Cwd); trimmed != "" {
		dir, err := launch.ResolveSessionDir(m.root, trimmed)
		if err != nil {
			return "", err
		}
		cwd = dir
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.spawn(ctx, name, commandPaneLabel+name, cwd, commandGeometry, input.CloseOnExit, []string{shellBin, shellLoginFlag, input.Command}); err != nil {
		return "", fmt.Errorf("run command: %w", err)
	}
	where := sessionRootScope
	if rel, err := filepath.Rel(m.root, cwd); err == nil && rel != currentDir {
		where = rel
	}
	return fmt.Sprintf(runningFormat, name, where, name, name), nil
}

func (m *paneManager) read(ctx context.Context, input readPaneInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	m.mu.Lock()
	id := m.panes[name]
	m.mu.Unlock()
	if id == "" {
		return "", noSuchPane(name)
	}
	out, err := m.host.Capture(ctx, id, input.Full)
	if err != nil {
		return "", fmt.Errorf("read pane %q: %w", name, err)
	}
	text := strings.TrimRight(out, "\n")
	if strings.TrimSpace(text) == "" {
		return fmt.Sprintf(noOutputFormat, name), nil
	}
	if len(text) > readPaneLimit {
		text = truncatedPrefix + text[len(text)-readPaneLimit:]
	}
	return text, nil
}

func (m *paneManager) closePane(ctx context.Context, input paneNameInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.panes[name] == "" {
		return "", noSuchPane(name)
	}
	m.closeLocked(ctx, name)
	return fmt.Sprintf(closedFormat, name), nil
}

func (m *paneManager) showDiff(ctx context.Context, input showDiffInput) (string, error) {
	repoAbs, scope, label := "", allReposScope, diffPaneName
	if repo := strings.TrimSpace(input.Repo); repo != "" {
		dir, err := launch.ResolveSessionDir(m.root, repo)
		if err != nil {
			return "", err
		}
		repoAbs, scope, label = dir, repo, diffPaneName+diffPaneSeparator+filepath.Base(dir)
	}
	command := diffCommand(repoAbs, strings.TrimSpace(input.Base), input.Staged)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.spawn(ctx, label, diffPaneLabel+label, m.root, commandGeometry, false, []string{shellBin, shellLoginFlag, command}); err != nil {
		return "", fmt.Errorf("show diff: %w", err)
	}
	return fmt.Sprintf(showingDiffFormat, scope, label), nil
}

func (m *paneManager) notify(ctx context.Context, input notifyInput) (string, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return "", ErrMessageRequired
	}
	script := sessionpaths.NotifyScript(m.root)
	command := fmt.Sprintf(toastCommandFormat, launch.ShellQuote(script), launch.ShellQuote(message), toastSeconds)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.spawn(ctx, notifyPaneName, notifyPaneLabel, m.root, toastGeometry, true, []string{shellBin, shellLoginFlag, command}); err != nil {
		return "", fmt.Errorf("notify: %w", err)
	}
	return fmt.Sprintf(notifiedFormat, message), nil
}

// escalate spawns the picker pre-filled with name (and, if given, prefix),
// keeping keyboard focus on it — the deliberate exception to spawn-returns-
// focus, since no agent is waiting for the terminal back once the picker is
// up. It then blocks until the manifest records an escalation outcome newer
// than the spawn.
//
// On confirm, the agent supervisor kills and re-execs this MCP server's
// parent process before the poll below would ever observe the outcome, so in
// practice this call never returns on that path — the handoff *is* the
// caller disappearing. Only cancel and timeout are ever actually seen.
func (m *paneManager) escalate(ctx context.Context, input escalateInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return "", ErrNameRequired
	}
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("escalate: %w", err)
	}
	command := []string{bin, pickSubcommand, sessionRootArg, m.root, nameArg, name}
	if prefix := strings.TrimSpace(input.BranchPrefix); prefix != "" {
		command = append(command, prefixArg, prefix)
	}

	spawnedAt := time.Now()
	m.mu.Lock()
	_, err = m.spawnFocus(ctx, escalatePaneName, escalatePaneLabel, m.root, pickerGeometry, true, true, command)
	m.mu.Unlock()
	if err != nil {
		return "", fmt.Errorf("escalate: %w", err)
	}

	return awaitEscalation(ctx, m.root, spawnedAt)
}

// awaitEscalation polls the manifest for an escalation outcome newer than
// spawnedAt, without holding m.mu — a blocking poll must not stall every
// other MCP tool the agent might call while the picker is open.
func awaitEscalation(ctx context.Context, root string, spawnedAt time.Time) (string, error) {
	ticker := time.NewTicker(escalatePollInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(escalateTimeout)
	defer timeout.Stop()
	for {
		if message, done := escalationOutcome(root, spawnedAt); done {
			return message, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timeout.C:
			return escalationTimeoutMessage, nil
		case <-ticker.C:
		}
	}
}

// escalationOutcome reports the picker's verdict once qrouton.json carries an
// escalation stanza newer than spawnedAt.
func escalationOutcome(root string, spawnedAt time.Time) (string, bool) {
	m, err := session.Load(root)
	if err != nil || m.Escalation == nil || !m.Escalation.At.After(spawnedAt) {
		return "", false
	}
	if m.Escalation.Status == session.EscalationCancelled {
		return escalationCancelledMessage, true
	}
	return escalationConfirmedMessage, true
}

func (m *paneManager) list() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.panes))
	for name := range m.panes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// diffCommand builds the shell that show_diff runs in a pane. A single repo relies
// on git's own pager/colour (the pane is a tty); the all-repos form forces colour
// through an explicit pager as it walks the src/* worktrees. A trailing footer keeps
// an empty diff from rendering as a blank pane.
func diffCommand(repoAbs, base string, staged bool) string {
	flags := ""
	if staged {
		flags += stagedFlag
	}
	if base != "" {
		flags += " " + launch.ShellQuote(base)
	}
	if repoAbs == "" {
		return fmt.Sprintf(allReposDiffFormat, sessionpaths.SrcDirName, flags, diffFooter)
	}
	return fmt.Sprintf(singleRepoDiffFormat, launch.ShellQuote(repoAbs), flags, diffFooter)
}
