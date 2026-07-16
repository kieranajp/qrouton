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
	argv := runnerArgv(r, false)
	if len(argv) != 3 || argv[0] != "codex" || argv[1] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("unexpected Codex argv: %#v", argv)
	}
	if !strings.Contains(argv[2], "Research, Plan, or Implement") || strings.Contains(argv[2], "QRSPI") {
		t.Fatalf("initial prompt does not present the RPI workflow: %q", argv[2])
	}
	open, err := chooseRunner(&Config{}, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	if argv := runnerArgv(open, false); len(argv) != len(open.Command) {
		t.Fatalf("unknown launch protocol should not receive a positional prompt: %#v", argv)
	}
}

func TestRunnerResumeArgvContinuesPreviousConversation(t *testing.T) {
	wants := map[string][]string{
		"claude":   {"--continue"},
		"codex":    {"resume", "--last"},
		"opencode": {"--continue"},
	}
	for _, runner := range builtinRunners {
		argv := runnerArgv(runner, true)
		if !reflect.DeepEqual(argv[len(argv)-len(wants[runner.ID]):], wants[runner.ID]) {
			t.Errorf("%s resume argv = %#v, want suffix %#v", runner.ID, argv, wants[runner.ID])
		}
		if strings.Contains(strings.Join(argv, " "), "just been launched") {
			t.Errorf("%s resume argv included fresh-session greeting: %#v", runner.ID, argv)
		}
	}
}

func TestResumedRunnerStillReceivesMCPConfiguration(t *testing.T) {
	for _, runner := range builtinRunners {
		argv, env, err := runnerLaunch(runner, "/bin/qrouton", "/work/session", editorCommand{Argv: []string{"vi"}}, "/tmp/zellij", true)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(argv, " ") + " " + strings.Join(env, " ")
		if !strings.Contains(joined, "qrouton") {
			t.Errorf("%s resumed without qrouton MCP config", runner.ID)
		}
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
		argv, env, err := runnerLaunch(r, "/bin/qrouton", "/work/session", editorCommand{Argv: []string{"vi"}}, "/tmp/zellij", false)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(argv, " ")
		if id != "opencode" && !strings.Contains(joined, "qrouton") {
			t.Fatalf("%s missing MCP config: %v", id, argv)
		}
		if id != "opencode" && !strings.Contains(joined, "editor-json") {
			t.Fatalf("%s missing explicit editor config: %v", id, argv)
		}
		if id != "opencode" && (!strings.Contains(joined, "zellij-session") || !strings.Contains(joined, "socket-dir")) {
			t.Fatalf("%s missing explicit Zellij target: %v", id, argv)
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
