package agentevent

import (
	"time"

	"github.com/kieranajp/qrouton/internal/codex"
)

// SignalTimeout bounds the hook's push to the workbench, which must finish
// inside the time a runner gives a hook before killing it.
const SignalTimeout = 2 * time.Second

const (
	HookSubagentStart = "SubagentStart"
	HookSubagentStop  = "SubagentStop"
	HookNotification  = "Notification"
	HookStop          = "Stop"

	QroutonBinEnvVar = "QROUTON_AGENT_EVENT_BIN"
	WorkbenchEnvVar  = "QROUTON_AGENT_EVENT_WORKBENCH"
	GenerationEnvVar = "QROUTON_AGENT_EVENT_GENERATION"
	ProviderEnvVar   = "QROUTON_AGENT_EVENT_PROVIDER"

	providerCodex = codex.Binary
)

const (
	HookPreToolUse         = "PreToolUse"
	HookPostToolUse        = "PostToolUse"
	HookPostToolUseFailure = "PostToolUseFailure"
	HookUserPromptSubmit   = "UserPromptSubmit"
	HookStopFailure        = "StopFailure"

	// TargetLimit and MessageLimit are in runes, and keep a reduced event small
	// whatever the tool was handed or the model wrote.
	TargetLimit  = 160
	MessageLimit = 1000
	// IDLimit bounds ids, names and statuses; ListLimit bounds how many
	// background tasks and task notifications one event carries.
	IDLimit   = 128
	ListLimit = 8

	ToolAskUserQuestion = "AskUserQuestion"
	ToolExitPlanMode    = "ExitPlanMode"

	ToolAgent        = "Agent"
	ToolTaskLegacy   = "Task"
	toolBash         = "Bash"
	toolGrep         = "Grep"
	toolGlob         = "Glob"
	toolWebFetch     = "WebFetch"
	toolWebSearch    = "WebSearch"
	mcpToolPrefix    = "mcp__"
	mcpToolSeparator = "__"
	mcpTargetJoin    = " · "
	truncationMark   = "…"

	spawnAsyncStatus   = "async_launched"
	backgroundSubagent = "subagent"

	notificationOpen  = "<task-notification>"
	notificationClose = "</task-notification>"
	tagTaskID         = "task-id"
	tagToolUseID      = "tool-use-id"
	tagStatus         = "status"
	tagResult         = "result"
	tagUsage          = "usage"
	tagSubagentTokens = "subagent_tokens"
	tagTotalTokens    = "total_tokens"
	tagToolUses       = "tool_uses"
	tagDurationMs     = "duration_ms"
)

// FileWriteTools are the tools whose target is a file the agent wrote.
var FileWriteTools = []string{"Edit", "Write", "MultiEdit", "NotebookEdit"}

// fileInputKeys are where a file tool names its file, in the order tried.
var fileInputKeys = []string{"file_path", "notebook_path", "path"}
