package mode

import "github.com/kieranajp/qrouton/internal/launch"

const (
	commandName      = "mode"
	commandUsage     = "Set a session's mode and relaunch its agent (assistant keeps the conversation)"
	commandArgsUsage = "<assistant|rpi>"

	sessionRootFlag  = launch.SessionRootFlag
	sessionRootUsage = "qrouton session root"
)
