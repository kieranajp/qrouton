package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// editorPaneName is the reserved registry key for the single editor pane; other
// panes are keyed by the caller-supplied name.
const editorPaneName = "editor"

// readPaneLimit caps how much pane output read_pane returns to the agent; the tail
// is kept because that is where fresh command output lands.
const readPaneLimit = 20000

type openFileInput struct {
	Path string `json:"path" jsonschema:"Path to an existing file in the qrouton session"`
	Line int    `json:"line,omitempty" jsonschema:"One-based line number; defaults to 1"`
}

type runCommandInput struct {
	Command     string `json:"command" jsonschema:"Shell command to run in a workspace pane the user can watch"`
	Name        string `json:"name,omitempty" jsonschema:"Pane label; reusing a name replaces that pane. Defaults to \"command\""`
	Cwd         string `json:"cwd,omitempty" jsonschema:"Working directory within the session; defaults to the session root"`
	CloseOnExit bool   `json:"close_on_exit,omitempty" jsonschema:"Close the pane automatically when the command exits (default: keep it open)"`
}

type readPaneInput struct {
	Name string `json:"name" jsonschema:"Name of a pane previously opened via run_command or open_file"`
	Full bool   `json:"full,omitempty" jsonschema:"Include the full scrollback instead of just the visible screen"`
}

type paneNameInput struct {
	Name string `json:"name" jsonschema:"Name of a pane previously opened via run_command or open_file"`
}

type showDiffInput struct {
	Repo   string `json:"repo,omitempty" jsonschema:"Repo worktree path within the session (e.g. src/app). Omit to diff every session repo"`
	Staged bool   `json:"staged,omitempty" jsonschema:"Show staged (index) changes instead of the working tree"`
	Base   string `json:"base,omitempty" jsonschema:"Diff against this git ref (e.g. main) instead of the working tree"`
}

type notifyInput struct {
	Message string `json:"message" jsonschema:"Short message to surface to the user, e.g. why you need their attention"`
}

// paneGeometry is the floating-pane placement passed to zellij (x, y, width, height).
type paneGeometry struct{ x, y, width, height string }

var (
	editorGeometry  = paneGeometry{x: "66%", y: "3%", width: "33%", height: "94%"}
	commandGeometry = paneGeometry{x: "40%", y: "8%", width: "58%", height: "84%"}
	toastGeometry   = paneGeometry{x: "25%", y: "5%", width: "50%", height: "18%"}
)

// paneManager owns qrouton's slice of the Zellij session: the editor pane plus any
// command panes the agent opens. Panes are floating and pinned so they stay visible
// while the user keeps typing to the agent, and every open returns focus to the tiled
// agent pane. The registry maps a logical name to the live Zellij pane id.
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

// textResult wraps a message as both an MCP text block and a structured payload,
// matching the shape callers already expect from open_file.
func textResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}
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

func newMCPServer(root string, editor launch.EditorCommand, zellij, session string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "qrouton", Version: "1"}, &mcp.ServerOptions{
		Instructions: "Drive the user's qrouton workspace. Panes you open are floating, pinned, and leave focus on the agent, so the user can watch them while chatting. Use open_file to show a document (especially after creating one); run_command to run long-lived or noisy work (dev servers, watchers, builds, logs) in a visible pane instead of your own shell; read_pane to inspect that output; show_diff to display a repo's changes for review; notify to get the user's attention when you finish or need them; close_pane/list_panes to manage them. All paths and working directories must belong to this session.",
	})
	pane := newPaneManager(root, editor, zellij, session)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_file",
		Description: "Open an existing session file in the user's configured terminal editor pane. The pane stays open for reference while the user keeps chatting with the agent. Use this after creating a document when showing it to the user is helpful.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input openFileInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.openFile(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_command",
		Description: "Run a shell command in a visible workspace pane instead of your own shell. Ideal for long-running or noisy processes (dev servers, test watchers, builds, log tails) the user should see live. The pane is floating and pinned, focus stays on the agent, and reusing a name replaces that pane. Read its output later with read_pane.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input runCommandInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.run(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_pane",
		Description: "Capture the current output of a pane opened with run_command (or open_file) and return it as text. Use this to check on a command you started — for example to confirm a dev server booted or to read a test run's failures. Set full to include the scrollback.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input readPaneInput) (*mcp.CallToolResult, any, error) {
		text, err := pane.read(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(text), map[string]any{"output": text}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "show_diff",
		Description: "Show a repo's git diff in a workspace pane for the user to review. Give repo as a worktree path within the session (e.g. src/app), or omit it to diff every session repo. Use base to compare against a ref (e.g. the default branch) or staged for index changes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input showDiffInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.showDiff(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "notify",
		Description: "Get the user's attention with an on-screen toast, the terminal bell, and a sound. Use this sparingly — when you finish a long task, need a decision, or are blocked — since the user may have stepped away while work runs.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input notifyInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.notify(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "close_pane",
		Description: "Close a pane previously opened with run_command or open_file, by name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input paneNameInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.closePane(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_panes",
		Description: "List the panes qrouton is currently managing for you, by name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		names := pane.list()
		message := "No qrouton-managed panes are open."
		if len(names) > 0 {
			message = "Open panes: " + strings.Join(names, ", ") + "."
		}
		return textResult(message), map[string]any{"panes": names}, nil
	})

	return server
}

// Run serves the qrouton MCP server over stdio. editorJSON is the resolved
// EditorCommand marshalled by the launcher (or inherited via QROUTON_EDITOR_JSON).
func Run(root, editorJSON, zellijSession, socketDir string) error {
	os.Setenv("ZELLIJ_SOCKET_DIR", socketDir)
	var editor launch.EditorCommand
	if err := json.Unmarshal([]byte(editorJSON), &editor); err != nil || len(editor.Argv) == 0 {
		return fmt.Errorf("mcp: invalid inherited editor configuration")
	}
	zellij, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("mcp: zellij is unavailable")
	}
	return newMCPServer(root, editor, zellij, zellijSession).Run(context.Background(), &mcp.StdioTransport{})
}
