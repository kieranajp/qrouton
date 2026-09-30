package agentevent

import (
	eventlog "github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/launch"
)

const (
	eventCommandName  = launch.AgentEventSubcommand
	eventCommandUsage = "Record an agent lifecycle hook event from stdin and tell the workbench what it said"

	workbenchJSONFlag  = launch.WorkbenchJSONFlag
	workbenchJSONUsage = "workbench handle stamped by the launcher"
	generationFlag     = launch.GenerationFlag
	generationUsage    = "runner generation stamped by the supervisor"
	providerFlag       = launch.ProviderFlag
	providerUsage      = "runner provider that emitted the event"

	hookNotification = eventlog.HookNotification
	hookStop         = eventlog.HookStop

	signalTimeout = eventlog.SignalTimeout
)
