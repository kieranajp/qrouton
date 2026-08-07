package agent

const (
	commandName  = "agent"
	commandUsage = "Supervise a session's agent runner, relaunching it on escalation or de-escalation (used by the workbench)"

	sessionRootFlag  = "session-root"
	sessionRootUsage = "qrouton session root"

	runnerFlag  = "runner"
	runnerUsage = "runner identifier to launch (claude, codex, opencode)"

	workbenchJSONFlag  = "workbench-json"
	workbenchJSONUsage = "workbench handle stamped by the launcher"

	editorJSONFlag  = "editor-json"
	editorJSONUsage = "resolved editor configuration"

	resumeFlag  = "resume"
	resumeUsage = "continue the runner's previous conversation on first launch"
)
