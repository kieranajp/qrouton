package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type Runner struct {
	ID       string
	Label    string
	Command  []string
	Path     string
	Override bool
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
}

type injectContext struct {
	qroutonBin string
	dir        string
	handle     workbench.Handle
	mcpArgs    []string
	generation uint64
	// override is true when the user replaced this runner's command, which
	// OpenCode reads before it writes a permission default over their config.
	override bool
}

var runnerSpecs = []runnerSpec{
	{
		ID: runnerIDClaude, Label: runnerLabelClaude,
		Command: []string{runnerIDClaude, claudeSkipPermissionsFlag},
		Resume:  resumeWith(claudeContinueFlag),
		Prompt:  promptAsArgument,
		MCP:     claudeMCP,
		Inject:  injectClaude,
	},
	{
		ID: runnerIDCodex, Label: runnerLabelCodex,
		Command: []string{runnerIDCodex, codexBypassSandboxFlag},
		Resume:  resumeWith(codexResumeCmd, codexResumeLast),
		Prompt:  promptAsArgument,
		MCP:     codexMCP,
		Inject:  injectCodex,
	},
	{
		ID: runnerIDOpenCode, Label: runnerLabelOpenCode,
		Command: []string{runnerIDOpenCode, openCodeAutoFlag},
		Resume:  resumeWith(claudeContinueFlag),
		Prompt:  promptBehindFlag(openCodePromptFlag),
		MCP:     openCodeMCP,
		Inject:  injectOpenCode,
	},
}

var builtinRunners = builtins()

func builtins() []Runner {
	out := make([]Runner, len(runnerSpecs))
	for i, spec := range runnerSpecs {
		out[i] = Runner{ID: spec.ID, Label: spec.Label, Command: slices.Clone(spec.Command)}
	}
	return out
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
	return spec.Inject(runnerArgv(spec, r, resume, sessionMode(dir), prompt), injectContext{
		qroutonBin: qroutonBin,
		dir:        dir,
		handle:     handle,
		generation: generation,
		mcpArgs: []string{mcpSubcommand, sessionRootFlag, dir,
			editorJSONFlag, editor.Marshal(), workbenchJSONFlag, handle.Marshal()},
		override: r.Override,
	})
}

func injectClaude(argv []string, c injectContext) ([]string, []string, error) {
	mcp, err := claudeMCP(c.qroutonBin, c.mcpArgs)
	if err != nil {
		return nil, nil, err
	}
	argv = append(argv, mcp.Args...)
	hookCommand := ShellQuote(c.qroutonBin) + " " + agentEventSubcommand +
		" " + workbenchJSONFlag + " " + ShellQuote(c.handle.Marshal()) +
		" " + generationFlag + " " + fmt.Sprint(c.generation) +
		" " + providerFlag + " " + runnerIDClaude
	// Chime only when the agent asks for attention, not on every turn, so the
	// user can step away.
	soundCommand := ShellQuote(sessionpaths.NotifyScript(c.dir))
	// Strings and maps of them: marshalling cannot fail.
	settings, _ := json.Marshal(map[string]any{claudeHooksKey: map[string]any{
		claudeSubagentStartHook: commandHook(hookCommand),
		claudeSubagentStopHook:  commandHook(hookCommand),
		claudeNotificationHook:  commandHook(soundCommand, hookCommand),
	}})
	return append(argv, claudeSettingsFlag, string(settings)), os.Environ(), nil
}

func injectCodex(argv []string, c injectContext) ([]string, []string, error) {
	mcp, err := codexMCP(c.qroutonBin, c.mcpArgs)
	if err != nil {
		return nil, nil, err
	}
	argv = append(argv, mcp.Args...)
	// Codex defaults to one level of nesting, which is a lead that cannot spawn
	// the specialists qrouton's topology has it delegate to. Raise it, unless
	// the user's own command already asks for at least as much.
	if codex.MaxDepth(argv) < codex.RequiredMaxDepth {
		argv = append(argv, codex.ConfigFlag, codex.MaxDepthSetting(codex.RequiredMaxDepth))
	}
	hookCommand := fmt.Sprintf(codexAgentEventCommandFormat, agentevent.QroutonBinEnvVar, agentEventSubcommand)
	hook := fmt.Sprintf(codexCommandHookFormat, quotedConfigString(hookCommand))
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
	return argv, workbench.WithEnv(os.Environ(), openCodeConfigEnvVar, config), nil
}

// MCPWiring is how one runner is told about a qrouton MCP server: arguments for
// its command line, and environment entries for the runner configured that way.
type MCPWiring struct {
	Args []string
	Env  []string
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
	// Strings and maps of them: marshalling cannot fail.
	config, _ := json.Marshal(map[string]any{claudeMCPServersKey: map[string]any{serverName: map[string]any{
		claudeTypeKey: claudeStdioType, claudeCommandKey: bin, claudeArgsKey: args}}})
	return MCPWiring{Args: []string{claudeMCPConfigFlag, string(config)}}, nil
}

func codexMCP(bin string, args []string) (MCPWiring, error) {
	// A string and a slice of them: marshalling cannot fail.
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

func commandHook(commands ...string) []map[string]any {
	entries := make([]map[string]string, len(commands))
	for i, command := range commands {
		entries[i] = map[string]string{claudeTypeKey: claudeCommandType, claudeCommandKey: command}
	}
	return []map[string]any{{claudeHooksKey: entries}}
}

// ShellQuote single-quotes s so it survives as one word in the POSIX shell that
// runs hook commands and window commands. Go's %q double-quoting would leave $,
// backticks, and backslashes live for the shell.
func ShellQuote(s string) string {
	return shellQuoteChar + strings.ReplaceAll(s, shellQuoteChar, shellQuoteEscape) + shellQuoteChar
}

func runnerArgv(spec runnerSpec, r Runner, resume bool, mode, prompt string) []string {
	argv := slices.Clone(r.Command)
	if resume {
		argv = spec.Resume(argv)
		if resumed := strings.TrimSpace(prompt); resumed != "" {
			argv = spec.Prompt(argv, resumed)
		}
		return argv
	}
	return spec.Prompt(argv, openingMessage(mode, prompt))
}

func openingMessage(mode, initialPrompt string) string {
	message := openingMessageRPI
	if mode == modeAssistant {
		message = openingMessageAssistant
	}
	if prompt := strings.TrimSpace(initialPrompt); prompt != "" {
		message += linearRequestSeparator + prompt
	}
	return message
}
