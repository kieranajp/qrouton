package mux

// The Zellij adapter. Everything Zellij-specific lives here: the vendored
// config, KDL layout rendering, the 0.44 version gate, socket-dir pinning,
// session lifecycle commands, and the `zellij action` pane driver used by the
// MCP server from inside the session.

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

//go:embed assets/zellij-config.kdl
var zellijAssets embed.FS

// commandContext is swapped by tests to intercept pane-driver invocations.
var commandContext = exec.CommandContext

// Zellij implements Launcher against a zellij binary.
type Zellij struct {
	bin, socketDir string
}

// NewZellij wires the adapter without probing the binary; newZellijLauncher
// is the checked production path, this is the seam for tests.
func NewZellij(bin, socketDir string) *Zellij {
	return &Zellij{bin: bin, socketDir: socketDir}
}

func newZellijLauncher() (*Zellij, error) {
	bin, err := exec.LookPath("zellij")
	if err != nil {
		return nil, fmt.Errorf("zellij 0.44 or newer is required; install Zellij and try again")
	}
	if err := requireZellij044(bin); err != nil {
		return nil, err
	}
	// macOS $TMPDIR is long enough that zellij's socket path ($TMPDIR/zellij-<uid>/…/<session>)
	// exceeds the 104-byte unix-socket cap for real session names. Pin it somewhere short.
	socketDir := os.Getenv("ZELLIJ_SOCKET_DIR")
	if socketDir == "" {
		socketDir = "/tmp/zellij"
	}
	os.Setenv("ZELLIJ_SOCKET_DIR", socketDir) // Lookup/Kill below must hit the same socket
	return NewZellij(bin, socketDir), nil
}

