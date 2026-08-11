package onboard

const (
	commandName  = "onboard"
	commandUsage = "Choose or create a session, then hand the terminal over to its agent (used by the workbench)"

	socketFlag  = "socket"
	socketUsage = "workbench control socket to adopt the chosen session on"

	runnerFlag  = "runner"
	runnerUsage = "coding agent to launch (claude, codex, or opencode)"

	refreshFlag  = "refresh"
	refreshUsage = "refresh the cached org repo list"
)
