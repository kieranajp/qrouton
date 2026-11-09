package desktop

import (
	"errors"

	"github.com/kieranajp/qrouton/internal/core"
)

var (
	ErrNoAgentCommand  = core.ErrNoAgentCommand
	ErrNoControlSocket = errors.New("workbench has no control socket address")
	ErrNoConfig        = core.ErrNoConfig
	ErrStaleFrontend   = errors.New(staleFrontendError)

	ErrNoRevealCommand    = core.Unavailable("workbench has no command to reveal a session's directory with")
	ErrNoOpenTextCommand  = core.ErrNoOpenTextCommand
	ErrNoCopyImageCommand = core.Unavailable("workbench has no command to copy an image to the clipboard with")
	ErrNoCopyFileCommand  = core.Unavailable("workbench has no command to copy a file to the clipboard with")
	ErrNoDirectoryPicker  = core.ErrNoDirectoryPicker
)
