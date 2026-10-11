package launch

import (
	"context"
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
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
	"go.yaml.in/yaml/v3"
)

func reviewFixture(t *testing.T, sharedLink bool) (string, *config.Config, workbench.Handle, ReviewInput) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	thoughts := filepath.Join(root, "thoughts")
	if sharedLink {
		target := t.TempDir()
		if err := os.Symlink(target, thoughts); err != nil {
			t.Fatal(err)
		}
	}
	plan := filepath.Join(thoughts, "shared", "plans", "plan.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan, []byte("# Plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "src", "example")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	reviewGitFixture(t, repo, "init", "-b", "main")
	reviewGitFixture(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "--allow-empty", "-m", "base")
	reviewGitFixture(t, repo, "checkout", "-b", "implementation")
	reviewGitFixture(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "--allow-empty", "-m", "change")
	manifest := session.Manifest{Repos: []session.ManifestRepo{{Name: "example", Role: session.RepoRoleEditing, DefaultBranch: "main", WorktreePath: "src/example"}}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.Manifest(root), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return root, &config.Config{Reviewer: &config.Reviewer{Harness: "codex", Model: "model", Reasoning: "medium"}}, workbench.Handle{Socket: "/tmp/review.sock", SessionRoot: root}, ReviewInput{Plan: "thoughts/shared/plans/plan.md", Brief: "Review the completed change.", Verification: "Targeted tests passed."}
}

func reviewGitFixture(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return strings.TrimSpace(string(out))
}

func readReviewRequest(t *testing.T, path string) reviewRequest {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var request reviewRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	return request
}

func TestDisabledReviewerHasNoSideEffects(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Launch: map[string][]string{"broken": nil}}
	if state := ReviewerSettings(cfg); state.Enabled || state.Available || state.Settings != nil || state.Error != "" {
		t.Fatalf("None = %+v", state)
	}
	_, err := PrepareReview(context.Background(), cfg, root, "", workbench.Handle{}, EditorCommand{}, ReviewInput{})
	if !errors.Is(err, ErrReviewerDisabled) {
		t.Fatalf("disabled = %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("disabled wrote artifacts: %v %v", entries, err)
	}
}

func TestPrepareReviewPinsCommitsAndResolvesSharedPlan(t *testing.T) {
	installedReviewRunners(t)
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			root, cfg, handle, input := reviewFixture(t, shared)
			for _, file := range []string{"AGENTS.md", "CLAUDE.md", ".qrouton/agent.pid", ".qrouton/agent-notice"} {
				path := filepath.Join(root, file)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("unchanged"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			prepared, err := PrepareReview(context.Background(), cfg, root, "/bin/qrouton", handle, EditorCommand{Argv: []string{"editor", "{path}"}}, input)
			if err != nil {
				t.Fatal(err)
			}
			request := readReviewRequest(t, prepared.RequestPath)
			if request.Settings != *cfg.Reviewer || request.Handle != handle || !slices.Equal(request.Editor.Argv, []string{"editor", "{path}"}) {
				t.Fatalf("snapshot lost wiring: %+v", request)
			}
			repo := filepath.Join(root, "src", "example")
			if len(request.Repositories) != 1 || request.Repositories[0].Base != reviewGitFixture(t, repo, "rev-parse", "main") || request.Repositories[0].Head != reviewGitFixture(t, repo, "rev-parse", "HEAD") {
				t.Fatalf("unpinned scope: %+v", request.Repositories)
			}
			raw, err := os.ReadFile(prepared.ReportPath)
			if err != nil {
				t.Fatal(err)
			}
			var metadata struct{ Kind, Title, Plan string }
			parts := strings.SplitN(string(raw), "---", 3)
			if len(parts) != 3 {
				t.Fatal("missing frontmatter")
			}
			if err := yaml.Unmarshal([]byte(parts[1]), &metadata); err != nil {
				t.Fatal(err)
			}
			actual, err := filepath.EvalSymlinks(filepath.Join(filepath.Dir(prepared.ReportPath), metadata.Plan))
			if err != nil || actual != request.Plan || metadata.Kind != "review" || metadata.Title == "" {
				t.Fatalf("invalid plan reference: %+v %q %v", metadata, actual, err)
			}
			if !strings.Contains(string(raw), "[Plan](<"+metadata.Plan+">)") || !strings.Contains(string(raw), "No completed review has been recorded") {
				t.Fatalf("untruthful placeholder: %s", raw)
			}
			if prepared.Window.Select || prepared.Window.CloseOnExit || prepared.Window.Kind != workbench.KindTerminal || prepared.Window.Cwd != sessionpaths.ReviewRun(root, prepared.RunID) || !slices.Equal(prepared.Window.Command, []string{"/bin/qrouton", "reviewer", "--request", prepared.RequestPath}) {
				t.Fatalf("window = %+v", prepared.Window)
			}
			info, err := os.Stat(filepath.Dir(prepared.RequestPath))
			if err != nil || info.Mode().Perm() != 0o700 {
				t.Fatalf("run directory permissions: %v %v", info, err)
			}
			info, err = os.Stat(prepared.RequestPath)
			if err != nil || info.Mode().Perm() != 0o400 {
				t.Fatalf("request permissions: %v %v", info, err)
			}
			for _, file := range []string{"AGENTS.md", "CLAUDE.md", ".qrouton/agent.pid", ".qrouton/agent-notice"} {
				raw, err := os.ReadFile(filepath.Join(root, file))
				if err != nil || string(raw) != "unchanged" {
					t.Fatalf("changed %s", file)
				}
			}
			next, err := PrepareReview(context.Background(), cfg, root, "/bin/qrouton", handle, EditorCommand{}, input)
			if err != nil {
				t.Fatal(err)
			}
			if next.RunID == prepared.RunID || next.ReportPath == prepared.ReportPath {
				t.Fatal("reused review run")
			}
		})
	}
}

