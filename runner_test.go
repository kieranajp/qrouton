package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestRunnersDetectBuiltinsAndApplyConfiguredArguments(t *testing.T) {
	old := findExecutable
	t.Cleanup(func() { findExecutable = old })
	findExecutable = func(name string) (string, error) {
		if name == "claude" || name == "codex" || name == "team-agent" {
			return "/bin/" + name, nil
		}
		return "", fmt.Errorf("missing")
	}

	cfg := &Config{Launch: [][]string{{"codex", "--search"}, {"team-agent", "--fast"}}}
	got := runners(cfg)
	byID := make(map[string]Runner)
	for _, r := range got {
		byID[r.ID] = r
	}
	if !reflect.DeepEqual(byID["codex"].Command, []string{"codex", "--search"}) {
		t.Fatalf("configured Codex args lost: %#v", byID["codex"].Command)
	}
	if byID["opencode"].Path != "" {
		t.Fatal("missing runner reported installed")
	}
	if _, ok := byID["team-agent"]; ok {
		t.Fatal("unsupported custom runner was retained")
	}
	if !byID["codex"].Override {
		t.Fatal("configured built-in not marked as an override")
	}
}

func TestChooseRequestedRunnerAndInitialPrompt(t *testing.T) {
	old := findExecutable
	t.Cleanup(func() { findExecutable = old })
	findExecutable = func(name string) (string, error) { return "/bin/" + name, nil }

	r, err := chooseRunner(&Config{}, "codex")
	if err != nil {
		t.Fatal(err)
	}
	argv := runnerArgv(r)
	if len(argv) != 3 || argv[0] != "codex" || argv[1] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("unexpected Codex argv: %#v", argv)
	}
	open, err := chooseRunner(&Config{}, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	if argv := runnerArgv(open); len(argv) != len(open.Command) {
		t.Fatalf("unknown launch protocol should not receive a positional prompt: %#v", argv)
	}
}

func TestRunnerLaunchInjectsMCPAndOpenCodePermissions(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG_CONTENT", `{"model":"test"}`)
	for _, id := range []string{"claude", "codex", "opencode"} {
		var r Runner
		for _, candidate := range builtinRunners {
			if candidate.ID == id {
				r = candidate
			}
		}
		argv, env, err := runnerLaunch(r, "/bin/qrouton", "/work/session")
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(argv, " ")
		if id != "opencode" && !strings.Contains(joined, "qrouton") {
			t.Fatalf("%s missing MCP config: %v", id, argv)
		}
		if id == "opencode" {
			var raw string
			for _, item := range env {
				if strings.HasPrefix(item, "OPENCODE_CONFIG_CONTENT=") {
					raw = strings.TrimPrefix(item, "OPENCODE_CONFIG_CONTENT=")
				}
			}
			var cfg map[string]any
			if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
				t.Fatal(err)
			}
			if cfg["model"] != "test" || cfg["permission"] != "allow" {
				t.Fatalf("OpenCode content not merged: %s", raw)
			}
		}
	}
}
