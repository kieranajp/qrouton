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
}

// Geometry places a floating pane, in the backend's units (percent strings).
type Geometry struct {
	X, Y, Width, Height string
}

// SpawnOptions describes a runtime pane opened through a PaneHost.
type SpawnOptions struct {
	Label       string
	Cwd         string
	Geometry    Geometry
	CloseOnExit bool
	Command     []string
}

// Pane is one terminal in a workspace layout, running Command.
type Pane struct {
	Name        string
	Command     []string
	CloseOnExit bool
	Focus       bool
}

// Node is an element of the tiled layout tree: a leaf Pane, or a split with
// Children. Size is a backend-rendered hint — a percentage like "65%" or a
// fixed row count like "6"; empty means share evenly.
type Node struct {
	Pane     *Pane
	Split    string // "vertical" or "horizontal" when Children is set
	Size     string
	Children []Node
}

// Floating is a pane layered over the tiled layout.
type Floating struct {
	Pane
	Geometry Geometry
}

// Workspace is the backend-neutral description of a qrouton session's layout.
type Workspace struct {
	Slug     string // session name
	Dir      string // session root; panes start here
	Tiled    Node
	Floating []Floating
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
		return Handle{}, fmt.Errorf("multiplexer handle missing kind or session: %q", s)
	}
	return h, nil
}

// PaneHost resolves the handle to a live pane driver for its backend.
func (h Handle) PaneHost() (PaneHost, error) {
	switch h.Kind {
	case "zellij":
		return zellijHostFromHandle(h)
	default:
		return nil, fmt.Errorf("unsupported multiplexer %q", h.Kind)
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

// New returns the configured Launcher, verifying the backend is usable. An
// empty kind selects the default (Zellij).
func New(kind string) (Launcher, error) {
	switch strings.TrimSpace(kind) {
	case "", "zellij":
		return newZellijLauncher()
	default:
		return nil, fmt.Errorf("unsupported multiplexer %q in config (supported: zellij)", kind)
	}
}
