package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type Runner struct {
	ID       string
	Label    string
	Command  []string
	Path     string
	Override bool
}

var builtinRunners = []Runner{
	{ID: "claude", Label: "Claude Code", Command: []string{"claude", "--dangerously-skip-permissions"}},
	{ID: "codex", Label: "Codex CLI", Command: []string{"codex", "--dangerously-bypass-approvals-and-sandbox"}},
	{ID: "opencode", Label: "OpenCode", Command: []string{"opencode", "--auto"}},
}

var findExecutable = exec.LookPath

// runners applies exact configured overrides to qrouton's supported, tool-capable runners.
func Runners(cfg *config.Config) []Runner {
	out := make([]Runner, len(builtinRunners))
	copy(out, builtinRunners)
	byID := make(map[string]int, len(out))
	for i := range out {
		byID[out[i].ID] = i
	}
	for _, command := range cfg.Launch {
		if len(command) == 0 {
			continue
		}
		id := filepath.Base(command[0])
		if i, ok := byID[id]; ok {
			out[i].Command = append([]string(nil), command...)
			out[i].Override = true
			continue
		}
	}
	for i := range out {
		out[i].Path, _ = findExecutable(out[i].Command[0])
	}
	return out
}

// ByID returns the installed runner matching id, which may be a runner
// identifier ("claude") or the command qrouton would run ("claude", or a path
// to it). An uninstalled or unknown runner is an error: the caller asked for
// something specific, so silently substituting another would be wrong.
func ByID(cfg *config.Config, id string) (Runner, error) {
	for _, runner := range Runners(cfg) {
		if runner.Path == "" {
			continue
		}
		if runner.ID == id || runner.Command[0] == id || filepath.Base(runner.Command[0]) == id {
			return runner, nil
		}
	}
	return Runner{}, fmt.Errorf("runner %q is unavailable", id)
}

// FirstInstalled returns the first supported runner present on the system, for
// callers that did not ask for a particular one.
func FirstInstalled(cfg *config.Config) (Runner, error) {
	for _, runner := range Runners(cfg) {
		if runner.Path != "" {
			return runner, nil
		}
	}
	return Runner{}, ErrNoRunnerInstalled
}

func runnerLaunch(r Runner, qroutonBin, dir string, editor EditorCommand, handle mux.Handle, resume bool) ([]string, []string, error) {
	argv := runnerArgv(r, resume, sessionMode(dir))
	mcpArgs := []string{"mcp", "--session-root", dir, "--editor-json", editor.Marshal(), "--mux-json", handle.Marshal()}
	switch r.ID {
	case "claude":
		mcp := map[string]any{"mcpServers": map[string]any{"qrouton": map[string]any{"type": "stdio", "command": qroutonBin, "args": mcpArgs}}}
		b, _ := json.Marshal(mcp)
		argv = append(argv, "--mcp-config", string(b))
		hookCommand := ShellQuote(qroutonBin) + " agent-event --session-root " + ShellQuote(dir)
		hook := []map[string]any{{"hooks": []map[string]string{{"type": "command", "command": hookCommand}}}}
		// Chime only when the agent asks for attention (not on every turn), so the user
		// can step away; notify.sh is stamped into .qrouton by writeSupport.
		soundCommand := ShellQuote(sessionpaths.NotifyScript(dir))
		soundHook := []map[string]any{{"hooks": []map[string]string{{"type": "command", "command": soundCommand}}}}
		settings, _ := json.Marshal(map[string]any{"hooks": map[string]any{
			"SubagentStart": hook,
			"SubagentStop":  hook,
			"Notification":  soundHook,
		}})
		argv = append(argv, "--settings", string(settings))
	case "codex":
		command, _ := json.Marshal(qroutonBin)
		args, _ := json.Marshal(mcpArgs)
		argv = append(argv, "-c", "mcp_servers.qrouton.command="+string(command), "-c", "mcp_servers.qrouton.args="+string(args))
	case "opencode":
		content := map[string]any{}
		if existing := os.Getenv("OPENCODE_CONFIG_CONTENT"); existing != "" {
			if err := json.Unmarshal([]byte(existing), &content); err != nil {
				return nil, nil, fmt.Errorf("OPENCODE_CONFIG_CONTENT: %w", err)
			}
		}
		servers, _ := content["mcp"].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
		}
		servers["qrouton"] = map[string]any{"type": "local", "command": append([]string{qroutonBin}, mcpArgs...), "enabled": true}
		content["mcp"] = servers
		if !r.Override {
			content["permission"] = "allow"
		}
		b, _ := json.Marshal(content)
		return argv, mux.WithEnv(os.Environ(), openCodeConfigEnvVar, string(b)), nil
	default:
		return nil, nil, fmt.Errorf("unsupported runner %q", r.ID)
	}
	return argv, os.Environ(), nil
}

// ShellQuote single-quotes s so it survives as one word in the POSIX shell that
// runs hook commands and pane commands. Go's %q double-quoting would leave $,
// backticks, and backslashes live for the shell.
func ShellQuote(s string) string {
	return shellQuoteChar + strings.ReplaceAll(s, shellQuoteChar, shellQuoteEscape) + shellQuoteChar
}

func runnerArgv(r Runner, resume bool, mode string) []string {
	argv := append([]string(nil), r.Command...)
	if resume {
		switch r.ID {
		case "claude", "opencode":
			return append(argv, "--continue")
		case "codex":
			return append(argv, "resume", "--last")
		default:
			return argv
		}
	}
	switch r.ID {
	case "claude", "codex":
		argv = append(argv, openingMessage(mode))
	}
	return argv
}

// openingMessage is the fresh-session greeting injected as the runner's first
// prompt. RPI presents the orchestrated workflow; Assistant stays open-ended
// while pointing at the workflow the user can escalate into.
func openingMessage(mode string) string {
	if mode == modeAssistant {
		return "You have just been launched in a qrouton session. Read the session instructions and manifest, skim relevant thoughts/shared artifacts, then help with whatever the user asks — work directly and keep your own context lean. A structured Research → Plan → Implement workflow is available if the user wants it."
	}
	return "You have just been launched in a qrouton session. Read the session instructions and manifest, inspect relevant thoughts/shared artifacts, then respond naturally. Present the work as Research, Plan, or Implement; keep your own context lean by delegating execution wherever practical."
}
