package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
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

	cfg := &config.Config{Launch: map[string][]string{"codex": {"codex", "--search"}}}
	got, err := Runners(cfg)
	if err != nil {
		t.Fatal(err)
	}
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
	if !byID["codex"].Override {
		t.Fatal("configured built-in not marked as an override")
	}
}

// An override qrouton cannot wire up used to be dropped in silence, so a user
// who configured one got the built-in default and no explanation.
func TestRunnersRejectUnsupportedOverride(t *testing.T) {
	old := findExecutable
	t.Cleanup(func() { findExecutable = old })
	findExecutable = func(name string) (string, error) { return "/bin/" + name, nil }

	_, err := Runners(&config.Config{Launch: map[string][]string{"team-agent": {"team-agent", "--fast"}}})
	if !errors.Is(err, ErrUnsupportedOverride) {
		t.Fatalf("unsupported override error = %v, want ErrUnsupportedOverride", err)
	}
	if !strings.Contains(err.Error(), "team-agent") {
		t.Fatalf("error does not name the offending command: %v", err)
	}
}

func TestRequestedRunnerInitialPromptPresentsRPI(t *testing.T) {
	byID := make(map[string]Runner, len(builtinRunners))
	for _, r := range builtinRunners {
		byID[r.ID] = r
	}

	argv := runnerArgv(byID["codex"], false, modeRPI, "")
	if len(argv) != 3 || argv[0] != "codex" || argv[1] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("unexpected Codex argv: %#v", argv)
	}
	if !strings.Contains(argv[2], "Research, Plan, or Implement") || strings.Contains(argv[2], "QRSPI") {
		t.Fatalf("initial prompt does not present the RPI workflow: %q", argv[2])
	}
	if argv := runnerArgv(byID["opencode"], false, modeRPI, ""); len(argv) != len(byID["opencode"].Command)+2 || argv[len(argv)-2] != openCodePromptFlag ||
		!strings.Contains(argv[len(argv)-1], "Research, Plan, or Implement") {
		t.Fatalf("OpenCode should receive the opening through --prompt: %#v", argv)
	}
}

func TestAssistantModeInitialPromptStaysOpenEndedAndOffersEscalation(t *testing.T) {
	byID := make(map[string]Runner, len(builtinRunners))
	for _, r := range builtinRunners {
		byID[r.ID] = r
	}

	argv := runnerArgv(byID["claude"], false, modeAssistant, "")
	msg := argv[len(argv)-1]
	if strings.Contains(msg, "Present the work as Research, Plan, or Implement") {
		t.Fatalf("assistant opening should not mandate the RPI presentation: %q", msg)
	}
	if !strings.Contains(msg, "help with whatever the user asks") || !strings.Contains(msg, "Research → Plan → Implement workflow is available") {
		t.Fatalf("assistant opening should stay open-ended and offer escalation: %q", msg)
	}
}

func TestLinearPromptIsLayeredUnderQroutonOpeningMessage(t *testing.T) {
	for _, runner := range builtinRunners {
		argv := runnerArgv(runner, false, modeAssistant, "  Fix the login regression.  ")
		message := argv[len(argv)-1]
		if !strings.HasPrefix(message, openingMessageAssistant) ||
			!strings.HasSuffix(message, linearRequestSeparator+"Fix the login regression.") {
			t.Fatalf("%s opening message = %q", runner.ID, message)
		}
	}
}

func TestRunnerResumeArgvContinuesPreviousConversation(t *testing.T) {
	wants := map[string][]string{
		"claude":   {"--continue"},
		"codex":    {"resume", "--last"},
		"opencode": {"--continue"},
	}
	for _, runner := range builtinRunners {
		argv := runnerArgv(runner, true, modeRPI, "must not be repeated")
		if !reflect.DeepEqual(argv[len(argv)-len(wants[runner.ID]):], wants[runner.ID]) {
			t.Errorf("%s resume argv = %#v, want suffix %#v", runner.ID, argv, wants[runner.ID])
		}
		if strings.Contains(strings.Join(argv, " "), "just been launched") {
			t.Errorf("%s resume argv included fresh-session greeting: %#v", runner.ID, argv)
		}
		if strings.Contains(strings.Join(argv, " "), "must not be repeated") {
			t.Errorf("%s resume argv repeated the external prompt: %#v", runner.ID, argv)
		}
	}
}

