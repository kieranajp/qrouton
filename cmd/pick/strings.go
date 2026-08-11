package pick

const (
	commandName  = "pick"
	commandUsage = "Choose repositories to assemble into a live session (used by the workbench's add-repos button and the escalate tool)"

	sessionRootFlag  = "session-root"
	sessionRootUsage = "qrouton session root"

	nameFlag  = "name"
	nameUsage = "pre-filled name for the piece of work"

	prefixFlag  = "prefix"
	prefixUsage = "pre-filled branch prefix (feat, fix, chore, refactor, docs, test)"

	escalateFlag  = "escalate"
	escalateUsage = "also move the session to RPI mode on confirmation"
)
