// Package dock draws the permanent anchor that agent-owned panes stack into
// when minimised. Zellij collapses this pane to its title row while a docked
// pane is expanded, then expands it again when the last pane is restored or
// closed.
package dock

import (
	"fmt"
	"time"

	"github.com/kieranajp/qrouton/internal/paneui"
)

// Status redraws the empty dock occasionally so a terminal repaint cannot
// leave stale content behind. Most of its life it is collapsed to one title
// row by Zellij's native pane stack.
func Status() error {
	for {
		fmt.Print(paneui.Frame(statusLines()))
		time.Sleep(refreshInterval)
	}
}

func statusLines() []string {
	return []string{
		paneui.Title(PaneName),
		paneui.Muted(emptyStateLabel),
		paneui.Muted(emptyStateHint),
	}
}
