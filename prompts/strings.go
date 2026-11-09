package prompts

import "strconv"

// DigestLimit is the most characters a session digest may hold.
const DigestLimit = 600

const (
	claudeAgentsDir = ".claude/agents/"
	codexAgentsDir  = ".codex/agents/"
	// agy discovers an agent as a directory holding agent.md, so its name is a
	// path segment rather than a file stem.
	agyAgentsDir     = ".agents/agents/"
	agyAgentFileName = "agent.md"

	subagentChoiceFileName    = "subagent-choice.md"
	subagentChoicePlaceholder = "{{subagent-choice}}"

	workspaceWindowsFileName    = "workspace-windows.md"
	workspaceWindowsPlaceholder = "{{workspace-windows}}"

	artifactDisciplineFileName    = "artifact-discipline.md"
	artifactDisciplinePlaceholder = "{{artifact-discipline}}"

	evidenceDisciplineFileName    = "evidence-discipline.md"
	evidenceDisciplinePlaceholder = "{{evidence-discipline}}"

	returnContractFileName    = "return-contract.md"
	returnContractPlaceholder = "{{return-contract}}"

	sessionSummaryFileName    = "session-summary.md"
	sessionSummaryPlaceholder = "{{session-summary}}"

	sessionDigestFileName    = "session-digest.md"
	sessionDigestPlaceholder = "{{session-digest}}"

	digestLimitPlaceholder = "{{digest-limit}}"

	scratchCopyFileName    = "scratch-copy.md"
	scratchCopyPlaceholder = "{{scratch-copy}}"

	fixDisciplineFileName    = "fix-discipline.md"
	fixDisciplinePlaceholder = "{{fix-discipline}}"

	runTableFileName    = "run-table.md"
	runTablePlaceholder = "{{run-table}}"

	agyNameFormat   = "name: %s\ndescription: %s\n"
	agySubagentKey  = "subagent"
	agySubagentLine = agySubagentKey + ": true\n"

	codexNameFormat        = "name = %s\ndescription = %s\n"
	codexSandboxFormat     = "sandbox_mode = %s\n"
	codexInstructionsOpen  = "developer_instructions = \"\"\"\n"
	codexInstructionsClose = "\n\"\"\"\n"

	tomlTripleQuote        = `"""`
	tomlEscapedTripleQuote = `\"\"\"`

	sandboxReadOnly       = "read-only"
	sandboxWorkspaceWrite = "workspace-write"
)

// partials are the prose more than one prompt has to say, keyed by the
// placeholder that pulls each in. A partial holds no placeholder of its own, so
// one pass over this map expands them all.
var partials = map[string]string{
	subagentChoicePlaceholder:     subagentChoiceFileName,
	workspaceWindowsPlaceholder:   workspaceWindowsFileName,
	artifactDisciplinePlaceholder: artifactDisciplineFileName,
	evidenceDisciplinePlaceholder: evidenceDisciplineFileName,
	returnContractPlaceholder:     returnContractFileName,
	sessionSummaryPlaceholder:     sessionSummaryFileName,
	sessionDigestPlaceholder:      sessionDigestFileName,
	scratchCopyPlaceholder:        scratchCopyFileName,
	fixDisciplinePlaceholder:      fixDisciplineFileName,
	runTablePlaceholder:           runTableFileName,
}

// figures are numbers Go enforces, keyed by the placeholder a prompt states
// each through. They expand after the partials, so a partial may name one.
var figures = map[string]string{
	digestLimitPlaceholder: strconv.Itoa(DigestLimit),
}

// The leads, by the agent type the runner reports them as.
const (
	ResearchLead       = "qrouton-research-lead"
	AuditLead          = "qrouton-audit-lead"
	PlanningLead       = "qrouton-planning-lead"
	ImplementationLead = "qrouton-implementation-lead"
)

var (
	readOnlyAgents = map[string]bool{
		"audit-checker":       true,
		"audit-verifier":      true,
		"code-reviewer":       true,
		"codebase-researcher": true,
		"external-researcher": true,
		"pattern-finder":      true,
		"thoughts-researcher": true,
	}

	workspaceWriteAgents = map[string]bool{
		ResearchLead:       true,
		AuditLead:          true,
		PlanningLead:       true,
		ImplementationLead: true,
		"test-verifier":    true,
	}
)
