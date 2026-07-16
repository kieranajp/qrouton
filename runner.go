package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
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
func runners(cfg *Config) []Runner {
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

func runnerLaunch(r Runner, qroutonBin, dir string, editor editorCommand) ([]string, []string, error) {
	argv := runnerArgv(r)
	mcpArgs := []string{"mcp", "--session-root", dir, "--editor-json", editor.marshal()}
	switch r.ID {
	case "claude":
		mcp := map[string]any{"mcpServers": map[string]any{"qrouton": map[string]any{"type": "stdio", "command": qroutonBin, "args": mcpArgs}}}
		b, _ := json.Marshal(mcp)
		argv = append(argv, "--mcp-config", string(b))
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

func chooseRunner(cfg *Config, requested string) (Runner, error) {
	all := runners(cfg)
	if requested != "" {
		for _, r := range all {
			if r.ID == requested || r.Command[0] == requested {
				if r.Path == "" {
					return Runner{}, fmt.Errorf("runner %q is not installed (could not find %s)", r.ID, r.Command[0])
				}
				return r, nil
			}
		}
		return Runner{}, fmt.Errorf("unknown runner %q", requested)
	}

	var available []huh.Option[string]
	var unavailable []string
	selected := ""
	for _, r := range all {
		if r.Path == "" {
			unavailable = append(unavailable, r.Label)
			continue
		}
		if selected == "" || r.ID == "claude" {
			selected = r.ID
		}
		available = append(available, huh.NewOption(r.Label, r.ID))
	}
	if len(available) == 0 {
		return Runner{}, fmt.Errorf("no supported coding agent is installed")
	}
	description := "Choose the coding agent for this launch"
	if len(unavailable) > 0 {
		description += ". Not installed: \x1b[2m" + strings.Join(unavailable, ", ") + "\x1b[0m"
	}
	if err := huh.NewSelect[string]().Title("Coding agent").Description(description).
		Options(available...).Value(&selected).Run(); err != nil {
		return Runner{}, err
	}
	for _, r := range all {
		if r.ID == selected {
			return r, nil
		}
	}
	return Runner{}, fmt.Errorf("runner %q disappeared", selected)
}

func runnerArgv(r Runner) []string {
	argv := append([]string(nil), r.Command...)
	switch r.ID {
	case "claude", "codex":
		argv = append(argv, "You have just been launched in a qrouton session. Read the session instructions, qrouton.json, and existing thoughts/shared documents; derive the current QRSPI phase, greet the user, propose the next step, then wait.")
	}
	return argv
}
