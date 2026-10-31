package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type Runner struct {
	ID       string
	Label    string
	Command  []string
	Path     string
	Override bool
	Reviewer ReviewerCapabilities
}

type ReviewerCapabilities struct {
	ReasoningValues   []string
	ReasoningHelp     string
	ModelHelp         string
	ReasoningEditable bool
}

type reviewerOverridePolicy struct {
	Flags         []string
	Commands      []string
	ValueFlags    []string
	ConfigKeys    []string
	PathFlags     []string
	PathListFlags []string
}

type runnerSpec struct {
	ID      string
	Label   string
	Command []string
	// Resume turns a fresh argv into one continuing the last conversation.
	Resume func(argv []string) []string
	// Prompt appends the opening message the way this runner accepts it.
	Prompt func(argv []string, message string) []string
	// MCP points the runner at a qrouton MCP server, which is the part of the
	// wiring an eval stands its own binary into.
	MCP func(bin string, args []string) (MCPWiring, error)
	// Inject adds MCP and hook configuration, and answers the environment the
	// runner is launched with.
	Inject func(argv []string, c injectContext) (outArgv, env []string, err error)
	// Hooks are the agent-event hooks Inject wires, which is everything the
	// workbench can learn about the runner's agents.
	Hooks             []string
	Reviewer          ReviewerCapabilities
	ReviewOverrides   reviewerOverridePolicy
	ValidateModel     func(string) bool
	ReviewArgs        func([]string, config.Reviewer, string) []string
	ReviewOpeningRole bool
	ReviewCwd         func(string, string) string
}

type injectContext struct {
	qroutonBin  string
	dir         string
	handle      workbench.Handle
	mcpArgs     []string
	generation  uint64
	reviewer    bool
	reviewHome  string
	reviewRunID string
	reviewRole  string
	// override is true when the user replaced this runner's command, which
	// OpenCode reads before it writes a permission default over their config.
	override bool
}

var claudeHooks = []string{
	agentevent.HookSubagentStart, agentevent.HookSubagentStop,
	agentevent.HookNotification, agentevent.HookStop, agentevent.HookStopFailure,
	agentevent.HookPreToolUse, agentevent.HookPostToolUse, agentevent.HookPostToolUseFailure,
	agentevent.HookUserPromptSubmit,
}

var runnerSpecs = []runnerSpec{
	{
		ID: runnerIDClaude, Label: runnerLabelClaude,
		Reviewer:        ReviewerCapabilities{ReasoningValues: claudeReviewerEfforts, ReasoningHelp: reviewerDefaultReasoningHelp, ModelHelp: reviewerModelHelp},
		ReviewOverrides: claudeReviewerOverrides,
		Command:         []string{runnerIDClaude, claudeSkipPermissionsFlag},
		Resume:          resumeWith(claudeContinueFlag),
		Prompt:          promptAsArgument,
		MCP:             claudeMCP,
		Inject:          injectClaude,
		Hooks:           claudeHooks,
		ReviewArgs:      reviewClaudeArgs,
		ReviewCwd:       sessionpaths.ReviewRun,
	},
	{
		ID: runnerIDCodex, Label: runnerLabelCodex,
		Reviewer:        ReviewerCapabilities{ReasoningEditable: true, ReasoningHelp: codexReviewerReasoningHelp, ModelHelp: reviewerModelHelp},
		ReviewOverrides: codexReviewerOverrides,
		Command:         []string{runnerIDCodex, codexBypassSandboxFlag},
		Resume:          resumeWith(codexResumeCmd, codexResumeLast),
		Prompt:          promptAsArgument,
		MCP:             codexMCP,
		Inject:          injectCodex,
		ReviewArgs:      reviewCodexArgs,
		ReviewCwd:       sessionpaths.ReviewRun,
		Hooks:           []string{agentevent.HookSubagentStart, agentevent.HookSubagentStop},
	},
	{
		ID: runnerIDOpenCode, Label: runnerLabelOpenCode,
		Reviewer:        ReviewerCapabilities{ReasoningHelp: openCodeReviewerReasoningHelp, ModelHelp: openCodeReviewerModelHelp},
		ReviewOverrides: openCodeReviewerOverrides,
		ValidateModel: func(model string) bool {
			provider, name, ok := strings.Cut(model, "/")
			return ok && strings.TrimSpace(provider) != "" && strings.TrimSpace(name) != ""
		},
		Command:    []string{runnerIDOpenCode, openCodeAutoFlag},
		Resume:     resumeWith(claudeContinueFlag),
		Prompt:     promptBehindFlag(openCodePromptFlag),
		MCP:        openCodeMCP,
		Inject:     injectOpenCode,
		ReviewArgs: reviewOpenCodeArgs,
		ReviewCwd:  reviewSessionRoot,
	},
	{
		ID: runnerIDAgy, Label: runnerLabelAgy,
		Reviewer:          ReviewerCapabilities{ReasoningValues: agyReviewerEfforts, ReasoningHelp: reviewerDefaultReasoningHelp, ModelHelp: reviewerModelHelp},
		ReviewOverrides:   agyReviewerOverrides,
		Command:           []string{runnerIDAgy, agySkipPermissionsFlag},
		Resume:            resumeWith(agyContinueFlag),
		Prompt:            promptBehindFlag(agyPromptInteractiveFlag),
		MCP:               agyMCP,
		Inject:            injectAgy,
		ReviewArgs:        reviewAgyArgs,
		ReviewOpeningRole: true,
		ReviewCwd:         reviewSessionRoot,
	},
}

