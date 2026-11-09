package core

import "time"

const (
	termEnvVar      = "TERM"
	termValue       = "xterm-256color"
	colorTermEnvVar = "COLORTERM"
	colorTermValue  = "truecolor"
	noColorEnvVar   = "NO_COLOR"

	// ptyReadBuffer is one read of the PTY. An agent painting a full-screen
	// frame produces well under this, so a repaint arrives as one event.
	ptyReadBuffer = 32 * 1024

	WindowScrollback = 256 * 1024

	// screenLines is what a read without full returns.
	screenLines = 50

	// AgentTailBytes is how much of a conversation is kept back for the log its
	// supervisor's exit writes; logLimit is when a log is rotated.
	AgentTailBytes = 8 * 1024
	logLimit       = 256 * 1024
)

const (
	logPreviousSuffix = ".1"

	agentExitLogFormat  = "%s agent exited: provider=%s status=%d\n"
	agentExitTailFormat = "--- last output ---\n%s\n--- end output ---\n"
)

// terminateGrace is how long a PTY's process tree gets to exit on SIGTERM
// before it is killed outright. launch has its own for the runner the
// supervisor signals; the two bound different processes and need not agree.
const terminateGrace = 3 * time.Second
