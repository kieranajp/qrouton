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

	// shellBin runs the generated quick-reference script.
	shellBin = "sh"

	shellQuoteChar   = "'"
	shellQuoteEscape = `'\''`

	// Pane names in the generated workspace layout. A pane's title carries a
	// chord only where a real binding operates that pane; movement and the
	// rest of the key list are stated once, in the strip and in Alt-?.
	agentPaneName        = "agent"
	shellPaneName        = "shell"
	shellPaneTitleSuffix = " · Alt-g new · Alt-↑↓ switch · Ctrl-d close"
	reposPaneName        = "repos"
	agentsPaneName       = "agents"
	statusPaneName       = "status"
	helpPaneName         = "keys · press any key to close"

	// Subcommands qrouton launches against itself to drive its own panes.
	mcpSubcommand        = "mcp"
	reposSubcommand      = "repos"
	agentsSubcommand     = "agents"
	statusSubcommand     = "status"
	shellSubcommand      = "shell"
	agentEventSubcommand = "agent-event"
	agentSubcommand      = "agent"

	sessionRootFlag = "--session-root"
	runnerFlag      = "--runner"
	editorJSONFlag  = "--editor-json"
	muxJSONFlag     = "--mux-json"
	resumeFlag      = "--resume"

	// codexDepthWarning is passed as help.sh's $1 on the startup invocation
	// only, when Codex's subagent nesting is too shallow; the Alt-? route
	// omits it (a launch-time-only concern). The one-shot splash's mode
	// tagline moved into the script itself, which reads ./qrouton.json at
	// runtime — mode-correct per session, including immediately after an
	// escalation, from a single global copy.
	codexDepthWarning = "Codex agents.max_depth is under 2. Set it to 3 " +
		"in ~/.codex/config.toml for nested subagents."
)

// Sizes and geometry of the generated workspace. Percentages are the
// multiplexer's own units; watchPaneRows is a fixed row count, since the repo
// and subagent readouts need a known few lines rather than a share of the
// column.
const (
	agentColumnSize = "65%"
	reposColumnSize = "35%"
	watchPaneRows   = "6"
	stripPaneRows   = "1"
)

// scriptMode is the permission bit the generated pane scripts need.
const scriptMode = 0o755

const (
	shellEnvVar    = "SHELL"
	defaultShell   = "/bin/sh"
	loginShellFlag = "-l"

	treeCommand    = "tree"
	treeDepthFlag  = "-L"
	treeDepth      = "2"
	treeColourFlag = "-C"

	findCommand   = "find"
	findRoot      = "."
	findDepthFlag = "-maxdepth"
	findDepth     = "2"
	findPrintFlag = "-print"

	lastShellMessage = "qrouton: the shell area always keeps one shell; starting a fresh one"
	joinShellError   = "join shell stack"
	countShellError  = "count shell panes"
)

// Runner identifiers, labels, and the arguments qrouton launches each with.
// The permission-bypass flags are deliberate: a qrouton session is an
// already-isolated worktree the user opened for the agent to work in.
const (
	runnerIDClaude   = "claude"
	runnerIDCodex    = "codex"
	runnerIDOpenCode = "opencode"

	runnerLabelClaude   = "Claude Code"
	runnerLabelCodex    = "Codex CLI"
	runnerLabelOpenCode = "OpenCode"

	claudeSkipPermissionsFlag = "--dangerously-skip-permissions"
	codexBypassSandboxFlag    = "--dangerously-bypass-approvals-and-sandbox"
	openCodeAutoFlag          = "--auto"

	// Resume arguments: each runner spells "continue the last conversation"
	// differently.
	claudeContinueFlag = "--continue"
	codexResumeCmd     = "resume"
	codexResumeLast    = "--last"

	claudeMCPConfigFlag = "--mcp-config"
	claudeSettingsFlag  = "--settings"
	codexConfigFlag     = "-c"
)

// Keys in the runner configuration qrouton injects. Each runner accepts MCP
// servers and hooks in its own shape.
const (
	serverName = "qrouton"

	claudeMCPServersKey = "mcpServers"
	claudeStdioType     = "stdio"
	claudeCommandKey    = "command"
	claudeArgsKey       = "args"
	claudeTypeKey       = "type"
	claudeHooksKey      = "hooks"
	claudeCommandType   = "command"

	// Claude hook events qrouton subscribes to: the subagent pair drives the
	// agents pane, and Notification chimes only when the agent asks for
	// attention, so the user can step away.
	claudeSubagentStartHook = "SubagentStart"
	claudeSubagentStopHook  = "SubagentStop"
	claudeNotificationHook  = "Notification"

	codexMCPCommandKey = "mcp_servers.qrouton.command="
	codexMCPArgsKey    = "mcp_servers.qrouton.args="

	openCodeMCPKey        = "mcp"
	openCodeLocalType     = "local"
	openCodeEnabledKey    = "enabled"
	openCodePermissionKey = "permission"
	openCodeAllowValue    = "allow"
)

// Opening messages: the first prompt a fresh session sends its runner. RPI
// presents the orchestrated workflow; Assistant stays open-ended while pointing
// at the workflow the user can escalate into.
const (
	openingMessageRPI = "You have just been launched in a qrouton session. " +
		"Read the session instructions and manifest, inspect relevant thoughts/shared artifacts, " +
		"then respond naturally. Present the work as Research, Plan, or Implement; keep your own " +
		"context lean by delegating execution wherever practical."

	openingMessageAssistant = "You have just been launched in a qrouton session. " +
		"Read the session instructions and manifest, skim relevant thoughts/shared artifacts, " +
		"then help with whatever the user asks — work directly and keep your own context lean. " +
		"A structured Research → Plan → Implement workflow is available if the user wants it."
)

// Editor resolution. A configured editor command is a template: qrouton
// substitutes the file path, and the line number where the editor accepts one.
const (
	pathPlaceholder    = "{path}"
	linePlaceholder    = "{line}"
	linePlaceholderArg = "+" + linePlaceholder

	firstLine = 1
)

var (
	// editorEnvVars are consulted in order when no editor is configured.
	editorEnvVars = []string{"VISUAL", "EDITOR"}

	// fallbackEditors are tried last, as terminal editors that accept +line.
	fallbackEditors = []string{"nvim", "vim", "vi"}
)
