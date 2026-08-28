package desktop

import "time"

const (
	applicationName        = "qrouton"
	applicationDescription = "qrouton workbench"
	linearConfigPath       = "~/.linear/coding-tools.json"
	linearIssueTemplate    = "{{issue.identifier}}"

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

	// rootPath is the mux pattern the embedded tree is served under.
	rootPath          = "/"
	contentTypeHeader = "Content-Type"

	windowIDFormat = "window-%d"

	terminalIDFormat = "term-%d"
)

const (
	linearConfigDirMode  = 0o755
	linearConfigFileMode = 0o644
)

// Events the Go side emits at the pages; a window's or a conversation's own id
// is appended so each page hears only its own stream.
const (
	ptyDataEvent = "pty:data:"
	ptyExitEvent = "pty:exit:"
	chromeEvent  = "chrome:update"

	windowDataEvent = "window:data:"
	windowExitEvent = "window:exit:"
	windowsEvent    = "window:open"

	reposRefreshEvent      = "repos:refresh"
	assemblyProgressEvent  = "assembly:progress"
	assemblyRequestedEvent = "assembly:requested"

	assemblyOutcomeDraft    = "draft"
	assemblyOutcomeExisting = "existing-session"
	assemblyOutcomeQueued   = "queued"
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

	// windowScrollback caps what a window keeps for the agent to read back.
	windowScrollback = 256 * 1024

	// windowScreenLines is what a read without full returns.
	windowScreenLines = 50
)

const (
	// chromeInterval bounds how stale the window chrome can be after an
	// escalation.
	chromeInterval = 2 * time.Second

	// repoStatInterval paces the git stats: two subprocesses per active repo.
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

	agentRootID = "root"

	agentRoleOrchestrator = "Orchestrator"
	agentRoleLead         = "Lead"
	agentRoleSpecialist   = "Specialist"
	agentRoleUnavailable  = "Role unavailable"

	agentStateWaiting  = "Waiting for you"
	agentStateWorking  = "Working"
	agentStateIdle     = "Idle"
	agentStateActive   = "Active"
	agentStateFinished = "Finished"
	agentStateFailed   = "Failed"
)