func TestResumedRunnerStillReceivesMCPConfiguration(t *testing.T) {
	for _, runner := range builtinRunners {
		argv, env, err := runnerLaunch(runner, "/bin/qrouton", "/work/session", EditorCommand{Argv: []string{"vi"}}, testHandle(), 1, true, "")
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
	argv, _, err := runnerLaunch(r, "/tmp/qrouton", "/tmp/session", EditorCommand{}, testHandle(), 7, false, "")
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
	handle := workbench.Handle{Socket: "/tmp/qr outon/it's.sock", SessionRoot: dir}
	argv, _, err := runnerLaunch(r, bin, dir, EditorCommand{}, handle, 7, false, "")
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
	callback := settings.Hooks["SubagentStart"][0].Hooks[0].Command
	want := []string{bin, "agent-event", "--session-root", dir, "--workbench-json", handle.Marshal(), "--generation", "7", "--provider", "claude"}
	if got := shellWords(t, callback); !reflect.DeepEqual(got, want) {
		t.Fatalf("hook command splits to %q, want %q", got, want)
	}
	// Notification runs the sound and then the callback that turns the header
	// peach; losing either leaves an agent blocked on the user saying nothing.
	notification := settings.Hooks["Notification"][0].Hooks
	if len(notification) != 2 {
		t.Fatalf("Notification carries %d commands, want the sound and the callback", len(notification))
	}
	if got := shellWords(t, notification[0].Command); !reflect.DeepEqual(got, []string{sessionpaths.NotifyScript(dir)}) {
		t.Fatalf("notification sound splits to %q", got)
	}
	if notification[1].Command != callback {
		t.Fatalf("notification callback = %s", notification[1].Command)
	}
}

func TestRunnerLaunchInjectsCodexAgentHooks(t *testing.T) {
	r := Runner{ID: runnerIDCodex, Command: []string{runnerIDCodex, codexBypassSandboxFlag}}
	handle := testHandle()
	argv, env, err := runnerLaunch(r, "/tmp/qrouton", "/tmp/session", EditorCommand{}, handle, 7, false, "")
	if err != nil {
		t.Fatal(err)
	}
	hook := fmt.Sprintf(codexCommandHookFormat,
		quotedConfigString(fmt.Sprintf(codexAgentEventCommandFormat, agentevent.QroutonBinEnvVar, agentEventSubcommand)))
	for _, want := range []string{
		codexBypassHookTrustFlag,
		codexSubagentStartHook + hook,
		codexSubagentStopHook + hook,
	} {
		if !slices.Contains(argv, want) {
			t.Fatalf("Codex launch missing %q: %v", want, argv)
		}
	}
	for key, value := range map[string]string{
		agentevent.QroutonBinEnvVar:  "/tmp/qrouton",
		agentevent.SessionRootEnvVar: "/tmp/session",
		agentevent.WorkbenchEnvVar:   handle.Marshal(),
		agentevent.GenerationEnvVar:  "7",
		agentevent.ProviderEnvVar:    runnerIDCodex,
	} {
		if !slices.Contains(env, key+"="+value) {
			t.Errorf("Codex hook environment missing %s=%q", key, value)
		}
	}
}

func TestConfiguredCodexKeepsHookTrustReview(t *testing.T) {
	r := Runner{ID: runnerIDCodex, Command: []string{runnerIDCodex}, Override: true}
	argv, _, err := runnerLaunch(r, "/tmp/qrouton", "/tmp/session", EditorCommand{}, testHandle(), 7, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(argv, codexBypassHookTrustFlag) {
		t.Fatalf("configured Codex launch bypassed the user's hook trust: %v", argv)
	}
	if !strings.Contains(strings.Join(argv, " "), codexSubagentStartHook) {
		t.Fatalf("configured Codex launch lost lifecycle hooks: %v", argv)
	}
}

// shellWords is what /bin/sh makes of a hook command. Asserting on the string
// stops at the first thing left unquoted; asserting on the words the real shell
// recovers is the property ShellQuote exists for.
func shellWords(t *testing.T, command string) []string {
	t.Helper()
	out, err := exec.Command("/bin/sh", "-c", `printf '%s\n' `+command).Output()
	if err != nil {
		t.Fatalf("sh could not parse %s: %v", command, err)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
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
		argv, env, err := runnerLaunch(r, "/bin/qrouton", "/work/session", EditorCommand{Argv: []string{"vi"}}, testHandle(), 1, false, "")
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
		if id != "opencode" && (!strings.Contains(joined, "workbench-json") || !strings.Contains(joined, testSocket)) {
			t.Fatalf("%s missing explicit session handle: %v", id, argv)
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

const testSocket = "/tmp/qrouton/501/deadbeef.sock"

// testHandle is the session identity runnerLaunch threads into MCP args.
func testHandle() workbench.Handle {
	return workbench.Handle{Socket: testSocket, SessionRoot: "/sessions/session"}
}

// The old shape keyed an override by argv[0], so `[["claude"]]` read as "my
// runner is claude" while meaning "run claude with no flags" — silently
// dropping --dangerously-skip-permissions. Keyed by runner id, dropping flags
// is something you can only do on purpose.
func TestRunnersOverrideReplacesArgvForTheKeyedRunnerOnly(t *testing.T) {
	old := findExecutable
	t.Cleanup(func() { findExecutable = old })
	findExecutable = func(name string) (string, error) { return "/bin/" + filepath.Base(name), nil }

	got, err := Runners(&config.Config{Launch: map[string][]string{"claude": {"/opt/beta/claude", "--flag"}}})
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]Runner)
	for _, r := range got {
		byID[r.ID] = r
	}
	// The key is the identity, so argv need not name the runner at all: an
	// override may point somewhere else entirely and still be claude.
	if !reflect.DeepEqual(byID["claude"].Command, []string{"/opt/beta/claude", "--flag"}) {
		t.Fatalf("claude command = %#v", byID["claude"].Command)
	}
	// Untouched runners keep their built-in flags.
	if !reflect.DeepEqual(byID["codex"].Command, []string{"codex", codexBypassSandboxFlag}) {
		t.Fatalf("codex lost its built-in flags: %#v", byID["codex"].Command)
	}
	if byID["codex"].Override {
		t.Fatal("untouched runner marked as an override")
	}
}

// An override with no command would report the runner as not installed, which
// sends you looking at PATH instead of at your config.
func TestRunnersRejectEmptyOverride(t *testing.T) {
	old := findExecutable
	t.Cleanup(func() { findExecutable = old })
	findExecutable = func(name string) (string, error) { return "/bin/" + name, nil }

	_, err := Runners(&config.Config{Launch: map[string][]string{"claude": {}}})
	if !errors.Is(err, ErrEmptyOverride) {
		t.Fatalf("empty override error = %v, want ErrEmptyOverride", err)
	}
}

// Every spec has to answer all three questions the launch path asks it. An entry
// added with one of them left nil would panic on the launch it was added for.
func TestEverySpecIsCompletelyWired(t *testing.T) {
	if len(runnerSpecs) == 0 {
		t.Fatal("no runners are registered")
	}
	seen := map[string]bool{}
	for _, spec := range runnerSpecs {
		if spec.ID == "" || spec.Label == "" || len(spec.Command) == 0 {
			t.Errorf("spec %+v has no identity or no command", spec)
		}
		if seen[spec.ID] {
			t.Errorf("two specs claim id %q, so specFor answers with whichever is first", spec.ID)
		}
		seen[spec.ID] = true
		if spec.Resume == nil || spec.Prompt == nil || spec.MCP == nil || spec.Inject == nil {
			t.Errorf("spec %q is missing Resume, Prompt, MCP or Inject", spec.ID)
		}
	}
}

// The eval reaches the qrouton MCP server through this, standing its own binary
// and mock arguments in: a runner missing from it is a runner the eval cannot
// grade as it ships.
func TestEveryRunnerPointsAtAQroutonMCPServer(t *testing.T) {
	for _, spec := range runnerSpecs {
		wiring, err := RunnerMCPWiring(spec.ID, "/opt/qrouton", []string{"mcp", "--session-root", "/s"})
		if err != nil {
			t.Fatalf("%s: %v", spec.ID, err)
		}
		configured := strings.Join(append(append([]string(nil), wiring.Args...), wiring.Env...), " ")
		for _, want := range []string{serverName, "/opt/qrouton", "--session-root"} {
			if !strings.Contains(configured, want) {
				t.Errorf("%s wiring %q does not carry %q", spec.ID, configured, want)
			}
		}
	}
	if _, err := RunnerMCPWiring("aider", "/opt/qrouton", nil); !errors.Is(err, ErrUnsupportedRunner) {
		t.Fatalf("wiring an unknown runner = %v, want %v", err, ErrUnsupportedRunner)
	}
}

// builtinRunners is derived, so the resolver cannot fall out of step with the
// behaviour table — and it has to be a copy, or an override would edit the spec
// every later call reads.
func TestBuiltinRunnersMirrorTheSpecTableWithoutSharingIt(t *testing.T) {
	if len(builtinRunners) != len(runnerSpecs) {
		t.Fatalf("%d builtins for %d specs", len(builtinRunners), len(runnerSpecs))
	}
	for i, spec := range runnerSpecs {
		if builtinRunners[i].ID != spec.ID || builtinRunners[i].Label != spec.Label {
			t.Errorf("builtin %d = %q/%q, spec = %q/%q",
				i, builtinRunners[i].ID, builtinRunners[i].Label, spec.ID, spec.Label)
		}
	}
	fresh := builtins()
	fresh[0].Command[0] = "clobbered"
	if runnerSpecs[0].Command[0] == "clobbered" {
		t.Fatal("builtins share the spec's command slice, so one caller's override reaches every later one")
	}
}

// A Runner is an exported struct with exported fields, so one can reach the
// launch path without the per-runner wiring. Refusing beats launching an agent
// with no MCP server and no hooks.
func TestAnUnregisteredRunnerIsRefusedRatherThanLaunchedBare(t *testing.T) {
	_, _, err := runnerLaunch(Runner{ID: "handrolled", Command: []string{"echo"}},
		"/bin/qrouton", t.TempDir(), EditorCommand{}, testHandle(), 1, false, "")
	if !errors.Is(err, ErrUnsupportedRunner) {
		t.Fatalf("runnerLaunch error = %v, want ErrUnsupportedRunner", err)
	}
}

// The same runner, unregistered, must not silently get a bare argv either.
func TestAnUnregisteredRunnerArgvIsJustItsCommand(t *testing.T) {
	r := Runner{ID: "handrolled", Command: []string{"echo", "--flag"}}
	if got := runnerArgv(r, false, modeRPI, ""); !reflect.DeepEqual(got, r.Command) {
		t.Fatalf("argv = %v, want the command untouched", got)
	}
}

// codexArgv is what the launch path hands Codex, with CODEX_HOME pointed at a
// config qrouton controls so the test reads its own depth rather than the
// developer's.
func codexArgv(t *testing.T, command []string, config string) []string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	if config != "" {
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := Runner{ID: runnerIDCodex, Label: runnerLabelCodex, Command: command}
	argv, _, err := runnerLaunch(r, "/bin/qrouton", t.TempDir(), EditorCommand{}, testHandle(), 1, false, "")
	if err != nil {
		t.Fatal(err)
	}
	return argv
}

// AGENTS.md bounds subagent depth at three levels: orchestrator, lead,
// specialist. Codex defaults to one, so a lead could not spawn anything —
// launch has to raise it before the runner starts.
func TestCodexIsLaunchedAtTheDepthALeadNeeds(t *testing.T) {
	argv := codexArgv(t, []string{runnerIDCodex, codexBypassSandboxFlag}, "")
	if got := codex.MaxDepth(argv); got != codex.RequiredMaxDepth {
		t.Fatalf("codex would run at depth %d, want %d\nargv: %v", got, codex.RequiredMaxDepth, argv)
	}
}

// A user who configured more nesting than qrouton needs keeps it: the injection
// raises a shallow default, it does not pin the setting.
func TestCodexKeepsADeeperConfiguredDepth(t *testing.T) {
	deeper := codex.RequiredMaxDepth + 2
	argv := codexArgv(t, []string{runnerIDCodex}, fmt.Sprintf("[agents]\nmax_depth = %d\n", deeper))
	if got := codex.MaxDepth(argv); got != deeper {
		t.Fatalf("configured depth %d became %d\nargv: %v", deeper, got, argv)
	}
}

// Same for a depth set in the user's own launch override, which reaches the
// injector as part of the command rather than through the config file.
func TestCodexKeepsADeeperDepthFromALaunchOverride(t *testing.T) {
	deeper := codex.RequiredMaxDepth + 1
	argv := codexArgv(t,
		[]string{runnerIDCodex, codex.ConfigFlag, codex.MaxDepthSetting(deeper)}, "")
	if got := codex.MaxDepth(argv); got != deeper {
		t.Fatalf("overridden depth %d became %d\nargv: %v", deeper, got, argv)
	}
}

// The depth setting is Codex's alone; the other runners take their nesting from
// their own defaults and must not be handed a -c they do not understand.
func TestOnlyCodexGetsTheDepthSetting(t *testing.T) {
	setting := codex.MaxDepthSetting(codex.RequiredMaxDepth)
	for _, id := range []string{runnerIDClaude, runnerIDOpenCode} {
		spec, ok := specFor(id)
		if !ok {
			t.Fatalf("no spec for %q", id)
		}
		r := Runner{ID: spec.ID, Label: spec.Label, Command: spec.Command}
		argv, _, err := runnerLaunch(r, "/bin/qrouton", t.TempDir(), EditorCommand{}, testHandle(), 1, false, "")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(argv, " "), setting) {
			t.Errorf("%s was handed %q", id, setting)
		}
	}
}