func reviewSessionRoot(root, _ string) string { return root }

var builtinRunners = builtins()

func builtins() []Runner {
	out := make([]Runner, len(runnerSpecs))
	for i, spec := range runnerSpecs {
		capabilities := spec.Reviewer
		capabilities.ReasoningValues = slices.Clone(capabilities.ReasoningValues)
		out[i] = Runner{ID: spec.ID, Label: spec.Label, Command: slices.Clone(spec.Command), Reviewer: capabilities}
	}
	return out
}

// RunnerHooks are the agent-event hooks a runner is launched with; an unknown
// runner has none.
func RunnerHooks(id string) []string {
	spec, _ := specFor(id)
	return slices.Clone(spec.Hooks)
}

// RunnerLabel is what a runner is called on screen, and empty for one qrouton
// does not launch.
func RunnerLabel(id string) string {
	spec, _ := specFor(id)
	return spec.Label
}

func specFor(id string) (runnerSpec, bool) {
	for _, spec := range runnerSpecs {
		if spec.ID == id {
			return spec, true
		}
	}
	return runnerSpec{}, false
}

func resumeWith(flags ...string) func([]string) []string {
	return func(argv []string) []string { return append(argv, flags...) }
}

func promptAsArgument(argv []string, message string) []string {
	return append(argv, message)
}

func promptBehindFlag(flag string) func([]string, string) []string {
	return func(argv []string, message string) []string { return append(argv, flag, message) }
}

var findExecutable = exec.LookPath

// Runners applies configured overrides to qrouton's supported runners and
// reports which are installed. An override naming something qrouton cannot wire
// up is an error rather than a no-op: MCP and hook injection is per-runner, so
// an unrecognised command would launch without any of it.
func Runners(cfg *config.Config) ([]Runner, error) {
	out := make([]Runner, len(builtinRunners))
	copy(out, builtinRunners)
	byID := make(map[string]int, len(out))
	for i := range out {
		out[i].Command = slices.Clone(out[i].Command)
		out[i].Reviewer.ReasoningValues = slices.Clone(out[i].Reviewer.ReasoningValues)
		byID[out[i].ID] = i
	}
	for id, command := range cfg.Launch {
		i, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("%w: %q (supported: %s)", ErrUnsupportedOverride, id, strings.Join(builtinIDs(), ", "))
		}
		// An empty override would otherwise leave Command[0] to be looked up as
		// "", reporting the runner as simply not installed.
		if len(command) == 0 {
			return nil, fmt.Errorf("%w: %q", ErrEmptyOverride, id)
		}
		out[i].Command = append([]string(nil), command...)
		out[i].Override = true
	}
	for i := range out {
		out[i].Path, _ = findExecutable(out[i].Command[0])
	}
	return out, nil
}

