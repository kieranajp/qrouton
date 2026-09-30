package launch

import (
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
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

	flagPrefix = "--"

	sessionRootFlag   = flagPrefix + SessionRootFlag
	runnerFlag        = flagPrefix + RunnerFlag
	editorJSONFlag    = flagPrefix + EditorJSONFlag
	workbenchJSONFlag = flagPrefix + WorkbenchJSONFlag
	generationFlag    = flagPrefix + GenerationFlag
	providerFlag      = flagPrefix + ProviderFlag
	resumeFlag        = flagPrefix + ResumeFlag
	workbenchSpecFlag = flagPrefix + WorkbenchSpecFlag
)

// The command line qrouton launches itself with. The commands that parse it
// take their names from here.
const (
	MCPSubcommand        = "mcp"
	AgentEventSubcommand = "agent-event"
	AgentSubcommand      = "agent"
	ShellSubcommand      = "shell"

	SessionRootFlag   = "session-root"
	RunnerFlag        = "runner"
	EditorJSONFlag    = "editor-json"
	WorkbenchJSONFlag = "workbench-json"
	GenerationFlag    = "generation"
	ProviderFlag      = "provider"
	ResumeFlag        = "resume"

	// WorkbenchSpecFlag is the hidden marker that makes qrouton run the event
	// loop rather than assemble a session.
	WorkbenchSpecFlag = "workbench-spec"
)

const generationSignalTimeout = 2 * time.Second

const (
	workbenchFailureFormat = "%w: see %s"
	specParseError         = "parse workbench spec"
)

const (
	scriptMode = 0o755
	// configMode is the generated runner configuration qrouton writes into a
	// session.
	configMode = 0o644
)

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

	shellCommandFlag = "-c"
	// sh -c takes the script's own name before its arguments; $1 is the path.
	shellArgv0 = "sh"

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
	runnerIDAgy      = "agy"

	runnerLabelClaude   = "Claude Code"
	runnerLabelCodex    = "Codex CLI"
	runnerLabelOpenCode = "OpenCode"
	runnerLabelAgy      = "Antigravity CLI"

	claudeSkipPermissionsFlag = "--dangerously-skip-permissions"
	codexBypassSandboxFlag    = "--dangerously-bypass-approvals-and-sandbox"
	codexBypassHookTrustFlag  = "--dangerously-bypass-hook-trust"
	openCodeAutoFlag          = "--auto"
	openCodePromptFlag        = "--prompt"
	agySkipPermissionsFlag    = "--dangerously-skip-permissions"

	claudeContinueFlag = "--continue"
	claudeNameFlag     = "--name"
	codexResumeCmd     = "resume"
	codexResumeLast    = "--last"
	agyContinueFlag    = "--continue"

	// agy's own --prompt is an alias for --print: one turn, then exit.
	agyPromptInteractiveFlag = "--prompt-interactive"
	// agyGeminiDirFlag relocates agy's whole configuration root. It is absent
	// from --help, and it is the only route to a session-scoped MCP server.
	agyGeminiDirFlag = "--gemini_dir"

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

	claudeSubagentStartHook = agentevent.HookSubagentStart
	claudeSubagentStopHook  = agentevent.HookSubagentStop
	claudeNotificationHook  = agentevent.HookNotification
	claudeStopHook          = agentevent.HookStop

	codexMCPCommandKey           = "mcp_servers." + serverName + ".command="
	codexMCPArgsKey              = "mcp_servers." + serverName + ".args="
	codexSubagentStartHook       = "hooks." + agentevent.HookSubagentStart + "="
	codexSubagentStopHook        = "hooks." + agentevent.HookSubagentStop + "="
	codexCommandHookFormat       = `[{hooks=[{type="command",command=%s,timeout=%d}]}]`
	codexHookTimeoutSeconds      = int(agentevent.SignalTimeout/time.Second) + 1
	codexAgentEventCommandFormat = `"$%s" %s`

	openCodeMCPKey        = "mcp"
	openCodeLocalType     = "local"
	openCodeEnabledKey    = "enabled"
	openCodePermissionKey = "permission"
	openCodeAllowValue    = "allow"

	// agy takes MCP servers from <gemini root>/config/mcp_config.json and
	// nowhere else: not on argv, not from the environment.
	geminiDirName    = ".gemini"
	agyConfigDirName = "config"
	agyMCPConfigName = "mcp_config.json"
)

// Opening messages: the first prompt a fresh session sends its runner. RPI
// presents the orchestrated workflow; Assistant stays open-ended while pointing
// at the workflow the user can escalate into.
const (
	requestSeparator         = "\n\nRequest:\n"
	providerRequestSeparator = "\n\n%s request:\n"

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
	editorWindowLabel       = "Editor"
	documentLabelFormat     = "◆ %s"
	imagePathsRequiredError = "image paths are required"
	imagePathRequiredError  = "image path is required"
	unsupportedImageError   = "unsupported image format"
	imageEntryErrorFormat   = "image %d %q: %w"
)

var (
	// editorEnvVars are consulted in order when no editor is configured.
	editorEnvVars = []string{"VISUAL", "EDITOR"}

	// fallbackEditors are tried last, as terminal editors that accept +line.
	fallbackEditors = []string{"nvim", "vim", "vi"}
)
