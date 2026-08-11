package desktop

import (
	"errors"
	"fmt"
)

var (
	ErrNoAgentCommand     = errors.New("workbench has no agent command to run")
	ErrNoControlSocket    = errors.New("workbench has no control socket address")
	ErrTerminalNotStarted = errors.New("terminal is not started")

	ErrNoWindowOptions = errors.New("open request carries no window options")
	ErrNoWindowCommand = errors.New("a terminal window needs a command")
	ErrNotATerminal    = errors.New("window is not a terminal")
	ErrNoSessionRoot   = errors.New("adopt request carries no session root")
	ErrNoShellCommand  = errors.New("workbench has no shell command to run")
	ErrNoDocumentName  = errors.New("open request names no document")
	ErrNoEditorCommand = errors.New("workbench has no editor command to open a document with")
)

func noSuchWindow(id string) error {
	return fmt.Errorf("no open window with id %q", id)
}

func unknownOperation(op string) error {
	return fmt.Errorf("unknown workbench operation %q", op)
}
