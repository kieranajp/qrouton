package mcpserver

import (
	"errors"
	"fmt"
)

// Input errors, returned to the agent so it can correct its own call.
var (
	ErrCommandRequired = errors.New("command is required")
	ErrMessageRequired = errors.New("message is required")
	ErrNameRequired    = errors.New("name is required")

	// ErrReservedPaneName means the agent tried to claim the editor pane's
	// registry key for a command pane.
	ErrReservedPaneName = fmt.Errorf("%q is reserved for the editor pane; pick another name", editorPaneName)
)

// ErrInvalidEditor means the editor configuration the launcher marshalled into
// this process is missing or unusable.
var ErrInvalidEditor = errors.New("mcp: invalid inherited editor configuration")

func noSuchPane(name string) error {
	return fmt.Errorf("no open pane named %q", name)
}

// paneClosedByUser distinguishes a pane the user dismissed from one that never
// existed, so the agent reopens it rather than treating the name as wrong.
func paneClosedByUser(name string) error {
	return fmt.Errorf("pane %q was closed by the user; open it again if you still need it", name)
}
