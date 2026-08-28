package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

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

// runnerSpec is everything qrouton knows about one coding agent: how to start
// it, how to ask it to continue a conversation, how it takes a first prompt, and
// how its MCP servers and hooks are configured. One entry per runner, so adding
// a fourth is one literal rather than an edit in four places.
type runnerSpec struct {
	ID      string
	Label   string
	Command []string
	// Resume turns a fresh argv into one continuing the last conversation.
	Resume func(argv []string) []string
	// Prompt appends the opening message the way this runner accepts it.
	Prompt func(argv []string, message string) []string
	// Inject adds MCP and hook configuration, and answers the environment the
	// runner is launched with.
	Inject func(argv []string, c injectContext) (outArgv, env []string, err error)
}

// injectContext is what an Inject needs beyond the argv: where this binary and
// the session are, and the arguments the MCP child is started with.
type injectContext struct {
	qroutonBin string
	dir        string
	handle     workbench.Handle
	mcpArgs    []string
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
		Inject:  injectClaude,
	},
	{
		ID: runnerIDCodex, Label: runnerLabelCodex,
		Command: []string{runnerIDCodex, codexBypassSandboxFlag},
		Resume:  resumeWith(codexResumeCmd, codexResumeLast),
		Prompt:  promptAsArgument,
		Inject:  injectCodex,
	},
	{
		ID: runnerIDOpenCode, Label: runnerLabelOpenCode,
		Command: []string{runnerIDOpenCode, openCodeAutoFlag},
		Resume:  resumeWith(claudeContinueFlag),
		Prompt:  promptBehindFlag(openCodePromptFlag),
		Inject:  injectOpenCode,
	},
}

// builtinRunners is the spec table as the resolver sees it: identity and command
// only, with the behaviour left behind.
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

// FirstInstalled returns the first supported runner present on the system, for
// callers that did not ask for a particular one.
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

func runnerLaunch(r Runner, qroutonBin, dir string, editor EditorCommand, handle workbench.Handle, resume bool, initialPrompt string) ([]string, []string, error) {
	// Runner's fields are exported, so a hand-built one can still reach here
	// without the per-runner wiring the launch path needs.
	spec, ok := specFor(r.ID)
	if !ok {
		return nil, nil, fmt.Errorf("%w: %q", ErrUnsupportedRunner, r.ID)
	}
	return spec.Inject(runnerArgv(r, resume, sessionMode(dir), initialPrompt), injectContext{
		qroutonBin: qroutonBin,
		dir:        dir,
		handle:     handle,
		mcpArgs: []string{mcpSubcommand, sessionRootFlag, dir,
			editorJSONFlag, editor.Marshal(), workbenchJSONFlag, handle.Marshal()},
		override: r.Override,
	})
}

func injectClaude(argv []string, c injectContext) ([]string, []string, error) {
	mcp := map[string]any{claudeMCPServersKey: map[string]any{serverName: map[string]any{
		claudeTypeKey: claudeStdioType, claudeCommandKey: c.qroutonBin, claudeArgsKey: c.mcpArgs}}}
	b, _ := json.Marshal(mcp)
	argv = append(argv, claudeMCPConfigFlag, string(b))
	hookCommand := ShellQuote(c.qroutonBin) + " " + agentEventSubcommand +
		" " + sessionRootFlag + " " + ShellQuote(c.dir) +
		" " + workbenchJSONFlag + " " + ShellQuote(c.handle.Marshal())
	// Chime only when the agent asks for attention (not on every turn), so the user
	// can step away; notify.sh is stamped into .qrouton by writeSupport.
	soundCommand := ShellQuote(sessionpaths.NotifyScript(c.dir))
	settings, _ := json.Marshal(map[string]any{claudeHooksKey: map[string]any{
		claudeSubagentStartHook: commandHook(hookCommand),
		claudeSubagentStopHook:  commandHook(hookCommand),
		claudeNotificationHook:  commandHook(soundCommand, hookCommand),
	}})
	return append(argv, claudeSettingsFlag, string(settings)), os.Environ(), nil
}

func injectCodex(argv []string, c injectContext) ([]string, []string, error) {
	command, _ := json.Marshal(c.qroutonBin)
	args, _ := json.Marshal(c.mcpArgs)
	argv = append(argv, codexConfigFlag, codexMCPCommandKey+string(command),
		codexConfigFlag, codexMCPArgsKey+string(args))
	return argv, os.Environ(), nil
}

// injectOpenCode is the one runner configured through the environment rather
// than argv, and the one qrouton has to merge into a config the user may
// already be passing.
func injectOpenCode(argv []string, c injectContext) ([]string, []string, error) {
	content := map[string]any{}
	if existing := os.Getenv(openCodeConfigEnvVar); existing != "" {
		if err := json.Unmarshal([]byte(existing), &content); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", openCodeConfigEnvVar, err)
		}
	}
	servers, _ := content[openCodeMCPKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[serverName] = map[string]any{
		claudeTypeKey: openCodeLocalType, claudeCommandKey: append([]string{c.qroutonBin}, c.mcpArgs...), openCodeEnabledKey: true}
	content[openCodeMCPKey] = servers
	if !c.override {
		content[openCodePermissionKey] = openCodeAllowValue
	}
	b, _ := json.Marshal(content)
	return argv, workbench.WithEnv(os.Environ(), openCodeConfigEnvVar, string(b)), nil
}

// commandHook is one Claude hook entry running the given shell commands in
// order.
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

func runnerArgv(r Runner, resume bool, mode, initialPrompt string) []string {
	argv := slices.Clone(r.Command)
	spec, ok := specFor(r.ID)
	if !ok {
		return argv
	}
	if resume {
		return spec.Resume(argv)
	}
	return spec.Prompt(argv, openingMessage(mode, initialPrompt))
}

// openingMessage is the fresh-session greeting injected as the runner's first
// prompt.
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
