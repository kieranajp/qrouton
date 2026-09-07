package desktop

import (
	"time"

	"github.com/kieranajp/qrouton/internal/status"
)

const (
	applicationName        = "qrouton"
	applicationDescription = "qrouton workbench"

	mainWindowName   = "conversation"
	mainWindowTitle  = "qrouton"
	titleSeparator   = " — "
	mainWindowWidth  = 1100
	mainWindowHeight = 760

	shellWindowLabel        = "$ shell"
	shellWindowLabelNumbers = "$ shell %d"

	// The page URL is a directory: http.FileServer 301-redirects
	// /index.html to /, and the webview does not follow the redirect.
	frontendRoot = "/"

	assetRoot = "assets"

	staleFrontendError         = "workbench frontend is stale; run make front before building qrouton"
	staleFrontendBindingFormat = "%w: built pages do not call %s"

	noOwnersError = "add at least one organisation or username to search"

	// frontendSource is where the pages are written, not where they are built.
	frontendSource = "frontend/src/"

	rootPath          = "/"
	contentTypeHeader = "Content-Type"

	// deckAssetPath leads a deck's own media, addressed by the per-window token
	// that follows it. Window ids are sequential and never appear in a URL.
	deckAssetPath = "/deck/"

	windowIDFormat = "window-%d"

	terminalIDFormat = "term-%d"
)

// Events the Go side emits at the pages; a window's or a conversation's own id
// is appended so each page hears only its own stream.
const (
	ptyDataEvent = "pty:data:"
	ptyExitEvent = "pty:exit:"
	chromeEvent  = "chrome:update"

	windowDataEvent    = "window:data:"
	windowExitEvent    = "window:exit:"
	windowDiagramEvent = "window:diagram:"
	windowContentEvent = "window:content:"
	windowsEvent       = "window:open"

	reposRefreshEvent      = "repos:refresh"
	assemblyProgressEvent  = "assembly:progress"
	assemblyRequestedEvent = "assembly:requested"
	orgsChangedEvent       = "orgs:changed"
)

// A tab may only stand in for a window if it reports its process's state.
const (
	tabStatusRunning   = "running"
	tabStatusSucceeded = "succeeded"
	tabStatusFailed    = "failed"
	tabStatusWaiting   = "waiting"
)

const (
	socketNetwork = "unix"

	termEnvVar      = "TERM"
	termValue       = "xterm-256color"
	colorTermEnvVar = "COLORTERM"
	colorTermValue  = "truecolor"
	noColorEnvVar   = "NO_COLOR"

	// ptyReadBuffer is one read of the PTY. An agent painting a full-screen
	// frame produces well under this, so a repaint arrives as one event.
	ptyReadBuffer = 32 * 1024

	windowScrollback = 256 * 1024

	// windowScreenLines is what a read without full returns.
	windowScreenLines = 50

	// agentTailBytes is how much of a conversation is kept back for the log its
	// supervisor's exit writes; agentLogLimit is when that log is rotated.
	agentTailBytes = 8 * 1024
	agentLogLimit  = 256 * 1024
)

const (
	agentLogPreviousSuffix = ".1"

	agentExitLogFormat  = "%s agent exited: provider=%s status=%d\n"
	agentExitTailFormat = "--- last output ---\n%s\n--- end output ---\n"
)

const documentPoll = time.Second

const (
	// chromeInterval bounds how stale the window chrome can be after an
	// escalation.
	chromeInterval = 2 * time.Second

	// repoStatInterval paces the git stats: three subprocesses per active repo.
	repoStatInterval = 15 * time.Second

	// activityQuiet is how long the conversation PTY has to stay silent before
	// the agent counts as idle. A runner redraws its spinner far faster.
	activityQuiet = 3 * time.Second

	finishedAgentRetention = 90 * time.Second

	// terminateGrace is how long a PTY's process tree gets to exit on SIGTERM
	// before it is killed outright. launch has its own for the runner the
	// supervisor signals; the two bound different processes and need not agree.
	terminateGrace = 3 * time.Second

	// loginTimeout bounds the GitHub lookup behind the owners screen's help line,
	// which fills in after the screen has already drawn.
	loginTimeout       = 5 * time.Second
	ticketFetchTimeout = 15 * time.Second
)

const (
	agentProviderClaude   = "claude"
	agentProviderCodex    = "codex"
	agentProviderOpenCode = "opencode"

	agentRootID         = "root"
	agentSetupRunPrefix = "setup-"

	agentRoleOrchestrator = status.AgentRoleOrchestrator
	agentRoleLead         = status.AgentRoleLead
	agentRoleSpecialist   = status.AgentRoleSpecialist
	agentRoleUnavailable  = status.AgentRoleUnavailable

	agentStateWaiting  = status.AgentStateWaiting
	agentStateWorking  = status.AgentStateWorking
	agentStateIdle     = status.AgentStateIdle
	agentStateActive   = status.AgentStateActive
	agentStateFinished = status.AgentStateFinished
	agentStateFailed   = status.AgentStateFailed
)

// deckMediaTypes is the whole of what a deck can reach through its asset route.
var deckMediaTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".svg":  "image/svg+xml",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
}