func ValidateReviewer(reviewer *config.Reviewer, overrides map[string][]string) error {
	if reviewer == nil {
		return nil
	}
	id := strings.TrimSpace(reviewer.Harness)
	spec, ok := specFor(id)
	if !ok {
		return fmt.Errorf(reviewerHarnessErrorFormat, ErrInvalidReviewer, reviewer.Harness, strings.Join(builtinIDs(), ", "))
	}
	if strings.TrimSpace(reviewer.Model) == "" || !validReviewerText(reviewer.Model) {
		return fmt.Errorf(reviewerModelErrorFormat, ErrInvalidReviewer)
	}
	if spec.ValidateModel != nil && !spec.ValidateModel(strings.TrimSpace(reviewer.Model)) {
		return fmt.Errorf(reviewerModelCapabilityErrorFormat, ErrInvalidReviewer, id, spec.Reviewer.ModelHelp)
	}
	if !validReviewerText(reviewer.Reasoning) {
		return fmt.Errorf(reviewerReasoningEncodingErrorFormat, ErrInvalidReviewer)
	}
	reasoning := strings.TrimSpace(reviewer.Reasoning)
	if reasoning != "" && !spec.Reviewer.ReasoningEditable && !slices.Contains(spec.Reviewer.ReasoningValues, reasoning) {
		return fmt.Errorf(reviewerReasoningErrorFormat, ErrInvalidReviewer, id, spec.Reviewer.ReasoningHelp)
	}
	runners, err := Runners(&config.Config{Launch: overrides})
	if err != nil {
		return err
	}
	selected := slices.IndexFunc(runners, func(runner Runner) bool { return runner.ID == id })
	if selected < 0 || runners[selected].Path == "" {
		return fmt.Errorf("%w: %q", ErrRunnerUnavailable, id)
	}
	runner := runners[selected]
	if argument := conflictingReviewerArgument(runner.Command, spec.ReviewOverrides); argument != "" {
		return fmt.Errorf(reviewerOverrideErrorFormat, ErrReviewerOverrideConflict, id, argument)
	}
	return nil
}

