package desktop

import (
	"errors"
	"fmt"
)

var (
	ErrNoAgentCommand     = errors.New("workbench has no agent command to run")
	ErrNoControlSocket    = errors.New("workbench has no control socket address")
	ErrTerminalNotStarted = errors.New("terminal is not started")

	ErrNoWindowOptions  = errors.New("open request carries no window options")
	ErrNoWindowCommand  = errors.New("a terminal window needs a command")
	ErrNotATerminal     = errors.New("window is not a terminal")
	ErrNoSessionRoot    = errors.New("adopt request carries no session root")
	ErrNoSession        = errors.New("this control socket serves no session")
	ErrNoLandingSession = errors.New("no session is being chosen for this workbench to adopt")
	ErrNoShellCommand   = errors.New("workbench has no shell command to run")
	ErrNoDocumentName   = errors.New("open request names no document")
	ErrNoEditorCommand  = errors.New("workbench has no editor command to open a document with")
	ErrNoPickerCommand  = errors.New("workbench has no repository picker to run")
	ErrNoRevealCommand  = errors.New("workbench has no command to reveal a session's directory with")
	ErrNoOnboardCommand = errors.New("workbench has no session assembly to run")
)

func noSuchWindow(id string) error {
	return fmt.Errorf("no open window with id %q", id)
}

func noSuchTerminal(id string) error {
	return fmt.Errorf("no conversation terminal with id %q", id)
}

func unknownSession(slug string) error {
	return fmt.Errorf("no session named %q under the sessions root", slug)
}

// agentAlreadyRunning is a supervisor that outlived its workbench, which the one
// workbench at a time means cannot happen — so it is reported rather than shown.
func agentAlreadyRunning(slug string, pid int) error {
	return fmt.Errorf("session %q already has an agent running as pid %d", slug, pid)
}

// mismatchedManifest is a session directory holding another session's manifest.
// A removal resolves its target from the manifest, so it would take that other
// session's worktrees rather than this directory's.
func mismatchedManifest(dir, slug string) error {
	return fmt.Errorf("session directory %q holds the manifest of %q, so nothing was removed", dir, slug)
}

func unknownOperation(op string) error {
	return fmt.Errorf("unknown workbench operation %q", op)
}
