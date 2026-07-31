package mux

import (
	"errors"
	"fmt"
)

// ErrZellijRequired is returned when the Zellij binary is missing or too old
// for the layout and pane actions qrouton relies on.
var ErrZellijRequired = fmt.Errorf("zellij %d.%d or newer is required", minZellijMajor, minZellijMinor)

// ErrHandleIncomplete means a marshalled Handle crossed the exec boundary
// without the identity a pane driver needs.
var ErrHandleIncomplete = errors.New("multiplexer handle missing kind or session")

// ErrShellContext means the internal shell command was run outside the Zellij
// pane environment that identifies the current pane and session.
var ErrShellContext = errors.New("shell command is not running inside a Zellij pane")

// ErrPaneNotFound means a runtime pane lookup by exact title found no live
// terminal pane. Layout-owned panes such as the dock receive their ids only
// after Zellij starts the session, so title lookup is the stable bridge.
var ErrPaneNotFound = errors.New("multiplexer pane not found")

// unsupportedBackend reports a multiplexer qrouton has no adapter for, naming
// the ones it does.
func unsupportedBackend(kind string) error {
	return fmt.Errorf("unsupported multiplexer %q (supported: %s)", kind, KindZellij)
}
