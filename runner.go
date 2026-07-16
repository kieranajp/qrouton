package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
)

type Runner struct {
	ID      string
	Label   string
	Command []string
	Path    string
}

var builtinRunners = []Runner{
	{ID: "claude", Label: "Claude Code", Command: []string{"claude"}},
	{ID: "codex", Label: "Codex CLI", Command: []string{"codex"}},
	{ID: "opencode", Label: "OpenCode", Command: []string{"opencode"}},
	{ID: "agy", Label: "Agy", Command: []string{"agy"}},
	{ID: "pi", Label: "Pi", Command: []string{"pi"}},
}

var findExecutable = exec.LookPath

// runners combines qrouton's known adapters with legacy configured commands. A configured command
// for a built-in runner supplies its arguments; other commands remain available as custom runners.
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
			continue
		}
		byID[id] = len(out)
		out = append(out, Runner{ID: id, Label: id + " (custom)", Command: append([]string(nil), command...)})
	}
	for i := range out {
		out[i].Path, _ = findExecutable(out[i].Command[0])
	}
	return out
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
