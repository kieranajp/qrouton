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

	"github.com/kieranajp/qrouton/internal/sessionpaths"
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
	bin, err := exec.LookPath(zellijBin)
	if err != nil {
		return nil, fmt.Errorf("%w; install Zellij and try again", ErrZellijRequired)
	}
	if err := requireMinimumZellij(bin); err != nil {
		return nil, err
	}
	// macOS $TMPDIR is long enough that zellij's socket path ($TMPDIR/zellij-<uid>/…/<session>)
	// exceeds the 104-byte unix-socket cap for real session names. Pin it somewhere short.
	socketDir := os.Getenv(socketDirEnvVar)
	if socketDir == "" {
		socketDir = defaultSocketDir
	}
	os.Setenv(socketDirEnvVar, socketDir) // Lookup/Kill below must hit the same socket
	return NewZellij(bin, socketDir), nil
}

func requireMinimumZellij(bin string) error {
	out, err := exec.Command(bin, versionFlag).Output()
	if err != nil {
		return fmt.Errorf("check zellij version: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), versionSeparator)
	if len(parts) != versionFieldCount {
		return fmt.Errorf("unrecognized zellij version %q", strings.TrimSpace(string(out)))
	}
	nums := strings.Split(parts[1], versionComponentSep)
	major, _ := strconv.Atoi(nums[0])
	minor := 0
	if len(nums) > 1 {
		minor, _ = strconv.Atoi(nums[1])
	}
	if major == minZellijMajor && minor < minZellijMinor {
		return fmt.Errorf("%w (found %s)", ErrZellijRequired, parts[1])
	}
	return nil
}

func (z *Zellij) Kind() string { return KindZellij }

func (z *Zellij) Handle(slug string) Handle {
	return Handle{Kind: KindZellij, Session: slug, SocketDir: z.socketDir}
}

func zellijConfigPath(dir string) string {
	return filepath.Join(sessionpaths.Dir(dir), zellijConfigName)
}

func zellijLayoutPath(dir string) string {
	return filepath.Join(sessionpaths.Dir(dir), zellijLayoutName)
}

// Stage writes the session's zellij config and rendered layout on every
// launch, so resumed sessions pick up template changes.
func (z *Zellij) Stage(ws Workspace) error {
	if err := os.MkdirAll(sessionpaths.Dir(ws.Dir), 0o755); err != nil {
		return err
	}
	config, err := zellijAssets.ReadFile(configAssetPath)
	if err != nil {
		return err
	}
	// Run-block keybindings (the picker behind Alt-e) need the session
	// directory baked in; the config is already written per-session.
	staged := strings.ReplaceAll(string(config), sessionDirPlaceholder, ws.Dir)
	if err := os.WriteFile(zellijConfigPath(ws.Dir), []byte(staged), 0o644); err != nil {
		return err
	}
	return os.WriteFile(zellijLayoutPath(ws.Dir), []byte(renderKDL(ws)), 0o644)
}

func (z *Zellij) Lookup(slug string) (SessionState, error) {
	// A failing list (no server yet, stale socket) means nothing to attach to.
	out, err := exec.Command(z.bin, listSessionsCmd, listSessionsNoFmt).Output()
	if err != nil {
		return SessionMissing, nil
	}
	for _, l := range strings.Split(string(out), "\n") {
		if f := strings.Fields(l); len(f) > 0 && f[0] == slug {
			if strings.Contains(l, exitedMarker) {
				return SessionDead, nil
			}
			return SessionLive, nil
		}
	}
	return SessionMissing, nil
}

func (z *Zellij) Kill(slug string, force bool) error {
	if force {
		return exec.Command(z.bin, deleteSessionCmd, forceFlag, slug).Run()
	}
	return exec.Command(z.bin, deleteSessionCmd, slug).Run()
}

func (z *Zellij) Attach(ws Workspace, env []string) error {
	env = WithEnv(env, socketDirEnvVar, z.socketDir)
	return execvEnv(z.bin, []string{zellijBin, configFlag, zellijConfigPath(ws.Dir), attachCmd, ws.Slug}, ws.Dir, env)
}

func (z *Zellij) Start(ws Workspace, env []string) error {
	env = WithEnv(env, socketDirEnvVar, z.socketDir)
	// 0.44: -s + -n conflict; the session is named via session_name in the layout itself
	return execvEnv(z.bin, []string{zellijBin, configFlag, zellijConfigPath(ws.Dir), newSessionWithFlag, zellijLayoutPath(ws.Dir)}, ws.Dir, env)
}

// renderKDL serialises the workspace into a Zellij layout, wrapped in the
// tab-bar/status-bar chrome and named so the new session self-attaches.
func renderKDL(ws Workspace) string {
	var b strings.Builder
	b.WriteString(kdlLayoutOpen)
	b.WriteString(kdlTabBar)
	renderNode(&b, ws.Tiled, 1)
	b.WriteString(kdlStatusBar)
	if len(ws.Floating) > 0 {
		b.WriteString(kdlFloatingOpen)
		for _, f := range ws.Floating {
			attrs := fmt.Sprintf(kdlGeometryAttrs, f.Geometry.X, f.Geometry.Y, f.Geometry.Width, f.Geometry.Height, f.Name)
			if f.CloseOnExit {
				attrs += kdlCloseOnExit
			}
			if f.Focus {
				attrs += kdlFocus
			}
			b.WriteString(kdlIndent + kdlIndent + kdlPaneKeyword + " " + attrs + " {\n")
			renderCommand(&b, f.Command, 3)
			b.WriteString(kdlIndent + kdlIndent + kdlBlockClose)
		}
		b.WriteString(kdlIndent + kdlBlockClose)
	}
	b.WriteString(kdlBlockClose)
	b.WriteString(fmt.Sprintf(kdlSessionName, ws.Slug))
	return b.String()
}

func renderNode(b *strings.Builder, n Node, depth int) {
	pad := strings.Repeat(kdlIndent, depth)
	if n.Pane == nil {
		attrs := fmt.Sprintf(kdlSplitAttr, n.Split)
		if s := kdlSize(n.Size); s != "" {
			attrs += kdlSizeAttr + s
		}
		b.WriteString(pad + kdlPaneKeyword + " " + attrs + " {\n")
		for _, child := range n.Children {
			renderNode(b, child, depth+1)
		}
		b.WriteString(pad + kdlBlockClose)
		return
	}
	attrs := ""
	if s := kdlSize(n.Size); s != "" {
		attrs += kdlSizeAttr + s
	}
	attrs += fmt.Sprintf(kdlNameAttr, n.Pane.Name)
	if n.Pane.CloseOnExit {
		attrs += kdlCloseOnExit
	}
	if n.Pane.Focus {
		attrs += kdlFocus
	}
	b.WriteString(pad + kdlPaneKeyword + attrs + " {\n")
	renderCommand(b, n.Pane.Command, depth+1)
	b.WriteString(pad + kdlBlockClose)
}

func renderCommand(b *strings.Builder, command []string, depth int) {
	if len(command) == 0 {
		return
	}
	pad := strings.Repeat(kdlIndent, depth)
	b.WriteString(pad + fmt.Sprintf(kdlCommandFormat, command[0]))
	if len(command) > 1 {
		quoted := make([]string, len(command)-1)
		for i, a := range command[1:] {
			quoted[i] = fmt.Sprintf("%q", a)
		}
		b.WriteString(pad + kdlArgsKeyword + strings.Join(quoted, " ") + "\n")
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
	bin, err := exec.LookPath(zellijBin)
	if err != nil {
		return nil, fmt.Errorf("zellij is unavailable")
	}
	if h.SocketDir != "" {
		os.Setenv(socketDirEnvVar, h.SocketDir)
	}
	return NewZellijHost(bin, h.Session), nil
}

// action runs a zellij action against this session and returns its stdout.
func (z *zellijHost) action(ctx context.Context, args ...string) ([]byte, error) {
	return commandContext(ctx, z.bin, append([]string{sessionFlag, z.session, actionCmd}, args...)...).Output()
}

// Spawn opens a floating, pinned pane so it stays visible while the user keeps
// typing to the agent, then returns focus to the tiled agent pane.
func (z *zellijHost) Spawn(ctx context.Context, opts SpawnOptions) (string, error) {
	args := []string{newPaneAction, floatingFlag, pinnedFlag, trueValue,
		xFlag, opts.Geometry.X, yFlag, opts.Geometry.Y, widthFlag, opts.Geometry.Width, heightFlag, opts.Geometry.Height,
		nameFlag, opts.Label, cwdFlag, opts.Cwd}
	if opts.CloseOnExit {
		args = append(args, closeOnExitFlag)
	}
	args = append(args, endOfFlags)
	args = append(args, opts.Command...)
	out, err := z.action(ctx, args...)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(out))
	// The new pane is floating and focused; toggling the floating layer off returns
	// focus to the agent while pinned panes stay rendered on top for reference.
	_, _ = z.action(ctx, toggleFloatingAction)
	return id, nil
}

func (z *zellijHost) Close(ctx context.Context, id string) error {
	_, err := z.action(ctx, closePaneAction, paneIDFlag, id)
	return err
}

func (z *zellijHost) Capture(ctx context.Context, id string, full bool) (string, error) {
	args := []string{dumpScreenAction, paneIDFlag, id}
	if full {
		args = append(args, fullFlag)
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
