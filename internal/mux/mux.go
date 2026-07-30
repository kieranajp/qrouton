// Package mux defines qrouton's multiplexer ports. The launcher and the MCP
// pane tools never talk to a terminal multiplexer directly; they speak these
// interfaces, and an adapter (today: Zellij) supplies the mechanics. The
// backend's identity crosses the exec boundary as a marshalled Handle, the
// same way EditorCommand travels into the MCP child.
package mux

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SessionState is a Launcher's view of a named session.
type SessionState int

const (
	SessionMissing SessionState = iota // no session by that name
	SessionLive                        // running; attaching would join it
	SessionDead                        // recorded but exited; must be deleted before recreating
)

// Launcher provisions and enters workspaces. Attach and Start replace the
// current process on success (they exec the multiplexer) and only return on
// failure.
type Launcher interface {
	// Kind names the backend, e.g. "zellij".
	Kind() string
	// Handle returns the identity the MCP child needs to reach this
	// backend's session from inside the workspace.
	Handle(slug string) Handle
	// Stage materialises backend files for the workspace (layouts, config)
	// under <ws.Dir>/.qrouton. Run on every launch so resumed sessions pick
	// up template changes.
	Stage(ws Workspace) error
	// Lookup reports whether a session by this slug exists and is alive.
	Lookup(slug string) (SessionState, error)
	// Kill removes a session; force applies to live sessions.
	Kill(slug string, force bool) error
	// Attach joins the live session for ws. Execs on success.
	Attach(ws Workspace, env []string) error
	// Start creates a fresh session from the staged workspace. Execs on success.
	Start(ws Workspace, env []string) error
}

// PaneHost drives panes inside a running session on the agent's behalf. Spawn
// opens a pane the user can watch and leaves keyboard focus on the agent.
type PaneHost interface {
	Spawn(ctx context.Context, opts SpawnOptions) (id string, err error)
	Close(ctx context.Context, id string) error
	// Capture returns the pane's current output; full includes scrollback.
	Capture(ctx context.Context, id string, full bool) (string, error)
	// Attached reports whether a user's client is viewing the session. A
	// floating pane's percentage geometry is resolved against the attached
	// client's viewport at spawn time, so a pane opened before anyone is
	// looking comes up sized for the server's own default instead.
	Attached(ctx context.Context) (bool, error)
	// Reposition re-resolves a floating pane's geometry against the viewport as
	// it stands now. It is the repair for a pane spawned while nobody was
	// looking, whose percentages resolved against the server's own default.
	Reposition(ctx context.Context, id string, geom Geometry) error
	// Exists reports whether a pane id is still live in the session. A pane the
	// user closed by hand leaves a registry entry behind otherwise, and the
	// agent's next read of it fails with a backend error rather than a reason.
	Exists(ctx context.Context, id string) (bool, error)
}

// ShellStack is the small piece of pane control used by qrouton's interactive
// shell command. A new shell begins wherever Zellij's Run action can place it,
// then joins the canonical shell stack before handing the pane to the user's
// login shell. Count lets the final shell preserve the permanent right-hand
// region instead of closing it.
type ShellStack interface {
	JoinCurrent(ctx context.Context, titlePrefix, titleSuffix string) (number int, err error)
	Count(ctx context.Context, titlePrefix string) (int, error)
}

// Geometry places a floating pane, in the backend's units (percent strings).
type Geometry struct {
	X, Y, Width, Height string
}

// SpawnOptions describes a runtime pane opened through a PaneHost. Focus keeps
// keyboard focus on the spawned pane instead of the default
// spawn-returns-focus-to-the-agent behaviour.
//
// Nothing here says "dismissible": a pane the user can dismiss with Esc is one
// whose own command waits for Esc and then exits, which CloseOnExit turns into
// a closed pane. The backend is not told, because it does not need to be — see
// launch.DismissCommand.
type SpawnOptions struct {
	Label       string
	Cwd         string
	Geometry    Geometry
	CloseOnExit bool
	Focus       bool
	Command     []string
}

// Pane is one terminal in a workspace layout, running Command. Borderless
// drops the pane's frame — how a one-row strip renders as a bar rather than a
// framed pane.
type Pane struct {
	Name        string
	Command     []string
	CloseOnExit bool
	Focus       bool
	Borderless  bool
}

// Node is an element of the tiled layout tree: a leaf Pane, or a split with
// Children. Size is a backend-rendered hint — a percentage like "65%" or a
// fixed row count like "6"; empty means share evenly.
type Node struct {
	Pane     *Pane
	Split    string // SplitVertical or SplitHorizontal when Children is set
	Stacked  bool   // children share one region; one is expanded and the rest are title rows
	Size     string
	Children []Node
}

// Workspace is the backend-neutral description of a qrouton session's layout.
// Tiled panes only: a floating pane's geometry is resolved against the viewport
// as the backend creates it, and the layout is applied to a session with no
// client attached yet, so anything floated from here comes up sized for the
// backend's own default. Float panes at runtime through a PaneHost instead.
type Workspace struct {
	Slug       string // session name
	Dir        string // session root; panes start here
	HelpScript string // path to the global quick-reference panel, for Run-block keybindings
	Binary     string // qrouton's own executable, for Run-block keybindings that call a subcommand
	Tiled      Node
}

// Handle identifies a backend session across the exec boundary; the launcher
// marshals it into the MCP child's arguments.
type Handle struct {
	Kind      string `json:"kind"`
	Session   string `json:"session"`
	SocketDir string `json:"socket_dir,omitempty"`
}

func (h Handle) Marshal() string {
	b, _ := json.Marshal(h)
	return string(b)
}

func ParseHandle(s string) (Handle, error) {
	var h Handle
	if err := json.Unmarshal([]byte(s), &h); err != nil {
		return Handle{}, fmt.Errorf("multiplexer handle: %w", err)
	}
	if h.Kind == "" || h.Session == "" {
		return Handle{}, fmt.Errorf("%w: %q", ErrHandleIncomplete, s)
	}
	return h, nil
}

// PaneHost resolves the handle to a live pane driver for its backend.
func (h Handle) PaneHost() (PaneHost, error) {
	switch h.Kind {
	case KindZellij:
		return zellijHostFromHandle(h)
	default:
		return nil, unsupportedBackend(h.Kind)
	}
}

// WithEnv returns env with key set to value, replacing any existing entry. It
// builds the environment slices Launcher.Attach and Launcher.Start carry into
// the multiplexer.
func WithEnv(env []string, key, value string) []string {
	prefix := key + envKeyValueSep
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

// New returns the Launcher, verifying the backend is usable. Zellij is the only
// backend: these ports exist to quarantine its KDL/socket mechanics and to give
// the MCP pane tools something fakeable in tests, not to support a second one.
func New() (Launcher, error) {
	return newZellijLauncher()
}
