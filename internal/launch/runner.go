package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
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

func runnerLaunch(r Runner, qroutonBin, dir string, editor EditorCommand, socketDir string, resume bool) ([]string, []string, error) {
	argv := runnerArgv(r, resume)
	mcpArgs := []string{"mcp", "--session-root", dir, "--editor-json", editor.Marshal(), "--zellij-session", filepath.Base(dir), "--socket-dir", socketDir}
	switch r.ID {
	case "claude":
		mcp := map[string]any{"mcpServers": map[string]any{"qrouton": map[string]any{"type": "stdio", "command": qroutonBin, "args": mcpArgs}}}
		b, _ := json.Marshal(mcp)
		argv = append(argv, "--mcp-config", string(b))
		hookCommand := fmt.Sprintf("%q agent-event --session-root %q", qroutonBin, dir)
		hook := []map[string]any{{"hooks": []map[string]string{{"type": "command", "command": hookCommand}}}}
		// Chime when the agent finishes a turn or asks for attention, so the user can
		// step away while work runs; notify.sh is stamped into .qrouton by writeSupport.
		soundCommand := fmt.Sprintf("%q", filepath.Join(dir, ".qrouton", "notify.sh"))
		soundHook := []map[string]any{{"hooks": []map[string]string{{"type": "command", "command": soundCommand}}}}
		settings, _ := json.Marshal(map[string]any{"hooks": map[string]any{
			"SubagentStart": hook,
			"SubagentStop":  hook,
			"Stop":          soundHook,
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
		return argv, withEnv(os.Environ(), "OPENCODE_CONFIG_CONTENT", string(b)), nil
	default:
		return nil, nil, fmt.Errorf("unsupported runner %q", r.ID)
	}
	return argv, os.Environ(), nil
}

func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

func runnerArgv(r Runner, resume bool) []string {
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
		argv = append(argv, "You have just been launched in a qrouton session. Read the session instructions and manifest, inspect relevant thoughts/shared artifacts, then respond naturally. Present the work as Research, Plan, or Implement; keep your own context lean by delegating execution wherever practical.")
	}
	return argv
}
