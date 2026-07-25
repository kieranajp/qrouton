package launch

// Literals the launch path depends on: runner identifiers and their arguments,
// the environment variables and shell fragments qrouton injects, and the pane
// names the generated workspace uses.

const (
	// EditorEnvVar carries the resolved editor into the MCP child, which the
	// runner spawns beyond qrouton's own argument list.
	EditorEnvVar = "QROUTON_EDITOR_JSON"

	// openCodeConfigEnvVar is how OpenCode accepts inline configuration.
	openCodeConfigEnvVar = "OPENCODE_CONFIG_CONTENT"

	// shellBin runs the generated pane scripts and hook commands. Panes use a
	// login shell so the user's PATH and aliases apply.
	shellBin       = "sh"
	shellLoginFlag = "-lc"

	shellQuoteChar   = "'"
	shellQuoteEscape = `'\''`

	// Pane names in the generated workspace layout.
	agentPaneName  = "agent"
	shellPaneName  = "shell"
	reposPaneName  = "repos"
	agentsPaneName = "agents"
	helpPaneName   = "qrouton · quick start"

	// Subcommands qrouton launches against itself to drive its own panes.
	mcpSubcommand        = "mcp"
	reposSubcommand      = "repos"
	agentsSubcommand     = "agents"
	agentEventSubcommand = "agent-event"

	sessionRootFlag = "--session-root"
	runnerFlag      = "--runner"
	editorJSONFlag  = "--editor-json"
	muxJSONFlag     = "--mux-json"

	// Placeholders the embedded help script substitutes at stamp time.
	warningPlaceholder = "@@WARNING@@"
	taglinePlaceholder = "@@TAGLINE@@"

	rpiTagline       = "Coordinate here; delegate work to subagents."
	assistantTagline = "Open-ended session; ask to switch to RPI anytime."
)
