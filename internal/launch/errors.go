package launch

import "errors"

var (
	ErrNoRunnerInstalled = errors.New("no supported coding agent is installed")

	ErrUnsupportedOverride = errors.New("launch override is not a supported runner")

	// ErrEmptyOverride means a launch override supplied no command, which would
	// otherwise report the runner as not installed rather than misconfigured.
	ErrEmptyOverride = errors.New("launch override has no command")

	ErrRunnerUnavailable = errors.New("runner is unavailable")

	// ErrUnsupportedRunner means a Runner reached the launch path with an id no
	// spec claims, so there is no MCP and hook wiring to launch it with.
	ErrUnsupportedRunner = errors.New("unsupported runner")

	ErrEditorPlaceholder = errors.New("editor must contain exactly one " + pathPlaceholder + " placeholder")

	ErrNoEditor = errors.New("no terminal editor found")

	// ErrInvalidEditor means the marshalled editor configuration a child process
	// was handed could not be read back. An absent editor is legitimate; a
	// malformed one is a configuration mistake worth stopping for.
	ErrInvalidEditor = errors.New("editor configuration is malformed")

	ErrWorkbenchSpecIncomplete = errors.New("workbench spec missing socket or command")

	// ErrWorkbenchExited means the detached workbench died before it served its
	// control socket, so there is no window and the reason is in its log.
	ErrWorkbenchExited = errors.New("workbench exited before it opened")

	ErrWorkbenchNotReady = errors.New("workbench did not open in time")

	// Errors resolving a path an agent asked qrouton to open. All of them mean
	// the path is not a usable thing inside this session.
	ErrNotRegularFile        = errors.New("not a regular file")
	ErrNotDirectory          = errors.New("not a directory")
	ErrOutsideSession        = errors.New("path is outside the qrouton session")
	ErrOutsideSessionMissing = errors.New("path does not exist in the qrouton session")
)
