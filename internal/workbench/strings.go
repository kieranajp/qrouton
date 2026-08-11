package workbench

import "time"

const (
	envKeyValueSep = "="

	handleParseError = "parse session handle"
)

// Operations the control socket understands: one per WindowHost method, plus
// the attention signal the runner's own hooks raise.
const (
	OpOpen      = "open"
	OpClose     = "close"
	OpRead      = "read"
	OpExists    = "exists"
	OpList      = "list"
	OpAdopt     = "adopt"
	OpAttention = "attention"
)

const (
	socketNetwork = "unix"

	// socketRoot is not /tmp/qrouton: that is where a scratch build of the
	// binary conventionally lands, and a file there would block every mkdir.
	socketRoot       = "/tmp/qrouton-sock"
	socketSuffix     = ".sock"
	logSuffix        = ".log"
	socketTokenBytes = 6
	socketDirMode    = 0o700

	// callTimeout bounds a request whose caller set no deadline, so a wedged
	// desktop process cannot hang an agent's tool call.
	callTimeout = 30 * time.Second
)
