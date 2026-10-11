package status

const (
	modeAssistantLabel = "ASSISTANT"
	modeRPILabel       = "RPI"

	phaseScratch   = "scratch"
	phaseResearch  = "Research"
	phasePlan      = "Plan"
	phaseImplement = "Implement"

	KindPlan      = "PLAN"
	KindSpec      = "SPEC"
	KindResearch  = "RESEARCH"
	KindNote      = "NOTE"
	KindExplainer = "EXPLAINER"

	repoSeparator  = "/"
	fieldSeparator = " "
	markdownSuffix = ".md"

	StageResearch  = "research"
	StagePlan      = "plan"
	StageImplement = "implement"

	researchDir           = "research"
	plansDir              = "plans"
	legacyQuestionsSuffix = "-questions.md"
	summaryHeading        = "Summary"
)

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
