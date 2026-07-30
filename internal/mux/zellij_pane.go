package mux

// The PaneHost driver: `zellij --session <s> action …`. Used by the MCP server
// from inside a live session to open, read, and close panes.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// commandContext is swapped by tests to intercept pane-driver invocations.
var commandContext = exec.CommandContext

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
