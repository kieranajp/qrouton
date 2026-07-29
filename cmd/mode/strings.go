package mode

const (
	commandName      = "mode"
	commandUsage     = "Set a session's mode and relaunch its agent (assistant keeps the conversation; used by the Alt-n binding)"
	commandArgsUsage = "<assistant|rpi>"

	sessionRootFlag  = "session-root"
	sessionRootUsage = "qrouton session root"

	shellStackFlag  = "shell-stack"
	shellStackUsage = "place this internal action in the workspace shell stack"

	deescalatingPaneSuffix = " · switching to assistant"
)
