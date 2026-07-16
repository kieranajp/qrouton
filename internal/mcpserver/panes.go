package mcpserver

// paneManager is the engine behind the MCP tools: it owns qrouton's slice of
// the Zellij session — the editor pane plus any command panes the agent opens.
// Panes are floating and pinned so they stay visible while the user keeps
// typing to the agent, and every open returns focus to the tiled agent pane.
// The registry maps a logical name to the live Zellij pane id.

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kieranajp/qrouton/internal/launch"
)

// editorPaneName is the reserved registry key for the single editor pane; other
// panes are keyed by the caller-supplied name.
const editorPaneName = "editor"

// readPaneLimit caps how much pane output read_pane returns to the agent; the tail
// is kept because that is where fresh command output lands.
const readPaneLimit = 20000

// paneGeometry is the floating-pane placement passed to zellij (x, y, width, height).
type paneGeometry struct{ x, y, width, height string }

var (
	editorGeometry  = paneGeometry{x: "66%", y: "3%", width: "33%", height: "94%"}
	commandGeometry = paneGeometry{x: "40%", y: "8%", width: "58%", height: "84%"}
	toastGeometry   = paneGeometry{x: "25%", y: "5%", width: "50%", height: "18%"}
)

type paneManager struct {
	root, zellij, session string
	editor                launch.EditorCommand
	mu                    sync.Mutex
	panes                 map[string]string
}

var commandContext = exec.CommandContext

func newPaneManager(root string, editor launch.EditorCommand, zellij, session string) *paneManager {
	return &paneManager{root: root, editor: editor, zellij: zellij, session: session, panes: map[string]string{}}
}

// action runs a zellij action against this session and returns its stdout.
func (m *paneManager) action(ctx context.Context, args ...string) ([]byte, error) {
	return commandContext(ctx, m.zellij, append([]string{"--session", m.session, "action"}, args...)...).Output()
}

