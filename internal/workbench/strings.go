package workbench

import "time"

const (
	envKeyValueSep = "="

	handleParseError = "parse session handle"
)

const (
	LifecycleStart = "start"
	LifecycleStop  = "stop"
)

// Operations the control socket understands: one per WindowHost method, plus
// the attention signal the runner's own hooks raise.
const (
	OpOpen               = "open"
	OpClose              = "close"
	OpRead               = "read"
	OpViewport           = "viewport"
	OpExists             = "exists"
	OpList               = "list"
	OpPicker             = "picker"
	OpAttention          = "attention"
	OpRunnerGeneration   = "runner-generation"
	OpDelegatedLifecycle = "delegated-lifecycle"
	OpOpenLinearIssue    = "open-linear-issue"
)

const (
	socketNetwork = "unix"

	// socketRoot is not /tmp/qrouton: that is where a scratch build of the
	// binary conventionally lands, and a file there would block every mkdir.
	socketRoot         = "/tmp/qrouton-sock"
	socketSuffix       = ".sock"
	logSuffix          = ".log"
	activeName         = "active-workbench.json"
	descriptorLockName = "active-workbench.lock"
	launchLockName     = "launch.lock"
	temporaryPattern   = ".workbench-*"
	socketTokenBytes   = 6
	socketDirMode      = 0o700
	descriptorMode     = 0o600
	descriptorVersion  = 1

	// callTimeout bounds a request whose caller set no deadline, so a wedged
	// desktop process cannot hang an agent's tool call.
	callTimeout = 30 * time.Second
)
