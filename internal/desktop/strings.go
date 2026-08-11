package desktop

import "time"

const (
	applicationName        = "qrouton"
	applicationDescription = "qrouton workbench"

	mainWindowName   = "conversation"
	mainWindowTitle  = "qrouton"
	titleSeparator   = " — "
	mainWindowWidth  = 1100
	mainWindowHeight = 760

	agentWindowWidth  = 900
	agentWindowHeight = 620

	shellWindowLabel = "$ shell"

	// The page URLs are directories: http.FileServer 301-redirects
	// /index.html to /, and the webview does not follow the redirect.
	frontendRoot  = "/"
	terminalPage  = "/terminal/"
	documentPage  = "/document/"
	windowIDQuery = "?id="

	assetRoot = "assets"

	windowIDFormat = "window-%d"
)

// Events the Go side emits at the pages; an agent window's own id is appended
// so each page hears only its own stream.
const (
	ptyDataEvent = "pty:data"
	ptyExitEvent = "pty:exit"
	chromeEvent  = "chrome:update"

	windowDataEvent = "window:data:"
	windowExitEvent = "window:exit:"
)

const (
	socketNetwork = "unix"

	termEnvVar      = "TERM"
	termValue       = "xterm-256color"
	colorTermEnvVar = "COLORTERM"
	colorTermValue  = "truecolor"

	// ptyReadBuffer is one read of the PTY. An agent painting a full-screen
	// frame produces well under this, so a repaint arrives as one event.
	ptyReadBuffer = 32 * 1024

	// windowScrollback caps what a window keeps for the agent to read back.
	windowScrollback = 256 * 1024

	// windowScreenLines is what a read without full returns.
	windowScreenLines = 50
)

const (
	// chromeInterval bounds how stale the window chrome can be after an
	// escalation.
	chromeInterval = 2 * time.Second

	// terminateGrace is how long a process tree gets to exit on SIGTERM before
	// it is killed outright.
	terminateGrace = 3 * time.Second
)
