package main

import (
	"fmt"
	"reflect"
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
	if byID["team-agent"].Path != "/bin/team-agent" {
		t.Fatal("custom configured runner not detected")
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
	if len(argv) != 2 || argv[0] != "codex" {
		t.Fatalf("unexpected Codex argv: %#v", argv)
	}
	open, err := chooseRunner(&Config{}, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	if argv := runnerArgv(open); len(argv) != 1 {
		t.Fatalf("unknown launch protocol should not receive a positional prompt: %#v", argv)
	}
}
