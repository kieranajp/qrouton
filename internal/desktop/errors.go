package desktop

import (
	"errors"

	"github.com/kieranajp/qrouton/internal/core"
)

var (
	ErrNoAgentCommand   = core.ErrNoAgentCommand
	ErrNoControlSocket  = errors.New("workbench has no control socket address")
	ErrNoConfig         = core.ErrNoConfig
	ErrStaleFrontend    = errors.New(staleFrontendError)
	ErrNoOwners         = errors.New(noOwnersError)
	ErrUIScale          = errors.New(uiScaleRefusal)
	ErrUIScaleAction    = errors.New(unknownUIScaleAction)
	ErrStickerDuplicate = errors.New(stickerDuplicate)

	ErrNoRevealCommand    = core.Unavailable("workbench has no command to reveal a session's directory with")
	ErrNoOpenTextCommand  = core.Unavailable("workbench has no command to open a file in a text editor")
	ErrOpenConfig         = errors.New("config.json could not be opened")
	ErrNoCopyImageCommand = core.Unavailable("workbench has no command to copy an image to the clipboard with")
	ErrNoCopyFileCommand  = core.Unavailable("workbench has no command to copy a file to the clipboard with")
	ErrNoRelaunch         = errors.New("workbench has no way to relaunch itself into a new sessions root")
	ErrNoDirectoryPicker  = core.Unavailable("workbench has no directory picker to choose a sessions root with")
)
