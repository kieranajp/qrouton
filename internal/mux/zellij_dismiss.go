package mux

// Dismissible-pane focus tracking. Zellij keybindings cannot express pane
// predicates, so this watcher reserves normal mode for transient panes while
// they hold focus — that is what makes the staged Esc binding safe.

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	dismissFocusPollInterval = 200 * time.Millisecond
	dismissFocusPollTimeout  = 2 * time.Second
)

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
