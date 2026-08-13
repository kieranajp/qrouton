package desktop

import (
	"errors"
	"fmt"

	"github.com/kieranajp/qrouton/internal/assembly"
)

var (
	ErrNoAgentCommand     = errors.New("workbench has no agent command to run")
	ErrNoControlSocket    = errors.New("workbench has no control socket address")
	ErrNoConfig           = errors.New("workbench has no configuration to assemble sessions against")
	ErrTerminalNotStarted = errors.New("terminal is not started")
	ErrStaleFrontend      = errors.New(staleFrontendError)

	ErrNoWindowOptions = errors.New("open request carries no window options")
	ErrNoWindowCommand = errors.New("a terminal window needs a command")
	ErrNotATerminal    = errors.New("window is not a terminal")
	ErrNoSessionRoot   = errors.New("picker request carries no session root")
	ErrNoSession       = errors.New("this control socket serves no session")
	ErrNoShellCommand  = errors.New("workbench has no shell command to run")
	ErrNoDocumentName  = errors.New("open request names no document")
	ErrNoEditorCommand = errors.New("workbench has no editor command to open a document with")
	ErrNoRevealCommand = errors.New("workbench has no command to reveal a session's directory with")
	ErrNoViewport      = errors.New("window has no source-mapped viewport")
	ErrInvalidViewport = errors.New("invalid document viewport report")
)

// draftRefused turns a validation problem into the refusal the page's promise
// rejects with, so pressing Create on an invalid form says which field.
func draftRefused(p assembly.Problem) error {
	return fmt.Errorf("%s: %s", p.Field, p.Message)
}

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
