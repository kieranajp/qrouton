package mux

// The Zellij adapter's session lifecycle: the vendored config, the 0.44 version
// gate, socket-dir pinning, and the create/attach/kill commands. The rest of the
// adapter lives alongside this file — layout rendering in zellij_kdl.go, the
// pane driver in zellij_pane.go and zellij_dismiss.go, and the shell stack in
// zellij_shell.go.

import (
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
	// Run-block keybindings (the shell behind Alt-g, picker behind Alt-e,
	// de-escalation behind Alt-n, and quick-reference panel behind Alt-?)
	// need the session directory, qrouton's own path, and the global help
	// script's path baked in; the config is already written per-session.
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

func execvEnv(bin string, argv []string, dir string, env []string) error {
	if err := os.Chdir(dir); err != nil {
		return err
	}
	return syscall.Exec(bin, argv, env)
}
