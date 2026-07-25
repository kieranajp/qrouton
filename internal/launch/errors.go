package launch

import "errors"

// Sentinel errors the launch path returns. Callers in main and the TUI surface
// these to the user verbatim, so their wording lives here rather than being
// retyped at each site.
var (
	// ErrNoRunnerInstalled means none of the supported coding agents is on PATH.
	ErrNoRunnerInstalled = errors.New("no supported coding agent is installed")

	// ErrUnsupportedOverride means the config's launch list names a command
	// qrouton has no runner wiring for.
	ErrUnsupportedOverride = errors.New("launch override is not a supported runner")

	// ErrEditorPlaceholder means a configured editor command cannot be used
	// because qrouton cannot tell where to substitute the file path.
	ErrEditorPlaceholder = errors.New("editor must contain exactly one {path} placeholder")
)
