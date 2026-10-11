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

	osascriptCommand = "osascript"
	osascriptExecute = "-e"

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

const (
	openCodeDatabaseEnvVar               = "OPENCODE_DB"
	agyCLIStateDirName                   = "antigravity-cli"
	codexLongConfigFlag                  = "--config"
	reviewerModelHelp                    = "Enter the model identifier accepted by this harness."
	openCodeReviewerModelHelp            = "Enter a model identifier in provider/model form."
	reviewerDefaultReasoningHelp         = "Choose a supported effort, or leave blank for the provider default."
	codexReviewerReasoningHelp           = "Enter the effort accepted by your model, or leave blank for the provider default."
	openCodeReviewerReasoningHelp        = "OpenCode reasoning uses the provider default; leave this value blank."
	reviewerHarnessErrorFormat           = "%w: choose a supported harness, got %q (supported: %s)"
	reviewerModelErrorFormat             = "%w: model must be nonblank UTF-8 text without control characters"
	reviewerModelCapabilityErrorFormat   = "%w: model is unsupported for %s. %s"
	reviewerReasoningEncodingErrorFormat = "%w: reasoning must be UTF-8 text without control characters"
	reviewerReasoningErrorFormat         = "%w: reasoning is unsupported for %s. %s"
	reviewerOverrideErrorFormat          = "%w: %s argument %q; move model, role and effort options to reviewer Settings and remove resume, headless or working-directory arguments"
)

var (
	agySharedReviewerFiles  = []string{"settings.json", "keybindings.json", "bin", "builtin"}
	claudeReviewerEfforts   = []string{"low", "medium", "high", "xhigh", "max"}
	agyReviewerEfforts      = []string{"low", "medium", "high"}
	claudeReviewerOverrides = reviewerOverridePolicy{
		Flags:         []string{"--continue", "-c", "--resume", "-r", "--fork-session", "--session-id", "--from-pr", "--teleport", "--print", "-p", "--background", "--bg", "--model", "--fallback-model", "--effort", "--agent", "--agents", "--system-prompt", "--system-prompt-file", "--append-system-prompt", "--append-system-prompt-file", "--worktree", "-w", "--add-dir"},
		Commands:      []string{"attach"},
		PathListFlags: []string{"--mcp-config", "--plugin-dir"},
		PathFlags:     []string{"--settings", "--debug-file", "--add-dir", "--append-system-prompt-file", "--system-prompt-file"},
		ValueFlags:    []string{"--settings", "--name", "-n", "--permission-mode", "--mcp-config", "--add-dir", "--allowedTools", "--allowed-tools", "--disallowedTools", "--disallowed-tools", "--tools", "--debug-file", "--setting-sources"},
	}
	codexReviewerOverrides = reviewerOverridePolicy{
		Flags:      []string{"--model", "-m", "--last", "--cd", "-C"},
		Commands:   []string{"resume", "fork", "exec", "e", "review", "exec-server", "app-server"},
		ValueFlags: []string{"--enable", "--disable", "--remote", "--remote-auth-token-env", "--image", "-i", "--local-provider", "--profile", "-p", "--sandbox", "-s", "--cd", "-C", "--add-dir", "--ask-for-approval", "-a"},
		PathFlags:  []string{"--image", "-i", "--add-dir"},
		ConfigKeys: []string{"model", "model_reasoning_effort", "developer_instructions"},
	}
	openCodeReviewerOverrides = reviewerOverridePolicy{
		Flags:      []string{"--continue", "-c", "--session", "-s", "--fork", "--model", "-m", "--agent", "--prompt", "--variant"},
		Commands:   []string{"run", "serve", "web", "acp", "attach"},
		ValueFlags: []string{"--log-level", "--port", "--hostname", "--mdns-domain", "--cors", "--replay-limit"},
	}
	agyReviewerOverrides = reviewerOverridePolicy{
		Flags:      []string{"--continue", "-c", "--conversation", "--print", "-p", "--prompt", "--prompt-interactive", "-i", "--model", "--effort", "--agent", "--project", "--gemini_dir"},
		ValueFlags: []string{"--add-dir", "--log-file", "--gemini_dir", "--mode"},
	}
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

	claudeAsyncKey = "async"

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
	noticeSeparator          = "\n\n"

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
	assetPathsRequiredError = "asset paths are required"
	assetPathRequiredError  = "asset path is required"
	unsupportedAssetError   = "unsupported asset format"
	assetEntryErrorFormat   = "asset %d %q: %w"
)

var (
	// editorEnvVars are consulted in order when no editor is configured.
	editorEnvVars = []string{"VISUAL", "EDITOR"}

	// fallbackEditors are tried last, as terminal editors that accept +line.
	fallbackEditors = []string{"nvim", "vim", "vi"}
)

const (
	ReviewerSubcommand            = "reviewer"
	ReviewRequestFlag             = "request"
	reviewerModelFlag             = "--model"
	reviewerEffortFlag            = "--effort"
	reviewerAgentFlag             = "--agent"
	claudeAppendSystemFlag        = "--append-system-prompt"
	codexDeveloperInstructionsKey = "developer_instructions="
	codexReasoningKey             = "model_reasoning_effort="
	openCodeAgentsKey             = "agent"
	openCodeModeKey               = "mode"
	openCodePrimaryValue          = "primary"
	openCodeAgentPromptKey        = "prompt"
	reviewerAgentName             = "qrouton-reviewer"
	reviewFieldLimit              = 16 << 10
	reviewPrivateMode             = 0o700
	reviewRequestMode             = 0o400
	reviewReportMode              = 0o644
	reviewWindowPrefix            = "review-"
	reviewTitlePrefix             = "Independent review "
	reviewPlaceholderFormat       = "---\nkind: review\ntitle: %s\nplan: %s\n---\n\n# %s\n\nNo completed review has been recorded. This request may be pending, interrupted, or failed.\n\n[Plan](<%s>)\n\nReviewer: %s; model: %s; reasoning: %s.\n\n%s"
	reviewRepoFormat              = "- %s (%s): base `%s`, head `%s`\n"
	reviewTaskFormat              = "Act only as the independent implementation reviewer, regardless of the conversation orchestrator instructions at the session root. Do not delegate, modify implementation or reference files, or publish. Write only the assigned report; preserve its kind, title, relative plan metadata and visible Plan link. Use absolute paths within the session root below for project context. Read its session manifest for reference repositories and read applicable repository instructions. Reference repositories provide read-only context. Review committed changes at these pinned commits; later HEAD changes are outside this review.\n\nSession root: %s\nPlan: %s\nSpec: %s\nReport: %s\nReviewer: %s; model: %s; reasoning: %s\n\nRepositories:\n%s\nPurpose:\n%s\n\nPrior verification:\n%s"
	reviewDefaultReasoning        = "provider default"
)
