package prompts

import "strconv"

// DigestLimit is the most characters a session digest may hold.
const DigestLimit = 600

const SummaryLimit = 140

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

	digestLimitPlaceholder  = "{{digest-limit}}"
	summaryLimitPlaceholder = "{{summary-limit}}"

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
	"{{audit-policy}}":            "audit-policy.md",
}

// figures are numbers Go enforces, keyed by the placeholder a prompt states
// each through. They expand after the partials, so a partial may name one.
var figures = map[string]string{
	digestLimitPlaceholder:    strconv.Itoa(DigestLimit),
	summaryLimitPlaceholder:   strconv.Itoa(SummaryLimit),
	"{{audit-tell-slugs}}":    auditTellSlugs(),
	"{{audit-tell-sections}}": auditTellSections(),
	"{{audit-tell-table}}":    auditTellTable(),
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

const (
	AuditHeadingSummary          = "Summary"
	AuditHeadingDuplicated       = "Duplicated behaviour or facts"
	AuditHeadingDead             = "Dead or needlessly defensive code"
	AuditHeadingHomegrown        = "Homegrown library or platform behaviour"
	AuditHeadingVolkswagen       = "Tests that pass or fail for the wrong reason"
	LegacyAuditHeadingVolkswagen = "Tests that cannot fail or pass for the wrong reason"
)

var auditTellMetadata = []AuditTell{
	{"duplicated", AuditHeadingDuplicated, "", "findings where one behaviour or fact is expressed more than one way, kept in step only by convention", true},
	{"dead", AuditHeadingDead, "", "findings where code is unreachable, or guards a case that cannot occur", true},
	{"homegrown", AuditHeadingHomegrown, "", "findings where code redoes what a well-known library or the platform provides, installed or not", true},
	{"volkswagen", AuditHeadingVolkswagen, LegacyAuditHeadingVolkswagen, "findings where a test never runs, cannot fail, dodges the code it names, was weakened to pass, restates the code so a harmless change breaks it or repeats another test, or where shipped code behaves differently under test", false},
}