func requireZellij044(bin string) error {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return fmt.Errorf("check zellij version: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), " ")
	if len(parts) != 2 {
		return fmt.Errorf("unrecognized zellij version %q", strings.TrimSpace(string(out)))
	}
	nums := strings.Split(parts[1], ".")
	major, _ := strconv.Atoi(nums[0])
	minor := 0
	if len(nums) > 1 {
		minor, _ = strconv.Atoi(nums[1])
	}
	if major == 0 && minor < 44 {
		return fmt.Errorf("zellij 0.44 or newer is required (found %s)", parts[1])
	}
	return nil
}

func (z *Zellij) Kind() string { return "zellij" }

func (z *Zellij) Handle(slug string) Handle {
	return Handle{Kind: "zellij", Session: slug, SocketDir: z.socketDir}
}

func zellijConfigPath(dir string) string {
	return filepath.Join(dir, ".qrouton", "zellij-config.kdl")
}

func zellijLayoutPath(dir string) string {
	return filepath.Join(dir, ".qrouton", "layout.kdl")
}

// Stage writes the session's zellij config and rendered layout on every
// launch, so resumed sessions pick up template changes.
func (z *Zellij) Stage(ws Workspace) error {
	if err := os.MkdirAll(filepath.Join(ws.Dir, ".qrouton"), 0o755); err != nil {
		return err
	}
	config, err := zellijAssets.ReadFile("assets/zellij-config.kdl")
	if err != nil {
		return err
	}
	if err := os.WriteFile(zellijConfigPath(ws.Dir), config, 0o644); err != nil {
		return err
	}
	return os.WriteFile(zellijLayoutPath(ws.Dir), []byte(renderKDL(ws)), 0o644)
}

func (z *Zellij) Lookup(slug string) (SessionState, error) {
	// A failing list (no server yet, stale socket) means nothing to attach to.
	out, err := exec.Command(z.bin, "list-sessions", "-n").Output()
	if err != nil {
		return SessionMissing, nil
	}
	for _, l := range strings.Split(string(out), "\n") {
		if f := strings.Fields(l); len(f) > 0 && f[0] == slug {
			if strings.Contains(l, "EXITED") {
				return SessionDead, nil
			}
			return SessionLive, nil
		}
	}
	return SessionMissing, nil
}

func (z *Zellij) Kill(slug string, force bool) error {
	if force {
		return exec.Command(z.bin, "delete-session", "--force", slug).Run()
	}
	return exec.Command(z.bin, "delete-session", slug).Run()
}

func (z *Zellij) Attach(ws Workspace, env []string) error {
	env = withEnv(env, "ZELLIJ_SOCKET_DIR", z.socketDir)
	return execvEnv(z.bin, []string{"zellij", "--config", zellijConfigPath(ws.Dir), "attach", ws.Slug}, ws.Dir, env)
}

func (z *Zellij) Start(ws Workspace, env []string) error {
	env = withEnv(env, "ZELLIJ_SOCKET_DIR", z.socketDir)
	// 0.44: -s + -n conflict; the session is named via session_name in the layout itself
	return execvEnv(z.bin, []string{"zellij", "--config", zellijConfigPath(ws.Dir), "--new-session-with-layout", zellijLayoutPath(ws.Dir)}, ws.Dir, env)
}

// renderKDL serialises the workspace into a Zellij layout, wrapped in the
// tab-bar/status-bar chrome and named so the new session self-attaches.
func renderKDL(ws Workspace) string {
	var b strings.Builder
	b.WriteString("layout {\n")
	b.WriteString("    pane size=1 borderless=true {\n        plugin location=\"zellij:tab-bar\"\n    }\n")
	renderNode(&b, ws.Tiled, 1)
	b.WriteString("    pane size=2 borderless=true {\n        plugin location=\"zellij:status-bar\"\n    }\n")
	if len(ws.Floating) > 0 {
		b.WriteString("    floating_panes {\n")
		for _, f := range ws.Floating {
			attrs := fmt.Sprintf("x=%q y=%q width=%q height=%q name=%q", f.Geometry.X, f.Geometry.Y, f.Geometry.Width, f.Geometry.Height, f.Name)
			if f.CloseOnExit {
				attrs += " close_on_exit=true"
			}
			if f.Focus {
				attrs += " focus=true"
			}
			b.WriteString("        pane " + attrs + " {\n")
			renderCommand(&b, f.Command, 3)
			b.WriteString("        }\n")
		}
		b.WriteString("    }\n")
	}
	b.WriteString("}\n")
	b.WriteString(fmt.Sprintf("session_name %q\nattach_to_session true\n", ws.Slug))
	return b.String()
}

func renderNode(b *strings.Builder, n Node, depth int) {
	pad := strings.Repeat("    ", depth)
	if n.Pane == nil {
		attrs := fmt.Sprintf("split_direction=%q", n.Split)
		if s := kdlSize(n.Size); s != "" {
			attrs += " size=" + s
		}
		b.WriteString(pad + "pane " + attrs + " {\n")
		for _, child := range n.Children {
			renderNode(b, child, depth+1)
		}
		b.WriteString(pad + "}\n")
		return
	}
	attrs := ""
	if s := kdlSize(n.Size); s != "" {
		attrs += " size=" + s
	}
	attrs += fmt.Sprintf(" name=%q", n.Pane.Name)
	if n.Pane.CloseOnExit {
		attrs += " close_on_exit=true"
	}
	if n.Pane.Focus {
		attrs += " focus=true"
	}
	b.WriteString(pad + "pane" + attrs + " {\n")
	renderCommand(b, n.Pane.Command, depth+1)
	b.WriteString(pad + "}\n")
}

func renderCommand(b *strings.Builder, command []string, depth int) {
	if len(command) == 0 {
		return
	}
	pad := strings.Repeat("    ", depth)
	b.WriteString(pad + fmt.Sprintf("command %q\n", command[0]))
	if len(command) > 1 {
		quoted := make([]string, len(command)-1)
		for i, a := range command[1:] {
			quoted[i] = fmt.Sprintf("%q", a)
		}
		b.WriteString(pad + "args " + strings.Join(quoted, " ") + "\n")
	}
}

// kdlSize renders a size hint: bare for row counts, quoted for percentages.
func kdlSize(size string) string {
	if size == "" {
		return ""
	}
	if _, err := strconv.Atoi(size); err == nil {
		return size
	}
	return fmt.Sprintf("%q", size)
}

// zellijHost implements PaneHost via `zellij --session <s> action …`.
type zellijHost struct {
	bin, session string
}

// NewZellijHost wires a pane driver to a session; the seam for tests.
func NewZellijHost(bin, session string) PaneHost {
	return &zellijHost{bin: bin, session: session}
}

func zellijHostFromHandle(h Handle) (PaneHost, error) {
	bin, err := exec.LookPath("zellij")
	if err != nil {
		return nil, fmt.Errorf("zellij is unavailable")
	}
	if h.SocketDir != "" {
		os.Setenv("ZELLIJ_SOCKET_DIR", h.SocketDir)
	}
	return NewZellijHost(bin, h.Session), nil
}

// action runs a zellij action against this session and returns its stdout.
func (z *zellijHost) action(ctx context.Context, args ...string) ([]byte, error) {
	return commandContext(ctx, z.bin, append([]string{"--session", z.session, "action"}, args...)...).Output()
}

// Spawn opens a floating, pinned pane so it stays visible while the user keeps
// typing to the agent, then returns focus to the tiled agent pane.
func (z *zellijHost) Spawn(ctx context.Context, opts SpawnOptions) (string, error) {
	args := []string{"new-pane", "--floating", "--pinned", "true",
		"--x", opts.Geometry.X, "--y", opts.Geometry.Y, "--width", opts.Geometry.Width, "--height", opts.Geometry.Height,
		"--name", opts.Label, "--cwd", opts.Cwd}
	if opts.CloseOnExit {
		args = append(args, "--close-on-exit")
	}
	args = append(args, "--")
	args = append(args, opts.Command...)
	out, err := z.action(ctx, args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	// The new pane is floating and focused; toggling the floating layer off returns
	// focus to the agent while pinned panes stay rendered on top for reference.
	_, _ = z.action(ctx, "toggle-floating-panes")
	return id, nil
}

func (z *zellijHost) Close(ctx context.Context, id string) error {
	_, err := z.action(ctx, "close-pane", "--pane-id", id)
	return err
}

func (z *zellijHost) Capture(ctx context.Context, id string, full bool) (string, error) {
	args := []string{"dump-screen", "--pane-id", id}
	if full {
		args = append(args, "--full")
	}
	out, err := z.action(ctx, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func execvEnv(bin string, argv []string, dir string, env []string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return syscall.Exec(bin, argv, env)
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}
