package mux

// The PaneHost driver: `zellij --session <s> action …`. Used by the MCP server
// from inside a live session to open, read, and close panes.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// commandContext is swapped by tests to intercept pane-driver invocations.
var commandContext = exec.CommandContext

// zellijHost implements PaneHost via `zellij --session <s> action …`.
//
// ownerPaneID is the pane this driver was constructed inside — the agent pane,
// since the MCP server runs as a child of the runner. Spawn focuses it by id to
// hand the keyboard back, which is why a driver built outside a pane (no
// ZELLIJ_PANE_ID) falls back to toggling the floating layer instead.
type zellijHost struct {
	bin, session string
	ownerPaneID  string
}

// NewZellijHost wires a pane driver to a session; the seam for tests.
func NewZellijHost(bin, session string) PaneHost {
	return &zellijHost{
		bin:         bin,
		session:     session,
		ownerPaneID: terminalPaneID(os.Getenv(zellijPaneIDEnvVar)),
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
		z.returnFocus(ctx)
	}
	return id, nil
}

// returnFocus hands the keyboard back to the pane this driver lives in, leaving
// the pinned floating pane rendered on top for reference.
//
// Focusing the owner by id rather than toggling the floating layer off: the
// toggle is a flip, so it depends on the layer's current state, and a user who
// had already hidden the layer with Alt-f got it *shown* by the agent opening a
// pane. Naming the destination has no such coupling. The toggle remains the
// fallback for a driver with no owning pane to name.
func (z *zellijHost) returnFocus(ctx context.Context) {
	if z.ownerPaneID == "" {
		_, _ = z.action(ctx, toggleFloatingAction)
		return
	}
	if _, err := z.action(ctx, focusPaneIDAction, z.ownerPaneID); err != nil {
		_, _ = z.action(ctx, toggleFloatingAction)
	}
}

func (z *zellijHost) Reposition(ctx context.Context, id string, geom Geometry) error {
	// pinned and borderless are deliberately not restated: the pane was created
	// with the ones it wants, and this call is about geometry alone.
	_, err := z.action(ctx, repositionAction, paneIDFlag, id,
		xFlag, geom.X, yFlag, geom.Y, widthFlag, geom.Width, heightFlag, geom.Height)
	return err
}

func (z *zellijHost) Exists(ctx context.Context, id string) (bool, error) {
	panes, err := z.panes(ctx)
	if err != nil {
		return false, err
	}
	for _, pane := range panes {
		if pane.paneID() == id && !pane.Exited {
			return true, nil
		}
	}
	return false, nil
}

// zellijPane is one entry of `list-panes --all --json`, the adapter's only view
// of what the session currently holds. Both the pane registry's liveness check
// and the shell stack's numbering read it.
type zellijPane struct {
	ID         int    `json:"id"`
	IsPlugin   bool   `json:"is_plugin"`
	IsFloating bool   `json:"is_floating"`
	Title      string `json:"title"`
	Exited     bool   `json:"exited"`
}

// paneID renders the entry's id the way every pane-addressing action spells it.
// The prefix is the discriminator: pane 3 as a terminal and pane 3 as a plugin
// are different panes.
func (p zellijPane) paneID() string {
	if p.IsPlugin {
		return pluginPanePrefix + strconv.Itoa(p.ID)
	}
	return terminalPaneID(strconv.Itoa(p.ID))
}

func (z *zellijHost) panes(ctx context.Context) ([]zellijPane, error) {
	out, err := z.action(ctx, listPanesAction, allFlag, jsonFlag)
	if err != nil {
		return nil, err
	}
	var panes []zellijPane
	if err := json.Unmarshal(out, &panes); err != nil {
		return nil, fmt.Errorf("parse zellij panes: %w", err)
	}
	return panes, nil
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
