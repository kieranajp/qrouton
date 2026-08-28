package status

// The window's copy: mode, phase and document-kind labels.

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
