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

// unsupportedBackend reports a multiplexer qrouton has no adapter for, naming
// the ones it does.
func unsupportedBackend(kind string) error {
	return fmt.Errorf("unsupported multiplexer %q (supported: %s)", kind, KindZellij)
}
