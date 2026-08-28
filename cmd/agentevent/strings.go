package agentevent

import "time"

const (
	eventCommandName  = "agent-event"
	eventCommandUsage = "Record an agent lifecycle hook event from stdin and tell the workbench what it said"

	sessionRootFlag  = "session-root"
	sessionRootUsage = "qrouton session root"

	workbenchJSONFlag  = "workbench-json"
	workbenchJSONUsage = "workbench handle stamped by the launcher"
	generationFlag     = "generation"
	generationUsage    = "runner generation stamped by the supervisor"
	providerFlag       = "provider"
	providerUsage      = "runner provider that emitted the event"

	hookNotification = "Notification"

	signalTimeout = 2 * time.Second
)
