package shell

import "github.com/kieranajp/qrouton/internal/launch"

const (
	commandName  = launch.ShellSubcommand
	commandUsage = "Run the session's user shell (used by the workbench)"

	sessionRootFlag  = launch.SessionRootFlag
	sessionRootUsage = "qrouton session root"
)
