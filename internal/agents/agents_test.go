package agents

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// recorded is the log as the collector left it, one decoded event per line.
func recorded(t *testing.T, root string) []Event {
	t.Helper()
	f, err := os.Open(sessionpaths.ClaudeAgentLog(root))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("log line %q does not decode: %v", scanner.Text(), err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func session(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, sessionpaths.DirName), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestClaudeAgentHooksAppendOneStampedLineEach(t *testing.T) {
	root := session(t)
	for _, input := range []string{
		`{"hook_event_name":"SubagentStart","agent_id":"agent-1","agent_type":"Explore","parent_agent_id":"lead-1"}`,
		`{"hook_event_name":"SubagentStop","agent_id":"agent-1","agent_type":"Explore"}`,
	} {
		event, hook, err := RecordEvent(root, bytes.NewBufferString(input))
		if err != nil {
			t.Fatal(err)
		}
		// cmd/agents maps the returned hook name to an attention state, so it
		// has to survive a successful write as well as a failed one.
		if hook == "" {
			t.Fatalf("RecordEvent(%s) reported no hook name", input)
		}
		if event.AgentID != "agent-1" || event.Timestamp == "" {
			t.Fatalf("RecordEvent(%s) = %#v, want the stamped live event", input, event)
		}
	}

	events := recorded(t, root)
	if len(events) != 2 {
		t.Fatalf("recorded %d events, want the start and the stop: %#v", len(events), events)
	}
	if events[0].HookEventName != hookSubagentStart || events[1].HookEventName != hookSubagentStop {
		t.Errorf("recorded hooks = %q, %q", events[0].HookEventName, events[1].HookEventName)
	}
	if events[0].ParentID != "lead-1" {
		t.Fatalf("recorded start lost its parent: %#v", events[0])
	}
	for i, event := range events {
		if event.AgentID != "agent-1" || event.AgentType != "Explore" {
			t.Errorf("event %d lost its identity: %#v", i, event)
		}
		// The hook itself carries no timestamp; the collector stamps one so the
		// log is orderable without depending on file offsets.
		if event.Timestamp == "" {
			t.Errorf("event %d was recorded unstamped", i)
		}
	}
}

// A hook qrouton does not collect, and an event naming no agent, still report
// their name: the caller signals attention off that even when nothing is logged.
func TestUncollectedEventsAreNamedButNotRecorded(t *testing.T) {
	for _, tc := range []struct{ name, input, hook string }{
		{"another hook", `{"hook_event_name":"Notification","agent_id":"agent-1"}`, "Notification"},
		{"no agent id", `{"hook_event_name":"SubagentStart","agent_type":"Explore"}`, hookSubagentStart},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := session(t)
			event, hook, err := RecordEvent(root, bytes.NewBufferString(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if hook != tc.hook {
				t.Errorf("hook = %q, want %q", hook, tc.hook)
			}
			if event.HookEventName != tc.hook {
				t.Errorf("event = %#v, want hook %q", event, tc.hook)
			}
			if events := recorded(t, root); len(events) != 0 {
				t.Errorf("recorded %#v, want nothing", events)
			}
		})
	}
}

func TestMalformedEventIsAnError(t *testing.T) {
	root := session(t)
	if _, _, err := RecordEvent(root, bytes.NewBufferString("not json")); err == nil {
		t.Fatal("expected a decode error")
	}
	if events := recorded(t, root); len(events) != 0 {
		t.Errorf("recorded %#v, want nothing", events)
	}
}
