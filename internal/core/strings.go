package core

import (
	"time"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

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

const (
	ollamaStateReady       = "ready"
	ollamaStateMissing     = "missing"
	ollamaStatePulling     = "pulling"
	ollamaStateUnreachable = "unreachable"
	ollamaStateFailed      = "failed"

	ollamaBinary       = "ollama"
	ollamaMacApp       = "/Applications/Ollama.app"
	ollamaProbeTimeout = 2 * time.Second
)

const (
	bugReportReviewLabel      = "Review bug report"
	bugReportCreateLabel      = "Create issue"
	bugReportCancelLabel      = "Cancel"
	bugReportCloseLabel       = "Close"
	bugReportPendingMessage   = "Review the bug report in qrouton before creating an issue."
	bugReportPostingMessage   = "Creating the confirmed GitHub issue…"
	bugReportCancelledMessage = "Bug report cancelled. No issue was sent."
	bugReportExpiredMessage   = "Bug report review expired. No issue was sent."
	bugReportCreatedFormat    = "Created GitHub issue #%d."
	bugReportRetention        = 30 * time.Minute
	bugReportPostTimeout      = 30 * time.Second

	specAnswersInFormat = "I've answered the design questions in %s. Please fold them into Decisions."
	auditTriageInFormat = "I've triaged the findings in %s. Please plan the accepted findings."
	conversationSubmit  = "\r"
	pasteStart          = "\x1b[200~"
	pasteEnd            = "\x1b[201~"

	overviewWindowLabel     = "Overview"
	shellWindowLabel        = "shell"
	shellWindowLabelNumbers = "shell %d"

	windowIDFormat = "window-%d"

	terminalIDFormat = "term-%d"
)

// A tab may only stand in for a window if it reports its process's state.
const (
	tabStatusRunning   = "running"
	tabStatusSucceeded = "succeeded"
	tabStatusFailed    = "failed"
	tabStatusWaiting   = "waiting"
	tabStatusIdle      = "idle"
)

const (
	socketNetwork = "unix"

	// An agent's terminal starts at this size when no page has measured it yet.
	unmountedCols = 80
	unmountedRows = 24
)

const documentPoll = time.Second

// vaultTimeout stays under the socket client's deadline, so a slow embed
// answers with an error rather than a dropped connection.
const vaultTimeout = workbench.CallTimeout - 5*time.Second

const (
	// chromeInterval bounds how stale the window chrome can be after an
	// escalation.
	chromeInterval = 2 * time.Second

	// foregroundInterval paces the read of each user shell's foreground job.
	foregroundInterval = 500 * time.Millisecond

	// repoStatInterval paces the git stats: three subprocesses per active repo.
	repoStatInterval = 15 * time.Second

	// pullRequestInterval paces the shown session's GitHub pull request fetch.
	pullRequestInterval  = 60 * time.Second
	pullRequestTimeout   = 10 * time.Second
	pullRequestLogFormat = "pull requests: %s: %v"

	// activityQuiet is how long the conversation PTY has to stay silent before
	// the agent counts as idle. A runner redraws its spinner far faster.
	activityQuiet = 3 * time.Second
)

const (
	agentRootID         = "root"
	agentSetupRunPrefix = "setup-"

	agentStateWorking       = status.AgentStateWorking
	agentStateWaitingAgents = status.AgentStateWaitingAgents
	agentStateNeedsYou      = status.AgentStateNeedsYou
	agentStateYourTurn      = status.AgentStateYourTurn
	agentStateIdle          = status.AgentStateIdle
	agentStateDone          = status.AgentStateDone
	agentStateFailed        = status.AgentStateFailed
	agentStateEnded         = status.AgentStateEnded
	// agentLive marks a record still running; its shown state is derived.
	agentLive = "live"

	// agentTreeInterval coalesces tree pushes; tool calls arrive several a second.
	agentTreeInterval = 250 * time.Millisecond
	agentFileLimit    = 50

	agentCoverageRootOnly = "%s reports the root agent only, so working and idle come from its terminal."
	agentCoverageNoTools  = "%s reports its agents starting and stopping, not what they are doing."

	agentReasonQuestion = "Asking you a question"
	agentReasonPlan     = "Plan ready for your review"

	notificationPermission  = "permission_prompt"
	notificationElicitation = "elicitation_dialog"

	// agentRecordCap bounds one session's tree; the oldest finished leaves go first.
	agentRecordCap = 500
	// agentToolHistory is how many tool calls an agent remembers.
	agentToolHistory      = 20
	agentOutcomeCompleted = "completed"
)

const (
	galleryAssetURLFormat = "/gallery/%s/%d"
	currentAssetFormat    = "Current asset: %d of %d"
	assetTokenBytes       = 16
)

const assetIndexErrorFormat = "%w: %d (expected 1 through %d)"

const (
	ticketFetchTimeout = 15 * time.Second

	// branchListTimeout bounds the branch listing GitHub is asked for, which the
	// base menu opens without waiting for.
	branchListTimeout = 5 * time.Second

	// repoRefreshTimeout bounds each owner's repository listing.
	repoRefreshTimeout = 30 * time.Second
)

const (
	placementLeavingNote  = "Moving a session out of a shared folder stops sharing it. Your sync tool will usually delete the folder on your teammates' machines, but versioned copies may remain."
	placementRelaunchNote = "A saved change to your folders is waiting for a relaunch, so this check uses the routing sessions are created with today."

	noOwnersError = "add at least one organisation or username to search"
)

// A Save refusal names the settings field it belongs under before the sentence.
const (
	settingsFieldOrgs     = "orgs"
	settingsFieldRoot     = "root"
	settingsFieldEditor   = "editor"
	settingsFieldLaunch   = "launch"
	settingsFieldStickers = "stickers"
	settingsFieldUIScale  = "uiScale"
	settingsFieldThoughts = "thoughts"

	thoughtsOrgJoin  = ", "
	thoughtsOrgSplit = ","

	settingsRefusalFormat = "%s: %s"
	settingsWrappedFormat = "%s: %w"
	settingsEmpty         = "cannot be empty"
	stickerFieldFormat    = "%s.%d"
	stickerDuplicate      = "another sticker already uses this colour and symbol"
	stickerIDBytes        = 4
	uiScaleRefusal        = "choose a scale from 80% to 150% in steps of 10%"
	unknownUIScaleAction  = "unknown UI scale action"
)

// What a scale shortcut asks the workbench to do.
const (
	uiScaleActionIn    = "in"
	uiScaleActionOut   = "out"
	uiScaleActionReset = "reset"
)

const (
	// loginTimeout bounds the GitHub lookup behind the owners screen's help line,
	// which fills in after the screen has already drawn.
	loginTimeout       = 5 * time.Second
	githubCheckTimeout = 4 * time.Second
)
