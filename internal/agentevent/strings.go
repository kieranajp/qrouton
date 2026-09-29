package agentevent

import "time"

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

	providerCodex = "codex"
)
