package evalharness

// Literal values the harness matches on or shells out to. Runner names, check
// kinds, and manifest roles are contract with the scenario files and the
// session manifest, so they live here rather than inline at each use.

const (
	gitBin     = "git"
	srcDirName = "src"

	manifestName = "qrouton.json"

	// Git subcommands and arguments the harness runs against fixture repos.
	gitInitCmd     = "init"
	gitConfigCmd   = "config"
	gitAddCmd      = "add"
	gitCommitCmd   = "commit"
	gitStatusCmd   = "status"
	gitRevParseCmd = "rev-parse"
	gitDiffCmd     = "diff"
	gitLsFilesCmd  = "ls-files"

	gitDirFlag        = "-C"
	gitPorcelainFlag  = "--porcelain"
	gitQuietFlag      = "-q"
	gitQuietMsgFlag   = "-qm"
	gitNoExtDiffFlag  = "--no-ext-diff"
	gitOthersFlag     = "--others"
	gitExcludeStdFlag = "--exclude-standard"
	gitAllPathspec    = "."
	gitHeadRef        = "HEAD"

	gitUserEmailKey = "user.email"
	gitUserNameKey  = "user.name"

	roleReference = "reference"

	// Runner names, as scenarios and the --runner flag spell them.
	runnerClaude = "claude"
	runnerCodex  = "codex"
	runnerAll    = "all"

	// Scenario selection: either of these runs every scenario.
	scenarioAll = "all"

	// Directory layout of the eval tree, relative to the repo root.
	evalDirName      = "eval"
	scenariosDirName = "scenarios"

	// Judge modes recorded in report metadata.
	judgeModePairwise = "pairwise"
	judgeModeNone     = "none"

	// The fixture baseline commit every eval repo starts from.
	fixtureCommitMessage = "fixture baseline"
	fixtureUserEmail     = "eval@qrouton.local"
	fixtureUserName      = "qrouton eval"
)

// Check kinds a scenario may declare. Each maps to one grader in gradeCheck.
const (
	checkArtifactExists   = "artifact_exists"
	checkArtifactAbsent   = "artifact_absent"
	checkArtifactContains = "artifact_contains"
	checkArtifactExcludes = "artifact_excludes"
	checkResponseContains = "response_contains"
	checkResponseExcludes = "response_excludes"
	checkResearchPair     = "research_pair"
	checkSentinelSafe     = "sentinel_safe"
	checkOpenFile         = "open_file"
	checkDelegation       = "delegation"
	checkRepoChanged      = "repo_changed"
	checkRepoUnchanged    = "repo_unchanged"
	checkTestsPass        = "tests_pass"
)

// Assertion names, as they appear in the report.
const (
	assertNoInternalLeak   = "no internal workflow terminology in final response"
	assertReferencesClean  = "reference repositories unchanged"
	assertResponseContains = "response contains "
	assertResponseExcludes = "response excludes internal terms"
	assertArtifactsExclude = "artifacts exclude sentinel"
	assertArtifactContains = "artifact contains progress: "
	assertArtifactExists   = "artifact exists: "
	assertArtifactAbsent   = "artifact absent: "
	assertResearchPair     = "paired research questions and findings"
	assertSentinelSafe     = "ticket sentinel absent from research briefs and artifacts"
	assertOpenFile         = "completed document presented with open_file"
	assertDelegatedTo      = "delegated to "
	assertRepoChanged      = "repository changed: "
	assertRepoUnchanged    = "repository unchanged: "
	assertTestsPass        = "tests pass: "
	assertUnknownCheck     = "unknown check: "

	evidenceUnsupportedCheck = "unsupported check kind"
	evidenceNoTestManifest   = "no supported test manifest"
	evidenceCollaboration    = "collaboration=%t target=%t"
	evidenceTimeoutFormat    = "test run exceeded %s: %s"
)

// Test commands testsPass runs, chosen by the manifest a fixture repo carries.
var (
	goTestCommand  = []string{"go", "test", "./..."}
	npmTestCommand = []string{"npm", "test", "--", "--runInBand"}
)

