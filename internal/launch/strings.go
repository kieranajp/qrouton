package launch

import (
	"time"

	"github.com/kieranajp/qrouton/internal/codex"
)

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
	generationFlag    = "--generation"
	providerFlag      = "--provider"
	resumeFlag        = "--resume"

	// workbenchSpecFlag is the hidden marker that makes qrouton run the event
	// loop rather than assemble a session.
	workbenchSpecFlag = "--workbench-spec"
)

const generationSignalTimeout = 2 * time.Second

const (
	workbenchFailureFormat = "%w: see %s"
	specParseError         = "parse workbench spec"
)

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
	runnerIDClaude = "claude"
	// runnerIDCodex is codex.Binary: the identifier and the command are the same
	// word, and codex owns the spelling.
	runnerIDCodex    = codex.Binary
	runnerIDOpenCode = "opencode"

	runnerLabelClaude   = "Claude Code"
	runnerLabelCodex    = "Codex CLI"
	runnerLabelOpenCode = "OpenCode"

	claudeSkipPermissionsFlag = "--dangerously-skip-permissions"
	codexBypassSandboxFlag    = "--dangerously-bypass-approvals-and-sandbox"
	codexBypassHookTrustFlag  = "--dangerously-bypass-hook-trust"
	openCodeAutoFlag          = "--auto"
	openCodePromptFlag        = "--prompt"

	claudeContinueFlag = "--continue"
	codexResumeCmd     = "resume"
	codexResumeLast    = "--last"

	claudeMCPConfigFlag = "--mcp-config"
	claudeSettingsFlag  = "--settings"

	claudeMaintainProjectWorkingDirEnvVar = "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR"
	claudeMaintainProjectWorkingDirValue  = "1"
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

	claudeSubagentStartHook = "SubagentStart"
	claudeSubagentStopHook  = "SubagentStop"
	claudeNotificationHook  = "Notification"

	codexMCPCommandKey           = "mcp_servers.qrouton.command="
	codexMCPArgsKey              = "mcp_servers.qrouton.args="
	codexSubagentStartHook       = "hooks.SubagentStart="
	codexSubagentStopHook        = "hooks.SubagentStop="
	codexCommandHookFormat       = `[{hooks=[{type="command",command=%s,timeout=3}]}]`
	codexAgentEventCommandFormat = `"$%s" %s`

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
	linearRequestSeparator = "\n\nLinear request:\n"

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

const (
	editorWindowLabel   = "Editor"
	documentLabelFormat = "◆ %s"
)

var (
	// editorEnvVars are consulted in order when no editor is configured.
	editorEnvVars = []string{"VISUAL", "EDITOR"}

	// fallbackEditors are tried last, as terminal editors that accept +line.
	fallbackEditors = []string{"nvim", "vim", "vi"}
)
