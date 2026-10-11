package agentevent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// A result quoting the notification's own tags must not bleed into the
// fields before it.
func TestTaskNotificationKeepsFieldsWhenTheResultQuotesTags(t *testing.T) {
	prompt := "<task-notification>\n<task-id>a1</task-id>\n<tool-use-id>t1</tool-use-id>\n<status>completed</status>\n" +
		"<result>See <status>x</status> and </task-id> in fixtures.</result>\n" +
		"<usage><subagent_tokens>10</subagent_tokens></usage>\n</task-notification>"
	got := TaskNotifications(prompt)
	if len(got) != 1 || got[0].TaskID != "a1" || got[0].ToolUseID != "t1" || got[0].Outcome.Status != "completed" ||
		got[0].Result != "See <status>x</status> and </task-id> in fixtures." || got[0].Outcome.Tokens != 10 {
		t.Fatalf("notification = %+v", got)
	}
}

func TestReduceKeepsOnlySubagentBackgroundTasks(t *testing.T) {
	payload := `{"hook_event_name":"Stop","background_tasks":[` +
		`{"id":"a1","type":"subagent"},{"id":"b1","type":"shell"},{"id":"c1"}]}`
	event, err := Reduce("claude", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(event.BackgroundTasks, []string{"a1", "c1"}) {
		t.Fatalf("background tasks = %v", event.BackgroundTasks)
	}
}

func TestReduceBoundsIdsAndLists(t *testing.T) {
	long := strings.Repeat("x", 4*IDLimit)
	var tasks, notes []string
	for i := range 3 * ListLimit {
		tasks = append(tasks, fmt.Sprintf(`{"id":"%s%d","type":"subagent"}`, long, i))
		notes = append(notes, fmt.Sprintf("<task-notification><task-id>%s%d</task-id><status>%s</status><result>%s</result></task-notification>",
			long, i, long, strings.Repeat("é", MessageLimit)))
	}
	stop := fmt.Sprintf(`{"hook_event_name":"Stop","agent_id":%q,"background_tasks":[%s]}`, long, strings.Join(tasks, ","))
	prompt, _ := json.Marshal(map[string]string{"hook_event_name": "UserPromptSubmit", "prompt": strings.Join(notes, "\n")})
	for _, payload := range []string{stop, string(prompt)} {
		event, err := Reduce("claude", strings.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		if len([]rune(event.AgentID)) > IDLimit || len(event.BackgroundTasks) > ListLimit || len(event.Notifications) > ListLimit {
			t.Fatalf("%s kept %d runes of id, %d tasks, %d notifications", event.Hook,
				len([]rune(event.AgentID)), len(event.BackgroundTasks), len(event.Notifications))
		}
		for _, n := range event.Notifications {
			if len([]rune(n.TaskID)) > IDLimit || len([]rune(n.Outcome.Status)) > IDLimit {
				t.Fatalf("notification kept an unbounded id or status")
			}
		}
	}
}

func TestReduceCarriesAnInterrupt(t *testing.T) {
	event, err := Reduce("claude", strings.NewReader(`{"hook_event_name":"PostToolUseFailure","tool_name":"Bash","tool_use_id":"t1","error":"interrupted","is_interrupt":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !event.Interrupt {
		t.Fatalf("interrupt dropped: %+v", event)
	}
}

func TestCodexFixtureJoinsChildIdentityOnStart(t *testing.T) {
	body, err := os.ReadFile("testdata/codex-0.162.0/hooks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("testdata/codex-0.162.0/agent-identity.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	child := identityFile(t, string(metadata))
	count := 0
	for _, line := range bytes.Split(bytes.TrimSpace(body), []byte("\n")) {
		original, hook, err := Record("codex", bytes.NewReader(line))
		if err != nil {
			t.Fatal(err)
		}
		if original.TaskName != "" {
			t.Fatalf("unavailable original fixture acquired a name: %+v", original)
		}
		if hook != HookSubagentStart {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(line, &payload); err != nil {
			t.Fatal(err)
		}
		payload["transcript_path"] = child
		rewritten, _ := json.Marshal(payload)
		event, _, err := Record("codex", bytes.NewReader(rewritten))
		if err != nil || event.AgentID != original.AgentID || event.TaskName != "/root/plan_codex_agent_names" {
			t.Fatalf("%s = %+v, err %v", hook, event, err)
		}
		count++
	}
	if count != 1 {
		t.Fatalf("decoded %d lifecycle events", count)
	}
}
