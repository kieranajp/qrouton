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
	ErrNoSessionRoot   = errors.New("adopt request carries no session root")
	ErrNoShellCommand  = errors.New("workbench has no shell command to run")
)

func noSuchWindow(id string) error {
	return fmt.Errorf("no open window with id %q", id)
}

func unknownOperation(op string) error {
	return fmt.Errorf("unknown workbench operation %q", op)
}