func TestReviewScopeRejectsInvalidTargetsBeforeAllocation(t *testing.T) {
	installedReviewRunners(t)
	for _, tc := range []string{"missing", "unknown", "ambiguous", "outside", "canceled"} {
		t.Run(tc, func(t *testing.T) {
			root, cfg, handle, input := reviewFixture(t, false)
			ctx := context.Background()
			switch tc {
			case "missing":
				input.Bases = map[string]string{"example": "absent"}
			case "unknown":
				input.Bases = map[string]string{"reference": "main"}
			case "ambiguous":
				reviewGitFixture(t, filepath.Join(root, "src", "example"), "tag", "main")
			case "outside":
				input.Plan = filepath.Join(t.TempDir(), "plan.md")
				if err := os.WriteFile(input.Plan, []byte("plan"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "canceled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			_, err := PrepareReview(ctx, cfg, root, "/bin/qrouton", handle, EditorCommand{}, input)
			if err == nil {
				t.Fatal("invalid request accepted")
			}
			if _, err := os.Stat(sessionpaths.ReviewRuns(root)); !os.IsNotExist(err) {
				t.Fatal("invalid request allocated run")
			}
			if _, err := os.Stat(sessionpaths.Reviews(root)); !os.IsNotExist(err) {
				t.Fatal("invalid request allocated report")
			}
		})
	}
}

func TestExplicitReviewBaseAndSnapshotSurviveConfigurationChanges(t *testing.T) {
	installedReviewRunners(t)
	root, cfg, handle, input := reviewFixture(t, false)
	input.Bases = map[string]string{"example": "HEAD"}
	cfg.Launch = map[string][]string{"codex": {"custom-codex", "--search"}}
	prepared, err := PrepareReview(context.Background(), cfg, root, "/bin/qrouton", handle, EditorCommand{}, input)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Launch["codex"][0] = "changed"
	cfg.Reviewer = nil
	request := readReviewRequest(t, prepared.RequestPath)
	if request.Settings.Harness != "codex" || request.Command[0] != "custom-codex" || request.Repositories[0].Base != request.Repositories[0].Head {
		t.Fatalf("snapshot changed: %+v", request)
	}
}

func TestReviewProvidersReceiveFreshRoleWithoutConversationHooks(t *testing.T) {
	root := sessionWithOwnHome(t)
	t.Setenv(agentevent.GenerationEnvVar, "42")
	t.Setenv(agentevent.ProviderEnvVar, "conversation")
	t.Setenv(openCodeConfigEnvVar, `{"permission":{"edit":"deny"},"agent":{"mine":{"prompt":"own"}},"mcp":{"other":{"type":"local","command":["other"]}},"theme":"custom"}`)
	role := "Review role\nquotes \" slash \\ unicode ∀"
	for _, runner := range builtinRunners {
		t.Run(runner.ID, func(t *testing.T) {
			model := "model"
			effort := "medium"
			if runner.ID == "opencode" {
				model = "provider/model"
				effort = ""
			}
			request := reviewRequest{RunID: "run", Root: root, Bin: "/bin/qrouton", Settings: config.Reviewer{Harness: runner.ID, Model: model, Reasoning: effort}, Command: runner.Command, Override: true, Handle: testHandle(), Role: role, Task: "review task"}
			argv, env, err := reviewerLaunch(request)
			if err != nil {
				t.Fatal(err)
			}
			joined := strings.Join(argv, "\n")
			for _, absent := range []string{"SubagentStart", "SubagentStop", "agent-event", openingMessageRPI, "--continue", "--name"} {
				if strings.Contains(joined, absent) {
					t.Fatalf("%s present in %v", absent, argv)
				}
			}
			if slices.Contains(env, agentevent.GenerationEnvVar+"=42") || slices.Contains(env, agentevent.ProviderEnvVar+"=conversation") {
				t.Fatalf("conversation identity retained: %v", env)
			}
			if !slices.Contains(argv, model) {
				t.Fatalf("model missing: %v", argv)
			}
			switch runner.ID {
			case "codex":
				if !slices.Contains(argv, codexDeveloperInstructionsKey+quotedConfigString(role)) || !slices.Contains(argv, codexReasoningKey+quotedConfigString(effort)) {
					t.Fatalf("role/effort lost: %v", argv)
				}
			case "claude":
				if !slices.Contains(argv, role) || !slices.Contains(argv, effort) {
					t.Fatalf("role/effort lost: %v", argv)
				}
			case "opencode":
				var content map[string]any
				for _, item := range env {
					if value, ok := strings.CutPrefix(item, openCodeConfigEnvVar+"="); ok {
						if err := json.Unmarshal([]byte(value), &content); err != nil {
							t.Fatal(err)
						}
					}
				}
				agents := content["agent"].(map[string]any)
				if content["theme"] != "custom" || agents["mine"] == nil || agents[reviewerAgentName].(map[string]any)["prompt"] != role || content["permission"].(map[string]any)["edit"] != "deny" || content["mcp"].(map[string]any)["other"] == nil {
					t.Fatalf("merge lost config: %+v", content)
				}
			case "agy":
				if !slices.Contains(argv, role+noticeSeparator+"review task") || !slices.Contains(argv, sessionpaths.ReviewAgyHome(root, "run")) {
					t.Fatalf("agy scope lost: %v", argv)
				}
				if _, err := os.Stat(sessionpaths.AgyHome(root)); !os.IsNotExist(err) {
					t.Fatal("conversation shim created")
				}
			}
		})
	}
}

func TestReviewChildProcess(t *testing.T) {
	path := os.Getenv("QROUTON_TEST_REVIEW_REQUEST")
	if path == "" {
		return
	}
	if err := RunReviewer(path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestReviewChildExecutesSnapshotAndRechecksAvailability(t *testing.T) {
	installedReviewRunners(t)
	root, cfg, handle, input := reviewFixture(t, false)
	provider := filepath.Join(root, "provider")
	output := filepath.Join(root, "argv.json")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + ShellQuote(output) + "\n"
	if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.Launch = map[string][]string{"codex": {provider, "--search"}}
	prepared, err := PrepareReview(context.Background(), cfg, root, "/bin/qrouton", handle, EditorCommand{}, input)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Reviewer = nil
	run := func() ([]byte, error) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestReviewChildProcess$")
		cmd.Env = append(os.Environ(), "QROUTON_TEST_REVIEW_REQUEST="+prepared.RequestPath)
		return cmd.CombinedOutput()
	}
	if out, err := run(); err != nil {
		t.Fatalf("child: %s %v", out, err)
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "--search\n") || !strings.Contains(string(raw), "model_reasoning_effort=\"medium\"") || strings.Contains(string(raw), "agent-event") {
		t.Fatalf("snapshot argv: %s", raw)
	}
	before := readReviewRequest(t, prepared.RequestPath)
	if err := os.Remove(provider); err != nil {
		t.Fatal(err)
	}
	if out, err := run(); err == nil || !strings.Contains(string(out), ErrRunnerUnavailable.Error()) {
		t.Fatalf("removed provider = %s %v", out, err)
	}
	if after := readReviewRequest(t, prepared.RequestPath); !reflect.DeepEqual(before, after) {
		t.Fatal("launch mutated request")
	}
	raw, err = os.ReadFile(prepared.ReportPath)
	if err != nil || !strings.Contains(string(raw), "No completed review") {
		t.Fatalf("failure changed placeholder: %s %v", raw, err)
	}
}

func TestReviewDoesNotReplaceConversationResumeHistory(t *testing.T) {
	installedReviewRunners(t)
	t.Setenv("HOME", t.TempDir())
	for _, builtin := range builtinRunners {
		t.Run(builtin.ID, func(t *testing.T) {
			root, cfg, handle, input := reviewFixture(t, false)
			provider := filepath.Join(t.TempDir(), "provider")
			script := `#!/bin/sh
resume=false
review=false
history="$PWD/.provider-history"
if [ -n "$OPENCODE_DB" ]; then history="$OPENCODE_DB"; fi
previous=
for arg do
if [ "$previous" = --gemini_dir ]; then history="$arg/antigravity-cli/.provider-history"; fi
previous="$arg"
case "$arg" in --continue|resume) resume=true;; --model) review=true;; esac
done
mkdir -p "$(dirname "$history")"
if $resume; then cat "$history"; elif $review; then printf review > "$history"; else printf conversation > "$history"; fi
`
			if err := os.WriteFile(provider, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			model := "model"
			if builtin.ID == "opencode" {
				model = "provider/model"
			}
			cfg.Reviewer = &config.Reviewer{Harness: builtin.ID, Model: model}
			cfg.Launch = map[string][]string{builtin.ID: {provider}}
			conversation := builtin
			conversation.Command, conversation.Override = []string{provider}, true
			runConversation := func(resume bool) string {
				argv, env, err := runnerLaunch(conversation, "/bin/qrouton", root, EditorCommand{}, handle, 1, resume, "")
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(argv[0], argv[1:]...)
				cmd.Dir, cmd.Env = root, env
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("conversation: %s %v", out, err)
				}
				return string(out)
			}
			runConversation(false)
			prepared, err := PrepareReview(context.Background(), cfg, root, "/bin/qrouton", handle, EditorCommand{}, input)
			if err != nil {
				t.Fatal(err)
			}
			request := readReviewRequest(t, prepared.RequestPath)
			if !strings.Contains(request.Task, "Session root: "+root) {
				t.Fatal("review lost session context")
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestReviewChildProcess$")
			cmd.Env = append(os.Environ(), "QROUTON_TEST_REVIEW_REQUEST="+prepared.RequestPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("review: %s %v", out, err)
			}
			historyPath := filepath.Join(prepared.Window.Cwd, ".provider-history")
			if builtin.ID == "opencode" {
				historyPath = sessionpaths.ReviewOpenCodeDatabase(root, prepared.RunID)
			} else if builtin.ID == "agy" {
				historyPath = filepath.Join(sessionpaths.ReviewAgyHome(root, prepared.RunID), agyCLIStateDirName, ".provider-history")
			}
			history, err := os.ReadFile(historyPath)
			if err != nil || string(history) != "review" {
				t.Fatalf("review history = %s %v", history, err)
			}
			if got := runConversation(true); got != "conversation" {
				t.Fatalf("resumed %q", got)
			}
		})
	}
}

func TestReviewCommandKeepsRelativeOverridePaths(t *testing.T) {
	root := t.TempDir()
	command := []string{"./provider", "--settings", "settings.json", "--mcp-config", "a.json", "b.json", "--plugin-dir", "plugins", "--mcp-config=c.json", "d.json", "--mcp-config={\"mcpServers\":{}}", "--debug-file=logs/debug.log", "--search"}
	got := reviewCommand(command, claudeReviewerOverrides, root)
	want := []string{filepath.Join(root, "provider"), "--settings", filepath.Join(root, "settings.json"), "--mcp-config", filepath.Join(root, "a.json"), filepath.Join(root, "b.json"), "--plugin-dir", filepath.Join(root, "plugins"), "--mcp-config=" + filepath.Join(root, "c.json"), filepath.Join(root, "d.json"), "--mcp-config={\"mcpServers\":{}}", "--debug-file=" + filepath.Join(root, "logs/debug.log"), "--search"}
	if !slices.Equal(got, want) || command[0] != "./provider" || command[2] != "settings.json" {
		t.Fatalf("command = %v; source = %v", got, command)
	}
}

func TestAgyReviewerSharesSettingsWithPrivateHistory(t *testing.T) {
	root := sessionWithOwnHome(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	userState := filepath.Join(home, geminiDirName, agyCLIStateDirName)
	writeTree(t, map[string]string{
		filepath.Join(userState, "settings.json"):                      "settings",
		filepath.Join(userState, "keybindings.json"):                   "keys",
		filepath.Join(userState, "conversations", "conversation.json"): "conversation",
		filepath.Join(userState, "conversation_summaries.db"):          "summaries",
		filepath.Join(userState, "history.jsonl"):                      "history",
	})
	request := reviewRequest{RunID: "run", Root: root, Bin: "/bin/qrouton", Settings: config.Reviewer{Harness: "agy", Model: "model"}, Command: agyRunner(t).Command, Handle: testHandle(), Role: "role", Task: "task"}
	if _, _, err := reviewerLaunch(request); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(sessionpaths.ReviewAgyHome(root, "run"), agyCLIStateDirName)
	if isLink(t, state) || !isLink(t, filepath.Join(state, "settings.json")) || !isLink(t, filepath.Join(state, "keybindings.json")) {
		t.Fatal("reviewer settings or state ownership changed")
	}
	for _, name := range []string{"conversations", "conversation_summaries.db", "history.jsonl"} {
		if _, err := os.Lstat(filepath.Join(state, name)); !os.IsNotExist(err) {
			t.Fatalf("reviewer shares %s: %v", name, err)
		}
	}
}

func TestReviewScopeSupportsSameNamesUnderDifferentOwners(t *testing.T) {
	installedReviewRunners(t)
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			root, cfg, handle, input := reviewFixture(t, false)
			first := filepath.Join(root, "src", "example")
			second := filepath.Join(root, "src", "second", "example")
			if err := os.MkdirAll(second, 0o755); err != nil {
				t.Fatal(err)
			}
			reviewGitFixture(t, second, "init", "-b", "main")
			reviewGitFixture(t, second, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "--allow-empty", "-m", "other base")
			reviewGitFixture(t, second, "checkout", "-b", "implementation")
			reviewGitFixture(t, second, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "--allow-empty", "-m", "other change")
			manifest := session.Manifest{Repos: []session.ManifestRepo{
				{Org: "first", Name: "example", Role: session.RepoRoleEditing, DefaultBranch: "main", WorktreePath: "src/example"},
				{Org: "second", Name: "example", Role: session.RepoRoleEditing, DefaultBranch: "main", WorktreePath: "src/second/example"},
			}}
			raw, _ := json.Marshal(manifest)
			if err := os.WriteFile(sessionpaths.Manifest(root), raw, 0o644); err != nil {
				t.Fatal(err)
			}
			if explicit {
				input.Bases = map[string]string{"first/example": "HEAD", "second/example": "main"}
			}
			prepared, err := PrepareReview(context.Background(), cfg, root, "/bin/qrouton", handle, EditorCommand{}, input)
			if err != nil {
				t.Fatal(err)
			}
			scope := readReviewRequest(t, prepared.RequestPath).Repositories
			if len(scope) != 2 || scope[0].Name != "first/example" || scope[1].Name != "second/example" {
				t.Fatalf("scope = %+v", scope)
			}
			expectedFirst := "main"
			if explicit {
				expectedFirst = "HEAD"
			}
			if scope[0].Base != reviewGitFixture(t, first, "rev-parse", expectedFirst) || scope[1].Base != reviewGitFixture(t, second, "rev-parse", "main") || scope[0].Head != reviewGitFixture(t, first, "rev-parse", "HEAD") || scope[1].Head != reviewGitFixture(t, second, "rev-parse", "HEAD") {
				t.Fatalf("wrong commits = %+v", scope)
			}
			report, err := os.ReadFile(prepared.ReportPath)
			if err != nil || !strings.Contains(string(report), "first/example") || !strings.Contains(string(report), "second/example") {
				t.Fatalf("report = %s %v", report, err)
			}
			input.Bases = map[string]string{"example": "main"}
			if _, err := PrepareReview(context.Background(), cfg, root, "/bin/qrouton", handle, EditorCommand{}, input); !errors.Is(err, ErrReviewScope) {
				t.Fatalf("ambiguous bare base = %v", err)
			}
		})
	}
}