func validReviewerText(value string) bool {
	return utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func conflictingReviewerArgument(argv []string, policy reviewerOverridePolicy) string {
	for i := 1; i < len(argv); i++ {
		argument := argv[i]
		flag, value, inline := strings.Cut(argument, "=")
		if slices.Contains(policy.Flags, flag) || slices.Contains(policy.Commands, argument) {
			return argument
		}
		for _, short := range policy.Flags {
			if len(short) == 2 && !strings.HasPrefix(short, flagPrefix) && strings.HasPrefix(argument, short) {
				return argument
			}
		}
		if len(policy.ConfigKeys) > 0 && (flag == codex.ConfigFlag || flag == codexLongConfigFlag || strings.HasPrefix(argument, codex.ConfigFlag) && !strings.HasPrefix(argument, flagPrefix)) {
			if flag != codex.ConfigFlag && flag != codexLongConfigFlag {
				value = strings.TrimPrefix(argument, codex.ConfigFlag)
			} else if !inline && i+1 < len(argv) {
				i++
				value = argv[i]
			}
			key, _, _ := strings.Cut(value, "=")
			key = strings.Trim(strings.TrimSpace(key), `"'`)
			if slices.Contains(policy.ConfigKeys, key) {
				return argument
			}
			continue
		}
		if !inline && slices.Contains(policy.ValueFlags, flag) && i+1 < len(argv) {
			i++
		}
	}
	return ""
}

func builtinIDs() []string {
	ids := make([]string, len(builtinRunners))
	for i, runner := range builtinRunners {
		ids[i] = runner.ID
	}
	return ids
}

// ByID returns the installed runner matching id, which may be a runner
// identifier ("claude") or the command qrouton would run ("claude", or a path
// to it). An uninstalled or unknown runner is an error: the caller asked for
// something specific, so silently substituting another would be wrong.
func ByID(cfg *config.Config, id string) (Runner, error) {
	runners, err := Runners(cfg)
	if err != nil {
		return Runner{}, err
	}
	for _, runner := range runners {
		if runner.Path == "" {
			continue
		}
		if runner.ID == id || runner.Command[0] == id || filepath.Base(runner.Command[0]) == id {
			return runner, nil
		}
	}
	return Runner{}, fmt.Errorf("%w: %q", ErrRunnerUnavailable, id)
}

func FirstInstalled(cfg *config.Config) (Runner, error) {
	runners, err := Runners(cfg)
	if err != nil {
		return Runner{}, err
	}
	for _, runner := range runners {
		if runner.Path != "" {
			return runner, nil
		}
	}
	return Runner{}, ErrNoRunnerInstalled
}

func runnerLaunch(r Runner, qroutonBin, dir string, editor EditorCommand, handle workbench.Handle, generation uint64, resume bool, prompt string) ([]string, []string, error) {
	// Runner's fields are exported, so a hand-built one can still reach here
	// without the per-runner wiring the launch path needs.
	spec, ok := specFor(r.ID)
	if !ok {
		return nil, nil, fmt.Errorf("%w: %q", ErrUnsupportedRunner, r.ID)
	}
	return spec.Inject(runnerArgv(spec, r, resume, sessionMode(dir), sessionTicket(dir), prompt), injectContext{
		qroutonBin: qroutonBin,
		dir:        dir,
		handle:     handle,
		generation: generation,
		mcpArgs: []string{MCPSubcommand, sessionRootFlag, dir,
			editorJSONFlag, editor.Marshal(), workbenchJSONFlag, handle.Marshal()},
		override: r.Override,
	})
}

func injectClaude(argv []string, c injectContext) ([]string, []string, error) {
	mcp, err := claudeMCP(c.qroutonBin, c.mcpArgs)
	if err != nil {
		return nil, nil, err
	}
	if name := sessionName(c.dir); name != "" && !c.reviewer {
		argv = append(argv, claudeNameFlag, name)
	}
	argv = append(argv, mcp.Args...)
	if c.reviewer {
		return argv, workbench.WithEnv(reviewerEnv(), claudeMaintainProjectWorkingDirEnvVar, claudeMaintainProjectWorkingDirValue), nil
	}
	hookCommand := ShellQuote(c.qroutonBin) + " " + AgentEventSubcommand +
		" " + workbenchJSONFlag + " " + ShellQuote(c.handle.Marshal()) +
		" " + generationFlag + " " + fmt.Sprint(c.generation) +
		" " + providerFlag + " " + runnerIDClaude
	// Async, so a tool call never waits on the workbench; the tracker accepts
	// events in any order.
	hooks := map[string]any{}
	for _, hook := range claudeHooks {
		hooks[hook] = commandHook(true, hookCommand)
	}
	// Strings and maps of them: marshalling cannot fail.
	settings, _ := json.Marshal(map[string]any{claudeHooksKey: hooks})
	env := workbench.WithEnv(os.Environ(), claudeMaintainProjectWorkingDirEnvVar,
		claudeMaintainProjectWorkingDirValue)
	return append(argv, claudeSettingsFlag, string(settings)), env, nil
}

func injectCodex(argv []string, c injectContext) ([]string, []string, error) {
	mcp, err := codexMCP(c.qroutonBin, c.mcpArgs)
	if err != nil {
		return nil, nil, err
	}
	argv = append(argv, mcp.Args...)
	if c.reviewer {
		return argv, reviewerEnv(), nil
	}
	// Codex defaults to one level of nesting, which is a lead that cannot spawn
	// the specialists qrouton's topology has it delegate to. Raise it, unless
	// the user's own command already asks for at least as much.
	if codex.MaxDepth(argv) < codex.RequiredMaxDepth {
		argv = append(argv, codex.ConfigFlag, codex.MaxDepthSetting(codex.RequiredMaxDepth))
	}
	hookCommand := fmt.Sprintf(codexAgentEventCommandFormat, agentevent.QroutonBinEnvVar, AgentEventSubcommand)
	hook := fmt.Sprintf(codexCommandHookFormat, quotedConfigString(hookCommand), codexHookTimeoutSeconds)
	if !c.override {
		argv = append(argv, codexBypassHookTrustFlag)
	}
	argv = append(argv,
		codex.ConfigFlag, codexSubagentStartHook+hook,
		codex.ConfigFlag, codexSubagentStopHook+hook,
	)
	return argv, agentEventEnv(c, runnerIDCodex), nil
}

func quotedConfigString(value string) string {
	quoted, _ := json.Marshal(value) // marshalling a string cannot fail
	return string(quoted)
}

func agentEventEnv(c injectContext, provider string) []string {
	env := os.Environ()
	env = workbench.WithEnv(env, agentevent.QroutonBinEnvVar, c.qroutonBin)
	env = workbench.WithEnv(env, agentevent.WorkbenchEnvVar, c.handle.Marshal())
	env = workbench.WithEnv(env, agentevent.GenerationEnvVar, fmt.Sprint(c.generation))
	env = workbench.WithEnv(env, agentevent.ProviderEnvVar, provider)
	return env
}

// injectOpenCode is the one runner configured through the environment rather
// than argv, and the one qrouton has to merge into a config the user may
// already be passing.
func injectOpenCode(argv []string, c injectContext) ([]string, []string, error) {
	var extra map[string]any
	if !c.override {
		extra = map[string]any{openCodePermissionKey: openCodeAllowValue}
	}
	config, err := openCodeConfig(c.qroutonBin, c.mcpArgs, extra)
	if err != nil {
		return nil, nil, err
	}
	env := os.Environ()
	if c.reviewer {
		var content map[string]any
		if err := json.Unmarshal([]byte(config), &content); err != nil {
			return nil, nil, err
		}
		agents, _ := content[openCodeAgentsKey].(map[string]any)
		if agents == nil {
			agents = map[string]any{}
		}
		agents[reviewerAgentName] = map[string]any{openCodeModeKey: openCodePrimaryValue, openCodeAgentPromptKey: c.reviewRole}
		content[openCodeAgentsKey] = agents
		raw, err := json.Marshal(content)
		if err != nil {
			return nil, nil, err
		}
		config = string(raw)
		env = reviewerEnv()
		env = workbench.WithEnv(env, openCodeDatabaseEnvVar, sessionpaths.ReviewOpenCodeDatabase(c.dir, c.reviewRunID))
	}
	return argv, workbench.WithEnv(env, openCodeConfigEnvVar, config), nil
}

// injectAgy moves agy's whole configuration root into the session, because a
// session-scoped MCP server can be declared nowhere else. Only the server
// document is generated; the rest of the root links back to the user's own, so
// their model, their history and their trusted workspaces still apply.
func injectAgy(argv []string, c injectContext) ([]string, []string, error) {
	home := sessionpaths.AgyHome(c.dir)
	if c.reviewer {
		home = c.reviewHome
	}
	config := filepath.Join(home, agyMCPConfigPath())
	if err := os.MkdirAll(filepath.Dir(config), dirMode); err != nil {
		return nil, nil, err
	}
	if err := linkGeminiRoot(home, c.reviewer); err != nil {
		return nil, nil, err
	}
	content, err := agyMCPConfig(config, c.qroutonBin, c.mcpArgs)
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(config, []byte(content), configMode); err != nil {
		return nil, nil, err
	}
	env := os.Environ()
	if c.reviewer {
		env = reviewerEnv()
	}
	return append(argv, agyGeminiDirFlag, home), env, nil
}

// linkGeminiRoot links the user's .gemini tree into the session's, entry by
// entry, so the one file qrouton generates is the only thing that differs.
func linkGeminiRoot(home string, reviewer bool) error {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	root := filepath.Join(userHome, geminiDirName)
	// The config directory is shadowed rather than linked: qrouton owns a file
	// inside it, and a link would put that file in the user's tree.
	generated := []string{agyConfigDirName}
	if reviewer {
		generated = append(generated, agyCLIStateDirName)
	}
	if err := linkEntries(root, home, generated...); err != nil {
		return err
	}
	if reviewer {
		source, destination := filepath.Join(root, agyCLIStateDirName), filepath.Join(home, agyCLIStateDirName)
		if err := os.MkdirAll(destination, reviewPrivateMode); err != nil {
			return err
		}
		for _, name := range agySharedReviewerFiles {
			target := filepath.Join(source, name)
			if _, err := os.Stat(target); errors.Is(err, fs.ErrNotExist) {
				continue
			} else if err != nil {
				return err
			}
			if err := os.Symlink(target, filepath.Join(destination, name)); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
		}
	}
	return linkEntries(filepath.Join(root, agyConfigDirName),
		filepath.Join(home, agyConfigDirName), agyMCPConfigName)
}

// linkEntries symlinks every entry of source into destination, skipping the
// names qrouton generates. A missing source is a machine agy has not run on.
// Whatever is already in destination for real is left alone: replacing it with
// a link would discard what agy put there.
func linkEntries(source, destination string, generated ...string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if slices.Contains(generated, entry.Name()) {
			continue
		}
		link, target := filepath.Join(destination, entry.Name()), filepath.Join(source, entry.Name())
		if info, err := os.Lstat(link); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if current, err := os.Readlink(link); err == nil && current == target {
				continue
			}
			if err := os.Remove(link); err != nil {
				return err
			}
		}
		if err := os.Symlink(target, link); err != nil {
			return err
		}
	}
	return nil
}

