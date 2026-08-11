package agents

// The subagent readers' vocabulary: the states they report, and the runner and
// rollout event names they read them from.

// The three states a subagent is reported in, whichever runner reported it.
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