// spawn replaces any pane registered under name with a fresh floating, pinned pane
// running command, then returns focus to the tiled agent pane. Callers hold m.mu.
func (m *paneManager) spawn(ctx context.Context, name, label, cwd string, geom paneGeometry, closeOnExit bool, command []string) (string, error) {
	m.closeLocked(ctx, name)
	args := []string{"new-pane", "--floating", "--pinned", "true",
		"--x", geom.x, "--y", geom.y, "--width", geom.width, "--height", geom.height,
		"--name", label, "--cwd", cwd}
	if closeOnExit {
		args = append(args, "--close-on-exit")
	}
	args = append(args, "--")
	args = append(args, command...)
	out, err := m.action(ctx, args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	m.panes[name] = id
	// The new pane is floating and focused; toggling the floating layer off returns
	// focus to the agent while pinned panes stay rendered on top for reference.
	_, _ = m.action(ctx, "toggle-floating-panes")
	return id, nil
}

// closeLocked closes and forgets the pane registered under name, if any. Callers hold m.mu.
func (m *paneManager) closeLocked(ctx context.Context, name string) {
	if id := m.panes[name]; id != "" {
		_, _ = m.action(ctx, "close-pane", "--pane-id", id)
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
	if _, err := m.spawn(ctx, editorPaneName, "Editor — Alt-f to view · Alt-x to close", m.root, editorGeometry, true, m.editor.Args(path, input.Line)); err != nil {
		return "", fmt.Errorf("open editor pane: %w", err)
	}
	rel, _ := filepath.Rel(m.root, path)
	return fmt.Sprintf("Opened %s at line %d in the editor pane (stays open for reference; focus is back on the agent).", rel, input.Line), nil
}

func (m *paneManager) run(ctx context.Context, input runCommandInput) (string, error) {
	if strings.TrimSpace(input.Command) == "" {
		return "", fmt.Errorf("command is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "command"
	}
	if name == editorPaneName {
		return "", fmt.Errorf("%q is reserved for the editor pane; pick another name", editorPaneName)
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
	if _, err := m.spawn(ctx, name, "▶ "+name, cwd, commandGeometry, input.CloseOnExit, []string{"sh", "-lc", input.Command}); err != nil {
		return "", fmt.Errorf("run command: %w", err)
	}
	where := "the session root"
	if rel, err := filepath.Rel(m.root, cwd); err == nil && rel != "." {
		where = rel
	}
	return fmt.Sprintf("Running in pane %q (cwd %s). Call read_pane with name %q to see its output; close_pane %q to dismiss it.", name, where, name, name), nil
}

func (m *paneManager) read(ctx context.Context, input readPaneInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	m.mu.Lock()
	id := m.panes[name]
	m.mu.Unlock()
	if id == "" {
		return "", fmt.Errorf("no open pane named %q", name)
	}
	args := []string{"dump-screen", "--pane-id", id}
	if input.Full {
		args = append(args, "--full")
	}
	out, err := m.action(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("read pane %q: %w", name, err)
	}
	text := strings.TrimRight(string(out), "\n")
	if strings.TrimSpace(text) == "" {
		return fmt.Sprintf("Pane %q has produced no output yet.", name), nil
	}
	if len(text) > readPaneLimit {
		text = "…(earlier output truncated)…\n" + text[len(text)-readPaneLimit:]
	}
	return text, nil
}

func (m *paneManager) closePane(ctx context.Context, input paneNameInput) (string, error) {
	name := strings.TrimSpace(input.Name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.panes[name] == "" {
		return "", fmt.Errorf("no open pane named %q", name)
	}
	m.closeLocked(ctx, name)
	return fmt.Sprintf("Closed pane %q.", name), nil
}

func (m *paneManager) showDiff(ctx context.Context, input showDiffInput) (string, error) {
	repoAbs, scope, label := "", "all session repos", "diff"
	if repo := strings.TrimSpace(input.Repo); repo != "" {
		dir, err := launch.ResolveSessionDir(m.root, repo)
		if err != nil {
			return "", err
		}
		repoAbs, scope, label = dir, repo, "diff:"+filepath.Base(dir)
	}
	command := diffCommand(repoAbs, strings.TrimSpace(input.Base), input.Staged)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.spawn(ctx, label, "◆ "+label, m.root, commandGeometry, false, []string{"sh", "-lc", command}); err != nil {
		return "", fmt.Errorf("show diff: %w", err)
	}
	return fmt.Sprintf("Showing the diff for %s in pane %q (Alt-f to scroll it).", scope, label), nil
}

func (m *paneManager) notify(ctx context.Context, input notifyInput) (string, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return "", fmt.Errorf("message is required")
	}
	// The toast rings the terminal bell, plays the generated cross-platform sound
	// (best effort), shows the message, then closes itself.
	script := filepath.Join(m.root, ".qrouton", "notify.sh")
	command := fmt.Sprintf(`%s >/dev/null 2>&1 & printf '\a\n  🔔  %%s\n\n  (auto-closes; Alt-x to dismiss)\n' %s; sleep 8`, shellQuote(script), shellQuote(message))
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.spawn(ctx, "notify", "🔔 notification", m.root, toastGeometry, true, []string{"sh", "-lc", command}); err != nil {
		return "", fmt.Errorf("notify: %w", err)
	}
	return fmt.Sprintf("Notified the user: %s", message), nil
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

// shellQuote wraps s so it survives as a single word inside an `sh -lc` string,
// keeping caller-supplied paths, refs, and messages out of the command grammar.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// diffCommand builds the shell that show_diff runs in a pane. A single repo relies
// on git's own pager/colour (the pane is a tty); the all-repos form forces colour
// through an explicit pager as it walks the src/* worktrees. A trailing footer keeps
// an empty diff from rendering as a blank pane.
func diffCommand(repoAbs, base string, staged bool) string {
	flags := ""
	if staged {
		flags += " --staged"
	}
	if base != "" {
		flags += " " + shellQuote(base)
	}
	footer := `printf '\n[end of diff — Alt-x to close]\n'`
	if repoAbs == "" {
		return fmt.Sprintf(`for d in src/*/; do git -C "$d" rev-parse --git-dir >/dev/null 2>&1 || continue; printf '\n=== %%s ===\n' "$d"; git -C "$d" -c color.ui=always diff%s; done | less -FRX; %s`, flags, footer)
	}
	return fmt.Sprintf(`git -C %s diff%s; %s`, shellQuote(repoAbs), flags, footer)
}
