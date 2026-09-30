package agent

import "github.com/kieranajp/qrouton/internal/launch"

const (
	commandName  = launch.AgentSubcommand
	commandUsage = "Supervise a session's agent runner, relaunching it on escalation or de-escalation (used by the workbench)"

	sessionRootFlag  = launch.SessionRootFlag
	sessionRootUsage = "qrouton session root"

	runnerFlag  = launch.RunnerFlag
	runnerUsage = "runner identifier to launch (claude, codex, opencode, agy)"

	workbenchJSONFlag  = launch.WorkbenchJSONFlag
	workbenchJSONUsage = "workbench handle stamped by the launcher"

	editorJSONFlag  = launch.EditorJSONFlag
	editorJSONUsage = "resolved editor configuration"

	resumeFlag  = launch.ResumeFlag
	resumeUsage = "continue the runner's previous conversation on first launch"
)