// MCPWiring is how one runner is told about a qrouton MCP server: arguments for
// its command line, environment entries for the runner configured that way, and
// a document for the runner that reads one off disk.
type MCPWiring struct {
	Args []string
	Env  []string
	File MCPFile
}

// MCPFile is a configuration document a runner reads from its own configuration
// root. Path is relative to that root, which the launch path moves per session.
type MCPFile struct {
	Path    string
	Content string
}

// RunnerMCPWiring points runner id at bin, invoked with args, as its qrouton MCP
// server. An eval stands its own binary and mock arguments in here, so a graded
// run reaches the tool surface the way a launched one does.
func RunnerMCPWiring(id, bin string, args []string) (MCPWiring, error) {
	spec, ok := specFor(id)
	if !ok {
		return MCPWiring{}, fmt.Errorf("%w: %q", ErrUnsupportedRunner, id)
	}
	return spec.MCP(bin, args)
}

func claudeMCP(bin string, args []string) (MCPWiring, error) {
	config, _ := json.Marshal(map[string]any{claudeMCPServersKey: map[string]any{serverName: map[string]any{
		claudeTypeKey: claudeStdioType, claudeCommandKey: bin, claudeArgsKey: args}}})
	return MCPWiring{Args: []string{claudeMCPConfigFlag, string(config)}}, nil
}

