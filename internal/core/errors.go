package core

import (
	"errors"
	"fmt"

	"github.com/kieranajp/qrouton/internal/assembly"
)

var ErrUnavailable = errors.New("this client cannot do that")

// Unavailable is a sentinel with its own text that also matches ErrUnavailable,
// so a caller can test for the one refusal or for any.
func Unavailable(text string) error {
	return &unavailable{text: text}
}

type unavailable struct{ text string }

func (u *unavailable) Error() string { return u.text }

func (u *unavailable) Is(target error) bool { return target == ErrUnavailable }

var ErrTerminalNotStarted = errors.New("terminal is not started")

var ErrNoThoughtsRoot = errors.New("this session's thoughts are in no thoughts root")

var (
	ErrBugReportInvalid       = errors.New("bug report needs an id, title, body, and deadline; an id cannot change content")
	ErrBugReportBusy          = errors.New("this session already has a bug report awaiting review or creation")
	ErrBugReportStale         = errors.New("bug report is no longer available for this session")
	ErrAssetGalleryClosed     = errors.New("asset galleries are unavailable for this session")
	ErrNotAssetGallery        = errors.New("window is not an asset gallery")
	ErrAssetIndex             = errors.New("asset index is out of range")
	ErrNoAssetFocus           = errors.New("asset focus request carries no selection")
	ErrNoWindowOptions        = errors.New("open request carries no window options")
	ErrNoWindowCommand        = errors.New("a terminal window needs a command")
	ErrNotATerminal           = errors.New("window is not a terminal")
	ErrNoSessionRoot          = errors.New("picker request carries no session root")
	ErrNoSession              = errors.New("this control socket serves no session")
	ErrSessionUnloaded        = errors.New("this session has been unloaded")
	ErrNoShellCommand         = errors.New("workbench has no shell command to run")
	ErrNoDocumentName         = errors.New("open request names no document")
	ErrNoEditorCommand        = errors.New("workbench has no editor command to open a document with")
	ErrNotAnAsset             = errors.New("that file is not an asset the gallery shows")
	ErrPathOutsideSession     = errors.New("that path is not inside the session it names")
	ErrNoViewport             = errors.New("window has no source-mapped viewport")
	ErrInvalidViewport        = errors.New("invalid document viewport report")
	ErrInvalidAssetGallery    = errors.New("invalid asset gallery options")
	ErrGalleryAssetToken      = errors.New("could not create gallery asset token")
	ErrNoTicket               = errors.New("open ticket request carries no ticket URL")
	ErrProcessIngressOnly     = errors.New("open ticket is available only on the published process socket")
	ErrNoRunnerGeneration     = errors.New("runner generation request carries no generation")
	ErrNoDelegatedLifecycle   = errors.New("delegated lifecycle request carries no event")
	ErrNoAgentEvent           = errors.New("agent event request carries no event")
	ErrOverviewWindowPinned   = errors.New("the Overview tab stays open and stays first")
	ErrOverviewWindowReserved = errors.New("only the workbench opens the Overview tab")
	ErrNoVault                = errors.New("no thoughts index is available; carry on without it")
	ErrNoVaultRequest         = errors.New("thoughts request carries no query or reference")
	ErrDocumentChanged        = errors.New("the document changed on disk since the pane read it")
	ErrNotAnswerable          = errors.New("window is not a spec or an audit")
	ErrWrongSend              = errors.New("that line is not the one this document sends")
	ErrNoSuchAsset            = errors.New("no open deck or gallery serves that asset")
	ErrNoAgentCommand         = errors.New("workbench has no agent command to run")
	ErrNoConfig               = errors.New("workbench has no configuration to assemble sessions against")
)

func noSuchWindow(id string) error {
	return fmt.Errorf("no open window with id %q", id)
}

func noSuchTerminal(id string) error {
	return fmt.Errorf("no conversation terminal with id %q", id)
}

func UnknownSession(slug string) error {
	return fmt.Errorf("no session named %q under the sessions root", slug)
}

// agentAlreadyRunning is a supervisor that outlived its workbench, which the one
// workbench at a time means cannot happen — so it is reported rather than shown.
func agentAlreadyRunning(slug string, pid int) error {
	return fmt.Errorf("session %q already has an agent running as pid %d", slug, pid)
}

func unknownOperation(op string) error {
	return fmt.Errorf("unknown workbench operation %q", op)
}

// draftRefused turns a validation problem into the refusal the page's promise
// rejects with, so pressing Create on an invalid form says which field.
func draftRefused(p assembly.Problem) error {
	return fmt.Errorf("%s: %s", p.Field, p.Message)
}
