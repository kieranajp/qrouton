package status

import "github.com/kieranajp/qrouton/internal/kinds"

const (
	modeAssistantLabel = "ASSISTANT"
	modeRPILabel       = "RPI"

	KindNote = "NOTE"

	repoSeparator  = "/"
	fieldSeparator = " "
	markdownSuffix = ".md"

	StageResearch  = "research"
	StagePlan      = "plan"
	StageImplement = "implement"

	legacyQuestionsSuffix = "-questions.md"
)

var (
	researchDir = kinds.Research.Dir
	plansDir    = kinds.Plan.Dir
	auditsDir   = kinds.Audit.Dir
)

// An audit's summary and tell headings. The audit counts as complete once
// each holds content, and the template carries each of them.
const (
	AuditHeadingSummary    = "Summary"
	AuditHeadingDuplicated = "Duplicated behaviour or facts"
	AuditHeadingDead       = "Dead or needlessly defensive code"
	AuditHeadingHomegrown  = "Homegrown library or platform behaviour"
)

var auditCompleteHeadings = []string{AuditHeadingSummary, AuditHeadingDuplicated, AuditHeadingDead, AuditHeadingHomegrown}

// Activity is what the workbench observes. Waiting only ever comes from the
// runner's own hook, so a runner without one never reports it.
const (
	ActivityWorking = "working"
	ActivityWaiting = "waiting"
	ActivityIdle    = "idle"

	// ActivityTurnEnded is the runner saying its own turn is over. It is never
	// displayed: it becomes waiting, or nothing while delegated work runs.
	ActivityTurnEnded = "turn-ended"
)

const (
	AgentAttentionUnread  = "unread"
	AgentAttentionNone    = "none"
	AgentAttentionUnknown = "unknown"

	AgentCoverageFull = "full"
	AgentCoverageRoot = "root"
	AgentCoverageNone = "none"

	// AgentState is one agent's state. Idle is only ever the PTY's timing, for
	// a runner that reports no tool calls.
	AgentStateWorking       = "working"
	AgentStateWaitingAgents = "waiting_agents"
	AgentStateNeedsYou      = "needs_you"
	AgentStateYourTurn      = "your_turn"
	AgentStateIdle          = "idle"
	AgentStateDone          = "done"
	AgentStateFailed        = "failed"
	AgentStateEnded         = "ended"
)
