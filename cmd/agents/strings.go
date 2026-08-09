package agents

import "time"

const (
	eventCommandName  = "agent-event"
	eventCommandUsage = "Record a Claude hook event from stdin and tell the workbench what it said"

	sessionRootFlag  = "session-root"
	sessionRootUsage = "qrouton session root"

	workbenchJSONFlag  = "workbench-json"
	workbenchJSONUsage = "workbench handle stamped by the launcher"

	hookNotification  = "Notification"
	hookSubagentStart = "SubagentStart"

	signalTimeout = 2 * time.Second
)