func codexMCP(bin string, args []string) (MCPWiring, error) {
	command, _ := json.Marshal(bin)
	encoded, _ := json.Marshal(args)
	return MCPWiring{Args: []string{
		codex.ConfigFlag, codexMCPCommandKey + string(command),
		codex.ConfigFlag, codexMCPArgsKey + string(encoded)}}, nil
}

func openCodeMCP(bin string, args []string) (MCPWiring, error) {
	config, err := openCodeConfig(bin, args, nil)
	if err != nil {
		return MCPWiring{}, err
	}
	return MCPWiring{Env: workbench.WithEnv(nil, openCodeConfigEnvVar, config)}, nil
}

// openCodeConfig merges the qrouton MCP server, and whatever else the launch
// asks for, into the config OpenCode is already being passed.
func openCodeConfig(bin string, args []string, extra map[string]any) (string, error) {
	content := map[string]any{}
	if existing := os.Getenv(openCodeConfigEnvVar); existing != "" {
		if err := json.Unmarshal([]byte(existing), &content); err != nil {
			return "", fmt.Errorf("%s: %w", openCodeConfigEnvVar, err)
		}
	}
	servers, _ := content[openCodeMCPKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[serverName] = map[string]any{
		claudeTypeKey: openCodeLocalType, claudeCommandKey: append([]string{bin}, args...), openCodeEnabledKey: true}
	content[openCodeMCPKey] = servers
	for key, value := range extra {
		content[key] = value
	}
	b, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// agyMCPConfigPath is the only place agy looks for MCP servers, relative to the
// configuration root --gemini_dir moves.
func agyMCPConfigPath() string { return filepath.Join(agyConfigDirName, agyMCPConfigName) }

func agyMCP(bin string, args []string) (MCPWiring, error) {
	content, err := agyMCPConfig("", bin, args)
	if err != nil {
		return MCPWiring{}, err
	}
	return MCPWiring{File: MCPFile{Path: agyMCPConfigPath(), Content: content}}, nil
}

// agyMCPConfig merges the qrouton server into the document at existing, so
// servers a previous launch wrote or the user added by hand survive. agy reads
// the file as HuJSON, so a hand-edited one can carry comments encoding/json
// refuses; failing beats writing a document their servers are missing from.
func agyMCPConfig(existing, bin string, args []string) (string, error) {
	content := map[string]any{}
	if existing != "" {
		raw, err := os.ReadFile(existing)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if trimmed := strings.TrimSpace(string(raw)); trimmed != "" {
			if err := json.Unmarshal([]byte(trimmed), &content); err != nil {
				return "", fmt.Errorf("%s: %w", existing, err)
			}
		}
	}
	servers, _ := content[claudeMCPServersKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[serverName] = map[string]any{
		claudeTypeKey: claudeStdioType, claudeCommandKey: bin, claudeArgsKey: args}
	content[claudeMCPServersKey] = servers
	b, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func commandHook(async bool, commands ...string) []map[string]any {
	entries := make([]map[string]any, len(commands))
	for i, command := range commands {
		entries[i] = map[string]any{claudeTypeKey: claudeCommandType, claudeCommandKey: command}
		if async {
			entries[i][claudeAsyncKey] = true
		}
	}
	return []map[string]any{{claudeHooksKey: entries}}
}

// ShellQuote single-quotes s so it survives as one word in the POSIX shell that
// runs hook commands and window commands. Go's %q double-quoting would leave $,
// backticks, and backslashes live for the shell.
func ShellQuote(s string) string {
	return shellQuoteChar + strings.ReplaceAll(s, shellQuoteChar, shellQuoteEscape) + shellQuoteChar
}

func runnerArgv(spec runnerSpec, r Runner, resume bool, mode, provider, prompt string) []string {
	argv := slices.Clone(r.Command)
	if resume {
		argv = spec.Resume(argv)
		if resumed := strings.TrimSpace(prompt); resumed != "" {
			argv = spec.Prompt(argv, resumed)
		}
		return argv
	}
	return spec.Prompt(argv, openingMessage(mode, provider, prompt))
}

// An external request is attributed to the provider it came from, and to
// nothing when no provider claims it.
func openingMessage(mode, provider, initialPrompt string) string {
	message := openingMessageRPI
	if mode == string(session.ModeAssistant) {
		message = openingMessageAssistant
	}
	prompt := strings.TrimSpace(initialPrompt)
	if prompt == "" {
		return message
	}
	if provider == "" {
		return message + requestSeparator + prompt
	}
	return message + fmt.Sprintf(providerRequestSeparator, provider) + prompt
}

func reviewClaudeArgs(argv []string, settings config.Reviewer, role string) []string {
	argv = append(argv, reviewerModelFlag, settings.Model, claudeAppendSystemFlag, role)
	if settings.Reasoning != "" {
		argv = append(argv, reviewerEffortFlag, settings.Reasoning)
	}
	return argv
}

func reviewCodexArgs(argv []string, settings config.Reviewer, role string) []string {
	argv = append(argv, reviewerModelFlag, settings.Model, codex.ConfigFlag, codexDeveloperInstructionsKey+quotedConfigString(role))
	if settings.Reasoning != "" {
		argv = append(argv, codex.ConfigFlag, codexReasoningKey+quotedConfigString(settings.Reasoning))
	}
	return argv
}

func reviewOpenCodeArgs(argv []string, settings config.Reviewer, _ string) []string {
	return append(argv, reviewerModelFlag, settings.Model, reviewerAgentFlag, reviewerAgentName)
}

func reviewAgyArgs(argv []string, settings config.Reviewer, _ string) []string {
	argv = append(argv, reviewerModelFlag, settings.Model)
	if settings.Reasoning != "" {
		argv = append(argv, reviewerEffortFlag, settings.Reasoning)
	}
	return argv
}

func reviewerEnv() []string {
	env := os.Environ()
	for _, key := range []string{agentevent.QroutonBinEnvVar, agentevent.WorkbenchEnvVar, agentevent.GenerationEnvVar, agentevent.ProviderEnvVar} {
		env = workbench.WithoutEnv(env, key)
	}
	return env
}
