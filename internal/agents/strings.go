package agents

import "time"

// The subagent watcher's vocabulary: the states it renders, the runner and
// rollout event names it reads them from, and the pane's copy.

const (
	// paneTitle heads the pane. refreshInterval is how often it redraws.
	paneTitle       = "agents"
	refreshInterval = 2 * time.Second

	// maxVisibleAgents is how many subagents fit before the pane summarises the
	// remainder as a count.
	maxVisibleAgents = 4

	noSubagentsLabel       = "No subagents yet"
	claudeUnavailableLabel = "Claude status unavailable"
	codexUnavailableLabel  = "Codex status unavailable"

	moreAgentsFormat = "+%d more"
	agentLineFormat  = "%s %s  %s"

	markerRunning = "●"
	markerDone    = "✓"
	markerFailed  = "!"
)

// The three states the pane draws, whichever runner reported them.
const (
	stateRunning = "running"
	stateDone    = "done"
	stateFailed  = "failed"
)

// runnerClaude selects the Claude hook log; anything else is read from Codex's
// rollout files.
const runnerClaude = "claude"

// Claude subagent lifecycle hook event names.
const (
	hookSubagentStart = "SubagentStart"
	hookSubagentStop  = "SubagentStop"
)

// Record and event types in Codex's rollout logs.
const (
	rolloutSessionMeta = "session_meta"
	rolloutEventMsg    = "event_msg"

	rolloutTaskStarted  = "task_started"
	rolloutTaskComplete = "task_complete"
	rolloutTurnAborted  = "turn_aborted"
)

// rolloutBufferInitial and rolloutBufferMax size the scanner: rollout records
// can carry large instruction payloads.
const (
	rolloutBufferInitial = 64 * 1024
	rolloutBufferMax     = 8 * 1024 * 1024
)
