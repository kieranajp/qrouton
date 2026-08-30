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
)

// Activity is what the workbench observes. Waiting only ever comes from the
// runner's own hook, so a runner without one never reports it.
const (
	ActivityWorking = "working"
	ActivityWaiting = "waiting"
	ActivityIdle    = "idle"
)

const (
	AgentAttentionNeedsYou = "needs-you"
	AgentAttentionNone     = "none"
	AgentAttentionUnknown  = "unknown"

	AgentCoverageFull = "full"
	AgentCoverageRoot = "root"
	AgentCoverageNone = "none"

	AgentRoleOrchestrator = "Orchestrator"
	AgentRoleLead         = "Lead"
	AgentRoleSpecialist   = "Specialist"
	AgentRoleUnavailable  = "Role unavailable"

	AgentStateWaiting  = "Waiting for you"
	AgentStateWorking  = "Working"
	AgentStateIdle     = "Idle"
	AgentStateActive   = "Active"
	AgentStateFinished = "Finished"
	AgentStateFailed   = "Failed"
)
