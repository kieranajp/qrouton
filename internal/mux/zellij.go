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
	"sync"
	"syscall"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

//go:embed assets/zellij-config.kdl
var zellijAssets embed.FS

const (
	dismissFocusPollInterval = 200 * time.Millisecond
	dismissFocusPollTimeout  = 2 * time.Second
)

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
	// Run-block keybindings (the picker behind Alt-e, de-escalation behind
	// Alt-n, the quick-reference panel behind Alt-?) need the session
	// directory, qrouton's own path, and the global help script's path baked
	// in; the config is already written per-session.
	staged := strings.ReplaceAll(string(config), sessionDirPlaceholder, ws.Dir)
	staged = strings.ReplaceAll(staged, helpScriptPlaceholder, ws.HelpScript)
	staged = strings.ReplaceAll(staged, binaryPlaceholder, ws.Binary)
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

// Start creates the session detached and then attaches to it. Handing the layout
// to a session we are simultaneously attaching to loses it: the client parses the
// layout and sends it to a server that is still booting, and when the terminal
// answers zellij's startup queries in that window the layout instruction lands in
// the server's retry queue — the session comes up with zellij's default layout
// (one pane, both plugin bars) and none of our panes. Creating it detached leaves
// no client traffic to race with, so the layout always lands.
func (z *Zellij) Start(ws Workspace, env []string) error {
	env = WithEnv(env, socketDirEnvVar, z.socketDir)
	create := exec.Command(z.bin, createArgv(zellijConfigPath(ws.Dir), zellijLayoutPath(ws.Dir), ws.Slug)...)
	create.Env, create.Dir = env, ws.Dir
	if out, err := create.CombinedOutput(); err != nil {
		// Creation refuses a name that is already taken (a session the caller's
		// delete could not remove). That one is not a failure — attach to it.
		if state, lookupErr := z.Lookup(ws.Slug); lookupErr != nil || state == SessionMissing {
			return fmt.Errorf("create session %q: %w: %s", ws.Slug, err, strings.TrimSpace(string(out)))
		}
	}
	return z.Attach(ws, env)
}

// createArgv builds the detached-creation arguments. The session is named by
// session_name in the layout; -b creates it without attaching.
func createArgv(config, layout, slug string) []string {
	return []string{configFlag, config, layoutFlag, layout, attachCmd, createBackground, slug}
}

// renderKDL serialises the workspace into a Zellij layout, topped with the
// status bar and named so the new session self-attaches. The bar goes on top
// because qrouton's own strip pane holds the bottom row.
func renderKDL(ws Workspace) string {
	var b strings.Builder
	b.WriteString(kdlLayoutOpen)
	b.WriteString(kdlBar)
	renderNode(&b, ws.Tiled, 1)
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
	if n.Pane.Borderless {
		attrs += kdlBorderless
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

	dismissMu       sync.Mutex
	dismissible     map[string]struct{}
	dismissWatch    sync.Once
	clientID        string
	ownerPaneID     string
	lastDismissMode string
}

// NewZellijHost wires a pane driver to a session; the seam for tests.
func NewZellijHost(bin, session string) PaneHost {
	return &zellijHost{
		bin:             bin,
		session:         session,
		dismissible:     map[string]struct{}{},
		ownerPaneID:     terminalPaneID(os.Getenv(zellijPaneIDEnvVar)),
		lastDismissMode: lockedMode,
	}
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
// typing to the agent, then returns focus to the tiled agent pane — unless
// opts.Focus asks to keep it on the new pane instead.
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
	if !opts.Focus {
		// The new pane is floating and focused; toggling the floating layer off returns
		// focus to the agent while pinned panes stay rendered on top for reference.
		_, _ = z.action(ctx, toggleFloatingAction)
	}
	if opts.DismissOnEsc {
		z.addDismissible(id)
	}
	return id, nil
}

// addDismissible starts one focus watcher per MCP host. Zellij keybindings do
// not expose pane predicates: CloseFocus alone cannot tell a transient pane
// from the agent or shell. The watcher reserves normal mode for a managed
// pane while it has this client's focus; the staged normal-mode Esc binding
// can then close that pane without making Esc dangerous in the tiled layout.
func (z *zellijHost) addDismissible(id string) {
	z.dismissMu.Lock()
	z.dismissible[id] = struct{}{}
	z.dismissMu.Unlock()
	z.dismissWatch.Do(func() { go z.watchDismissibleFocus() })
}

func (z *zellijHost) watchDismissibleFocus() {
	ticker := time.NewTicker(dismissFocusPollInterval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), dismissFocusPollTimeout)
		_ = z.syncDismissMode(ctx)
		cancel()
	}
}

// syncDismissMode changes mode only when the desired state changes. That keeps
// a deliberate Ctrl-g mode entered by the user intact until focus moves, and
// avoids sending redundant switch-mode actions on every watcher tick.
func (z *zellijHost) syncDismissMode(ctx context.Context) error {
	focused, err := z.focusedPane(ctx)
	if err != nil {
		return err
	}
	z.dismissMu.Lock()
	_, dismissible := z.dismissible[focused]
	mode := lockedMode
	if dismissible {
		mode = normalMode
	}
	if mode == z.lastDismissMode {
		z.dismissMu.Unlock()
		return nil
	}
	z.dismissMu.Unlock()
	_, err = z.action(ctx, switchModeAction, mode)
	if err == nil {
		z.dismissMu.Lock()
		z.lastDismissMode = mode
		z.dismissMu.Unlock()
	}
	return err
}

// focusedPane follows the attached client that owned the agent pane when the
// watcher started. list-clients reports the client's actual focused layer,
// unlike list-panes where tiled and floating layers each retain a focused pane.
func (z *zellijHost) focusedPane(ctx context.Context) (string, error) {
	out, err := z.action(ctx, listClientsAction)
	if err != nil {
		return "", err
	}
	clients := parseClientFocus(string(out))
	// switch-mode applies to every connected client. With more than one human
	// attached it cannot safely reserve normal mode for just this client's
	// transient pane, so prefer keeping every permanent pane protected.
	if len(clients) != 1 {
		return "", fmt.Errorf("dismissible pane focus requires one attached zellij client")
	}
	z.dismissMu.Lock()
	defer z.dismissMu.Unlock()
	if z.clientID == "" {
		for _, client := range clients {
			if client.paneID == z.ownerPaneID {
				z.clientID = client.id
				break
			}
		}
		if z.clientID == "" && len(clients) == 1 {
			z.clientID = clients[0].id
		}
	}
	for _, client := range clients {
		if client.id == z.clientID {
			return client.paneID, nil
		}
	}
	return "", fmt.Errorf("owning zellij client is not attached")
}

type clientFocus struct {
	id, paneID string
}

func parseClientFocus(output string) []clientFocus {
	var clients []clientFocus
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == listClientsHeader {
			continue
		}
		clients = append(clients, clientFocus{id: fields[0], paneID: fields[1]})
	}
	return clients
}

func terminalPaneID(id string) string {
	if id == "" || strings.HasPrefix(id, terminalPanePrefix) {
		return id
	}
	return terminalPanePrefix + id
}

// Attached reports whether a client is viewing the session: list-clients
// prints its column header either way, so attachment is a row beyond it.
func (z *zellijHost) Attached(ctx context.Context) (bool, error) {
	out, err := z.action(ctx, listClientsAction)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, listClientsHeader) {
			return true, nil
		}
	}
	return false, nil
}

func (z *zellijHost) Close(ctx context.Context, id string) error {
	z.dismissMu.Lock()
	delete(z.dismissible, id)
	z.dismissMu.Unlock()
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
