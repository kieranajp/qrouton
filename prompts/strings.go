package prompts

// Provider-specific rendering details: where each runner discovers agent
// definitions, and the TOML shape Codex expects.

const (
	claudeAgentsDir = ".claude/agents/"
	codexAgentsDir  = ".codex/agents/"

	frontmatterKeySep         = ":"
	frontmatterNameKey        = "name"
	frontmatterDescriptionKey = "description"
	subagentChoiceFileName    = "subagent-choice.md"
	subagentChoicePlaceholder = "{{subagent-choice}}"

	codexNameFormat        = "name = %s\ndescription = %s\n"
	codexSandboxFormat     = "sandbox_mode = %s\n"
	codexInstructionsOpen  = "developer_instructions = \"\"\"\n"
	codexInstructionsClose = "\n\"\"\"\n"

	tomlTripleQuote        = `"""`
	tomlEscapedTripleQuote = `\"\"\"`

	// Codex sandbox modes. Research and review agents only read; the leads that
	// write artifacts need the workspace.
	sandboxReadOnly       = "read-only"
	sandboxWorkspaceWrite = "workspace-write"
)

// Which agents get which sandbox. An agent absent from both sets inherits
// Codex's default.
var (
	readOnlyAgents = map[string]bool{
		"code-reviewer":       true,
		"codebase-researcher": true,
		"external-researcher": true,
		"pattern-finder":      true,
		"qrouton-researcher":  true,
		"thoughts-researcher": true,
	}

	workspaceWriteAgents = map[string]bool{
		"qrouton-research-lead": true,
		"test-verifier":         true,
	}
)
