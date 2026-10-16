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
// the summary and each chosen tell hold content.
const (
	AuditHeadingSummary    = "Summary"
	AuditHeadingDuplicated = "Duplicated behaviour or facts"
	AuditHeadingDead       = "Dead or needlessly defensive code"
	AuditHeadingHomegrown  = "Homegrown library or platform behaviour"
)

// The audit frontmatter key that lists the chosen tells, and each tell's slug.
const (
	tellsKey       = "tells"
	tellDuplicated = "duplicated"
	tellDead       = "dead"
	tellHomegrown  = "homegrown"
)

var auditTells = []auditTell{
	{tellDuplicated, AuditHeadingDuplicated},
	{tellDead, AuditHeadingDead},
	{tellHomegrown, AuditHeadingHomegrown},
}

// legacyTells are the tells an audit written before the tells key ran.
var legacyTells = []string{tellDuplicated, tellDead, tellHomegrown}

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
