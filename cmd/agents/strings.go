package agents

const (
	commandName  = "agents"
	commandUsage = "Watch a session's subagent statuses (redraws forever; used by the workspace layout)"

	eventCommandName  = "agent-event"
	eventCommandUsage = "Record a Claude subagent hook event from stdin"

	sessionRootFlag  = "session-root"
	sessionRootUsage = "qrouton session root"

	runnerFlag  = "runner"
	runnerUsage = "runner whose agents to scan (claude scans the session log, otherwise codex)"
)
