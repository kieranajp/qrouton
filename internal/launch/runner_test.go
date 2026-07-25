package launch

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/mux"
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

	cfg := &config.Config{Launch: [][]string{{"codex", "--search"}, {"team-agent", "--fast"}}}
	got := Runners(cfg)
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

func TestRequestedRunnerInitialPromptPresentsRPI(t *testing.T) {
	byID := make(map[string]Runner, len(builtinRunners))
	for _, r := range builtinRunners {
		byID[r.ID] = r
	}

	argv := runnerArgv(byID["codex"], false, modeRPI)
	if len(argv) != 3 || argv[0] != "codex" || argv[1] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("unexpected Codex argv: %#v", argv)
	}
	if !strings.Contains(argv[2], "Research, Plan, or Implement") || strings.Contains(argv[2], "QRSPI") {
		t.Fatalf("initial prompt does not present the RPI workflow: %q", argv[2])
	}
	if argv := runnerArgv(byID["opencode"], false, modeRPI); len(argv) != len(byID["opencode"].Command) {
		t.Fatalf("unknown launch protocol should not receive a positional prompt: %#v", argv)
	}
}

func TestAssistantModeInitialPromptStaysOpenEndedAndOffersEscalation(t *testing.T) {
	byID := make(map[string]Runner, len(builtinRunners))
	for _, r := range builtinRunners {
		byID[r.ID] = r
	}

	argv := runnerArgv(byID["claude"], false, modeAssistant)
	msg := argv[len(argv)-1]
	if strings.Contains(msg, "Present the work as Research, Plan, or Implement") {
		t.Fatalf("assistant opening should not mandate the RPI presentation: %q", msg)
	}
	if !strings.Contains(msg, "help with whatever the user asks") || !strings.Contains(msg, "Research → Plan → Implement workflow is available") {
		t.Fatalf("assistant opening should stay open-ended and offer escalation: %q", msg)
	}
}

func TestRunnerResumeArgvContinuesPreviousConversation(t *testing.T) {
	wants := map[string][]string{
		"claude":   {"--continue"},
		"codex":    {"resume", "--last"},
		"opencode": {"--continue"},
	}
	for _, runner := range builtinRunners {
		argv := runnerArgv(runner, true, modeRPI)
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
		argv, env, err := runnerLaunch(runner, "/bin/qrouton", "/work/session", EditorCommand{Argv: []string{"vi"}}, testHandle(), true)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(argv, " ") + " " + strings.Join(env, " ")
		if !strings.Contains(joined, "qrouton") {
			t.Errorf("%s resumed without qrouton MCP config", runner.ID)
		}
	}
}

func TestRunnerLaunchInjectsClaudeAgentHooks(t *testing.T) {
	r := Runner{ID: "claude", Command: []string{"claude"}}
	argv, _, err := runnerLaunch(r, "/tmp/qrouton", "/tmp/session", EditorCommand{}, testHandle(), false)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--settings", "SubagentStart", "SubagentStop", "agent-event", "--session-root"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("Claude launch missing %q: %v", want, argv)
		}
	}
	for _, want := range []string{"Notification", filepath.Join(".qrouton", "notify.sh")} {
		if !strings.Contains(joined, want) {
			t.Fatalf("Claude launch missing sound hook %q: %v", want, argv)
		}
	}
}

func TestClaudeHookCommandsSurviveShellMetacharacters(t *testing.T) {
	r := Runner{ID: "claude", Command: []string{"claude"}}
	bin := "/opt/qro uton/$peculiar/qrouton"
	dir := "/work/kieran's session"
	argv, _, err := runnerLaunch(r, bin, dir, EditorCommand{}, testHandle(), false)
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	for i, arg := range argv {
		if arg == "--settings" && i+1 < len(argv) {
			raw = argv[i+1]
		}
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		t.Fatalf("settings not parseable: %v\n%s", err, raw)
	}
	want := `'/opt/qro uton/$peculiar/qrouton' agent-event --session-root '/work/kieran'\''s session'`
	if got := settings.Hooks["SubagentStart"][0].Hooks[0].Command; got != want {
		t.Fatalf("hook command = %s, want %s", got, want)
	}
	if got := settings.Hooks["Notification"][0].Hooks[0].Command; !strings.HasPrefix(got, `'/work/kieran'\''s session/`) {
		t.Fatalf("notification command not shell-quoted: %s", got)
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
		argv, env, err := runnerLaunch(r, "/bin/qrouton", "/work/session", EditorCommand{Argv: []string{"vi"}}, testHandle(), false)
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
		if id != "opencode" && (!strings.Contains(joined, "mux-json") || !strings.Contains(joined, "zellij")) {
			t.Fatalf("%s missing explicit multiplexer handle: %v", id, argv)
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

// testHandle is the multiplexer identity runnerLaunch threads into MCP args.
func testHandle() mux.Handle {
	return mux.Handle{Kind: "zellij", Session: "session", SocketDir: "/tmp/zellij"}
}
