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

	workspaceWindowsFileName    = "workspace-windows.md"
	workspaceWindowsPlaceholder = "{{workspace-windows}}"

	codexNameFormat        = "name = %s\ndescription = %s\n"
	codexSandboxFormat     = "sandbox_mode = %s\n"
	codexInstructionsOpen  = "developer_instructions = \"\"\"\n"
	codexInstructionsClose = "\n\"\"\"\n"

	tomlTripleQuote        = `"""`
	tomlEscapedTripleQuote = `\"\"\"`

	// Codex sandbox modes. The research and review specialists only read; the
	// research lead and test-verifier hold the workspace.
	sandboxReadOnly       = "read-only"
	sandboxWorkspaceWrite = "workspace-write"
)

// partials are the prose more than one prompt has to say, keyed by the
// placeholder that pulls each in. A partial holds no placeholder of its own, so
// one pass over this map expands them all.
var partials = map[string]string{
	subagentChoicePlaceholder:   subagentChoiceFileName,
	workspaceWindowsPlaceholder: workspaceWindowsFileName,
}

// Which agents get which sandbox. An agent absent from both sets inherits
// Codex's default, as the planning and implementation leads do.
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
