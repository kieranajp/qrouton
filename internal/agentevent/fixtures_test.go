package agentevent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/workbench"
)

const claudeFixtures = "testdata/claude-2.1.286"

// reduced replays one recorded spike through Reduce, in recorded order.
func reduced(t *testing.T, name string) []workbench.AgentEventRequest {
	t.Helper()
	file, err := os.Open(filepath.Join(claudeFixtures, name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var out []workbench.AgentEventRequest
	for scanner.Scan() {
		event, err := Reduce("claude", bytes.NewReader(scanner.Bytes()))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out = append(out, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func find(events []workbench.AgentEventRequest, match func(workbench.AgentEventRequest) bool) []workbench.AgentEventRequest {
	var out []workbench.AgentEventRequest
	for _, event := range events {
		if match(event) {
			out = append(out, event)
		}
	}
	return out
}

func agentCalls(hook string) func(workbench.AgentEventRequest) bool {
	return func(e workbench.AgentEventRequest) bool { return e.Hook == hook && e.Tool == ToolAgent }
}

func TestReduceNamesTheSpawnedChild(t *testing.T) {
	for run, child := range map[string]string{
		"run1-hooks":    "a3068ab302d038a35",
		"run3-fg-hooks": "ab76fea07a9b71550",
		"run4-bg-hooks": "a5438536af5bc5c5e",
	} {
		posts := find(reduced(t, run), agentCalls(HookPostToolUse))
		if len(posts) != 1 || posts[0].ChildID != child || posts[0].Spawn == nil || posts[0].Spawn.Type != "spike-a" {
			t.Fatalf("%s Agent PostToolUse = %+v, want child %s", run, posts, child)
		}
	}
}

func TestReduceKeepsTheCallerOfADepthTwoSpawn(t *testing.T) {
	events := reduced(t, "run2-hooks")
	for _, hook := range []string{HookPreToolUse, HookPostToolUse} {
		var nested []workbench.AgentEventRequest
		for _, e := range find(events, agentCalls(hook)) {
			if e.AgentID != "" {
				nested = append(nested, e)
			}
		}
		if len(nested) != 1 || nested[0].AgentID != "a08c4b226d2052caf" || nested[0].AgentType != "spike-b" {
			t.Fatalf("depth-2 %s = %+v", hook, nested)
		}
	}
}

func TestReduceReadsAForegroundOutcome(t *testing.T) {
	post := find(reduced(t, "run3-fg-hooks"), agentCalls(HookPostToolUse))[0]
	if post.Outcome == nil || post.Outcome.Status != "completed" || post.Outcome.Tokens != 42191 ||
		post.Outcome.ToolUses != 1 || post.Outcome.DurationMs != 2878 || post.Message != "Done. Output was `fg`." {
		t.Fatalf("foreground outcome = %+v message %q", post.Outcome, post.Message)
	}
}

func TestReduceLeavesABackgroundLaunchWithoutAnOutcome(t *testing.T) {
	post := find(reduced(t, "run4-bg-hooks"), agentCalls(HookPostToolUse))[0]
	if post.Outcome != nil || !post.Spawn.Async {
		t.Fatalf("background launch = %+v", post)
	}
}

func TestReduceParsesATaskNotification(t *testing.T) {
	prompts := find(reduced(t, "run4-bg-hooks"), func(e workbench.AgentEventRequest) bool {
		return e.Hook == HookUserPromptSubmit && len(e.Notifications) > 0
	})
	if len(prompts) != 1 || len(prompts[0].Notifications) != 1 {
		t.Fatalf("task notifications = %+v", prompts)
	}
	n := prompts[0].Notifications[0]
	if n.TaskID != "a5438536af5bc5c5e" || n.ToolUseID != "toolu_01U5J7tJrjavw51MmDpS5f3t" ||
		n.Outcome.Status != "completed" || n.Outcome.Tokens != 40927 || n.Outcome.ToolUses != 1 ||
		n.Outcome.DurationMs != 9040 || !strings.HasPrefix(n.Result, "Done.") {
		t.Fatalf("notification = %+v", n)
	}
}

func TestReduceListsBackgroundTasksAtStop(t *testing.T) {
	stops := find(reduced(t, "run4-bg-hooks"), func(e workbench.AgentEventRequest) bool { return e.Hook == HookStop })
	if len(stops) != 2 || len(stops[0].BackgroundTasks) != 1 || stops[0].BackgroundTasks[0] != "a5438536af5bc5c5e" ||
		len(stops[1].BackgroundTasks) != 0 {
		t.Fatalf("stops = %+v", stops)
	}
}

func TestReducedEventsStaySmall(t *testing.T) {
	entries, err := os.ReadDir(claudeFixtures)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		for _, event := range reduced(t, name) {
			body, _ := json.Marshal(event)
			if len(body) > 4<<10 {
				t.Fatalf("%s %s reduced to %d bytes", name, event.Hook, len(body))
			}
		}
	}
}

func TestTargetPicksOneThingPerTool(t *testing.T) {
	for _, tc := range []struct{ tool, input, want string }{
		{"Read", `{"file_path":"/w/a.go"}`, "/w/a.go"},
		{"Edit", `{"file_path":"/w/b.go","old_string":"x"}`, "/w/b.go"},
		{"NotebookEdit", `{"notebook_path":"/w/n.ipynb"}`, "/w/n.ipynb"},
		{"Bash", `{"command":"go test ./...\necho done"}`, "go test ./..."},
		{"Grep", `{"pattern":"func Reduce","path":"/w"}`, "func Reduce"},
		{"Glob", `{"pattern":"**/*.go"}`, "**/*.go"},
		{"WebFetch", `{"url":"https://example.com"}`, "https://example.com"},
		{"Agent", `{"description":"Trace hooks","prompt":"long"}`, "Trace hooks"},
		{"mcp__qrouton__run_command", `{"command":"make"}`, "qrouton · run_command"},
		{"TodoWrite", `{"todos":[]}`, ""},
		{"AskUserQuestion", `{"questions":[{"question":"Which branch?","options":[]}]}`, "Which branch?"},
	} {
		if got := Target(tc.tool, json.RawMessage(tc.input)); got != tc.want {
			t.Errorf("Target(%s) = %q, want %q", tc.tool, got, tc.want)
		}
	}
}

func TestReduceTruncatesLongText(t *testing.T) {
	long := strings.Repeat("é", MessageLimit+50)
	payload, _ := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse", "tool_name": "Bash",
		"tool_input": map[string]string{"command": long},
	})
	event, err := Reduce("claude", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(event.Target)); n != TargetLimit {
		t.Fatalf("target is %d runes, want %d", n, TargetLimit)
	}
	payload, _ = json.Marshal(map[string]any{"hook_event_name": "Stop", "last_assistant_message": long})
	if event, _ = Reduce("claude", bytes.NewReader(payload)); len([]rune(event.Message)) != MessageLimit {
		t.Fatalf("message is %d runes", len([]rune(event.Message)))
	}
}

func TestReduceKeepsFailureText(t *testing.T) {
	for _, payload := range []string{
		`{"hook_event_name":"StopFailure","error":"rate_limit","error_details":"429 from the API"}`,
		`{"hook_event_name":"PostToolUseFailure","tool_name":"Bash","tool_use_id":"t1","error":"rate_limit: 429 from the API"}`,
	} {
		event, err := Reduce("claude", strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		if event.Message != "rate_limit: 429 from the API" {
			t.Fatalf("%s message = %q", event.Hook, event.Message)
		}
	}
}

func TestCodexFixtureStillDecodesAsLifecycle(t *testing.T) {
	body, err := os.ReadFile("testdata/codex-0.162.0/hooks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	starts := 0
	for _, line := range bytes.Split(bytes.TrimSpace(body), []byte("\n")) {
		event, hook, err := Record("codex", bytes.NewReader(line))
		if err != nil {
			t.Fatal(err)
		}
		if hook == HookSubagentStart && event.AgentID != "" && event.ParentID != "" {
			starts++
		}
	}
	if starts == 0 {
		t.Fatal("no Codex SubagentStart carried an agent and a parent")
	}
}
