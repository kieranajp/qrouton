package prompts

const (
	claudeAgentsDir = ".claude/agents/"
	codexAgentsDir  = ".codex/agents/"
	// agy discovers an agent as a directory holding agent.md, so its name is a
	// path segment rather than a file stem.
	agyAgentsDir     = ".agents/agents/"
	agyAgentFileName = "agent.md"

	frontmatterKeySep         = ":"
	frontmatterNameKey        = "name"
	frontmatterDescriptionKey = "description"

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
}

var (
	readOnlyAgents = map[string]bool{
		"code-reviewer":       true,
		"codebase-researcher": true,
		"external-researcher": true,
		"pattern-finder":      true,
		"thoughts-researcher": true,
	}

	workspaceWriteAgents = map[string]bool{
		"qrouton-research-lead":       true,
		"qrouton-planning-lead":       true,
		"qrouton-implementation-lead": true,
		"test-verifier":               true,
	}
)
