package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

var builtinRunners = []Runner{
	{ID: runnerIDClaude, Label: runnerLabelClaude, Command: []string{runnerIDClaude, claudeSkipPermissionsFlag}},
	{ID: runnerIDCodex, Label: runnerLabelCodex, Command: []string{runnerIDCodex, codexBypassSandboxFlag}},
	{ID: runnerIDOpenCode, Label: runnerLabelOpenCode, Command: []string{runnerIDOpenCode, openCodeAutoFlag}},
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

func runnerLaunch(r Runner, qroutonBin, dir string, editor EditorCommand, handle workbench.Handle, resume bool) ([]string, []string, error) {
	argv := runnerArgv(r, resume, sessionMode(dir))
	mcpArgs := []string{mcpSubcommand, sessionRootFlag, dir, editorJSONFlag, editor.Marshal(), workbenchJSONFlag, handle.Marshal()}
	switch r.ID {
	case runnerIDClaude:
		mcp := map[string]any{claudeMCPServersKey: map[string]any{serverName: map[string]any{
			claudeTypeKey: claudeStdioType, claudeCommandKey: qroutonBin, claudeArgsKey: mcpArgs}}}
		b, _ := json.Marshal(mcp)
		argv = append(argv, claudeMCPConfigFlag, string(b))
		hookCommand := ShellQuote(qroutonBin) + " " + agentEventSubcommand +
			" " + sessionRootFlag + " " + ShellQuote(dir) +
			" " + workbenchJSONFlag + " " + ShellQuote(handle.Marshal())
		// Chime only when the agent asks for attention (not on every turn), so the user
		// can step away; notify.sh is stamped into .qrouton by writeSupport.
		soundCommand := ShellQuote(sessionpaths.NotifyScript(dir))
		settings, _ := json.Marshal(map[string]any{claudeHooksKey: map[string]any{
			claudeSubagentStartHook: commandHook(hookCommand),
			claudeSubagentStopHook:  commandHook(hookCommand),
			claudeNotificationHook:  commandHook(soundCommand, hookCommand),
		}})
		argv = append(argv, claudeSettingsFlag, string(settings))
	case runnerIDCodex:
		command, _ := json.Marshal(qroutonBin)
		args, _ := json.Marshal(mcpArgs)
		argv = append(argv, codexConfigFlag, codexMCPCommandKey+string(command), codexConfigFlag, codexMCPArgsKey+string(args))
	case runnerIDOpenCode:
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
			claudeTypeKey: openCodeLocalType, claudeCommandKey: append([]string{qroutonBin}, mcpArgs...), openCodeEnabledKey: true}
		content[openCodeMCPKey] = servers
		if !r.Override {
			content[openCodePermissionKey] = openCodeAllowValue
		}
		b, _ := json.Marshal(content)
		return argv, workbench.WithEnv(os.Environ(), openCodeConfigEnvVar, string(b)), nil
	default:
		return nil, nil, fmt.Errorf("%w: %q", ErrUnsupportedRunner, r.ID)
	}
	return argv, os.Environ(), nil
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

func runnerArgv(r Runner, resume bool, mode string) []string {
	argv := append([]string(nil), r.Command...)
	if resume {
		switch r.ID {
		case runnerIDClaude, runnerIDOpenCode:
			return append(argv, claudeContinueFlag)
		case runnerIDCodex:
			return append(argv, codexResumeCmd, codexResumeLast)
		default:
			return argv
		}
	}
	switch r.ID {
	case runnerIDClaude, runnerIDCodex:
		argv = append(argv, openingMessage(mode))
	}
	return argv
}

// openingMessage is the fresh-session greeting injected as the runner's first
// prompt.
func openingMessage(mode string) string {
	if mode == modeAssistant {
		return openingMessageAssistant
	}
	return openingMessageRPI
}
