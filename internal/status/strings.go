package status

import (
	"github.com/kieranajp/qrouton/internal/kinds"
	"github.com/kieranajp/qrouton/prompts"
)

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

// A pull request row's state, the page's icon and colour.
const (
	PullRequestStateOpen   = "open"
	PullRequestStateDraft  = "draft"
	PullRequestStateMerged = "merged"
	PullRequestStateClosed = "closed"
)

var (
	researchDir = kinds.Research.Dir
	plansDir    = kinds.Plan.Dir
	auditsDir   = kinds.Audit.Dir
)

const (
	AuditHeadingSummary     = prompts.AuditHeadingSummary
	AuditHeadingDuplicated  = prompts.AuditHeadingDuplicated
	AuditHeadingDead        = prompts.AuditHeadingDead
	AuditHeadingHomegrown   = prompts.AuditHeadingHomegrown
	AuditHeadingVolkswagen  = prompts.AuditHeadingVolkswagen
	tellsKey                = "tells"
	legacyVolkswagenHeading = prompts.LegacyAuditHeadingVolkswagen
)

var auditTells = prompts.AuditTells()

var legacyTells = func() []string {
	var out []string
	for _, tell := range auditTells {
		if tell.LegacyDefault {
			out = append(out, tell.Slug)
		}
	}
	return out
}()

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
