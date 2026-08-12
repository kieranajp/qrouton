package launch

// Literals the launch path depends on: runner identifiers and their arguments,
// the environment variables and shell fragments qrouton injects, and the
// subcommands qrouton launches against itself.

const (
	// EditorEnvVar carries the resolved editor into the MCP child, which the
	// runner spawns beyond qrouton's own argument list.
	EditorEnvVar = "QROUTON_EDITOR_JSON"

	// openCodeConfigEnvVar is how OpenCode accepts inline configuration.
	openCodeConfigEnvVar = "OPENCODE_CONFIG_CONTENT"

	// shellBin runs the generated support scripts.
	shellBin = "sh"

	shellQuoteChar   = "'"
	shellQuoteEscape = `'\''`

	// Subcommands qrouton launches against itself.
	mcpSubcommand        = "mcp"
	agentEventSubcommand = "agent-event"
	agentSubcommand      = "agent"
	shellSubcommand      = "shell"

	sessionRootFlag   = "--session-root"
	runnerFlag        = "--runner"
	editorJSONFlag    = "--editor-json"
	workbenchJSONFlag = "--workbench-json"
	resumeFlag        = "--resume"
	socketFlag        = "--socket"

	// workbenchSpecFlag is the hidden marker that makes qrouton run the event
	// loop rather than assemble a session. Its literal is duplicated in main,
	// which defines the flag it names.
	workbenchSpecFlag = "--workbench-spec"
)

// The detached workbench's own plumbing: how a failure to start names the log
// that explains it, and the socket's network.
const (
	workbenchFailureFormat = "%w: see %s"
	specParseError         = "parse workbench spec"
)

// scriptMode is the permission bit the generated support scripts need.
const scriptMode = 0o755

const (
	shellEnvVar    = "SHELL"
	defaultShell   = "/bin/sh"
	loginShellFlag = "-l"

	treeCommand    = "tree"
	treeDepthFlag  = "-L"
	treeDepth      = "2"
	treeColourFlag = "-C"

	openCommand    = "open"
	openRevealFlag = "-R"

	findCommand   = "find"
	findRoot      = "."
	findDepthFlag = "-maxdepth"
	findDepth     = "2"
	findPrintFlag = "-print"
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

	// Claude hook events qrouton subscribes to: the subagent pair feeds the
	// event log, and Notification chimes only when the agent asks for
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

// How a session file's window is named.
const (
	editorWindowLabel   = "Editor"
	documentLabelFormat = "◆ %s"
	frontMatterFence    = "---"
)

var (
	// editorEnvVars are consulted in order when no editor is configured.
	editorEnvVars = []string{"VISUAL", "EDITOR"}

	// fallbackEditors are tried last, as terminal editors that accept +line.
	fallbackEditors = []string{"nvim", "vim", "vi"}
)
