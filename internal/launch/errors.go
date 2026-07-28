package launch

import "errors"

// Sentinel errors the launch path returns. Callers in main and the TUI surface
// these to the user verbatim, so their wording lives here rather than being
// retyped at each site.
var (
	// ErrNoRunnerInstalled means none of the supported coding agents is on PATH.
	ErrNoRunnerInstalled = errors.New("no supported coding agent is installed")

	// ErrUnsupportedOverride means the config's launch map is keyed by a runner
	// qrouton has no wiring for.
	ErrUnsupportedOverride = errors.New("launch override is not a supported runner")

	// ErrEmptyOverride means a launch override supplied no command, which would
	// otherwise report the runner as not installed rather than misconfigured.
	ErrEmptyOverride = errors.New("launch override has no command")

	// ErrRunnerUnavailable means the requested runner is not installed, or is
	// not one qrouton supports.
	ErrRunnerUnavailable = errors.New("runner is unavailable")

	// ErrUnsupportedRunner means a Runner reached the launch path without the
	// per-runner MCP and hook wiring runnerLaunch needs.
	ErrUnsupportedRunner = errors.New("unsupported runner")

	// ErrEditorPlaceholder means a configured editor command cannot be used
	// because qrouton cannot tell where to substitute the file path.
	ErrEditorPlaceholder = errors.New("editor must contain exactly one " + pathPlaceholder + " placeholder")

	// ErrNoEditor means qrouton found no terminal editor to open files with.
	ErrNoEditor = errors.New("no terminal editor found")

	// Errors resolving a path an agent asked qrouton to open. All of them mean
	// the path is not a usable thing inside this session.
	ErrNotRegularFile        = errors.New("not a regular file")
	ErrNotDirectory          = errors.New("not a directory")
	ErrOutsideSession        = errors.New("path is outside the qrouton session")
	ErrOutsideSessionMissing = errors.New("path does not exist in the qrouton session")
)