const (
	goModFile       = "go.mod"
	packageJSONFile = "package.json"

	// researchQuestionsSuffix pairs a questions document with its findings.
	researchQuestionsSuffix = "-questions"
	markdownExt             = ".md"

	researchPathSegment = "/research/"

	// Event markers the delegation and brief graders match on.
	subagentTypeKey  = `"subagent_type"`
	taskNameKey      = `"task_name"`
	spawnAgentMarker = "spawn_agent"
	collabToolCall   = `"type":"collab_tool_call"`
	initSubtype      = `"subtype":"init"`
	delegationKind   = "delegation"

	evidenceJoiner = ", "

	thoughtsDirName = "thoughts"
	sharedDirName   = "shared"

	// diffFailedFormat records a failed diff as the diff itself, so a broken
	// repository trips repo_unchanged instead of passing as "no changes".
	diffFailedFormat = "(git diff failed: %v)\n%s"
	untrackedFormat  = "\nUntracked files:\n%s\n"
)

// Normalized event kinds. Provider streams are mapped onto this small set so a
// grader can match on intent rather than on a provider's own event names.
const (
	kindToolCall      = "tool_call"
	kindAssistant     = "assistant"
	kindResult        = "result"
	kindProviderEvent = "provider_event"
)

// Keys the normalizer reads from a provider event, in preference order.
var (
	eventTypeKeys = []string{"type", "event", "kind"}
	sessionIDKeys = []string{"session_id", "thread_id"}
	toolNameKeys  = []string{"tool_name", "name"}

	// textKeys locate an event's human-visible text, outermost first.
	textKeys = []string{"result", "final_output", "text", "content", "message", "item"}
)

const (
	// Hidden reasoning is stripped: the harness grades what a user could see.
	hiddenThinking  = "thinking"
	hiddenReasoning = "reasoning"

	itemKey = "item"

	// Provider streams can carry large instruction payloads.
	streamBufferInitial = 64 * 1024
	streamBufferMax     = 4 * 1024 * 1024

	scenarioGlob = "*.json"

	outputDirMode  = 0o755
	outputFileMode = 0o644

	warnPairwiseNeedsBoth = "pairwise judging requires --runner all"
)

// Keys in the report's invocation metadata.
const (
	metaRunnerKey    = "runner"
	metaScenarioKey  = "scenario"
	metaSamplesKey   = "samples"
	metaAssetsDirKey = "assets_dir"
	metaNoJudgeKey   = "no_judge"
	metaJudgeModeKey = "judge_mode"
	metaTimeoutKey   = "timeout"
)

// The mock MCP server the harness points each runner at, so a graded run
// exercises the qrouton tool surface without a real multiplexer.
const (
	mockMCPSubcommand = "mock-mcp"
	mockMCPLogFlag    = "--log"
	mockMCPRootFlag   = "--root"

	mcpServerName = "qrouton"
	mcpServersKey = "mcpServers"
	mcpTypeKey    = "type"
	mcpCommandKey = "command"
	mcpArgsKey    = "args"
	mcpStdioType  = "stdio"

	codexMCPCommandKey = "mcp_servers.qrouton.command="
	codexMCPArgsKey    = "mcp_servers.qrouton.args="
)

// Claude and Codex invocation flags. Both are run non-interactively, streaming
// JSON, with sandboxing off — the workspace is already a throwaway fixture.
var (
	claudeBaseArgs = []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--setting-sources", "project",
		"--strict-mcp-config",
	}

	codexBaseArgs = []string{
		"--json",
		"--ephemeral",
		"--ignore-user-config",
		"--enable", "multi_agent",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
	}
)

const (
	claudeMCPConfigFlag = "--mcp-config"
	claudeResumeFlag    = "--resume"
	codexExecCmd        = "exec"
	codexResumeCmd      = "resume"
	codexConfigFlag     = "-c"
	modelFlag           = "--model"
	versionFlag         = "--version"

	// Event kinds the harness synthesises itself, rather than reading from a
	// provider stream.
	kindDuration = "duration"
	kindUser     = "user"
	roleUser     = "user"

	modelField = "model"

	// Output layout under the report directory.
	casesDirName     = "cases"
	workspaceDirName = "workspace"
	fixturesDirName  = "fixtures"
	diffsDirName     = "diffs"
	diffFileExt      = ".diff"

	outcomeTie = "tie"
)
