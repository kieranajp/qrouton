package agentevent

import (
	"bytes"
	"testing"
)

func TestSubagentHooksAreStampedAndNamed(t *testing.T) {
	for _, input := range []string{
		`{"hook_event_name":"SubagentStart","agent_id":"agent-1","agent_type":"Explore","parent_agent_id":"lead-1"}`,
		`{"hook_event_name":"SubagentStop","agent_id":"agent-1","agent_type":"Explore"}`,
	} {
		event, hook, err := Record("claude", bytes.NewBufferString(input))
		if err != nil {
			t.Fatal(err)
		}
		if hook != event.HookEventName || hook == "" {
			t.Fatalf("Record(%s) reported hook %q for %#v", input, hook, event)
		}
		if event.Provider != "claude" || event.AgentID != "agent-1" || event.AgentType != "Explore" {
			t.Fatalf("Record(%s) = %#v, want the decoded identity", input, event)
		}
		// The hook itself carries no timestamp; the collector stamps one so the
		// workbench can order what it is sent.
		if event.Timestamp == "" {
			t.Fatalf("Record(%s) returned an unstamped event", input)
		}
	}
}

func TestSubagentStartKeepsItsParent(t *testing.T) {
	event, _, err := Record("claude", bytes.NewBufferString(
		`{"hook_event_name":"SubagentStart","agent_id":"agent-1","agent_type":"Explore","parent_agent_id":"lead-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if event.ParentID != "lead-1" {
		t.Fatalf("event = %#v, want parent lead-1", event)
	}
}

func TestCodexParentSessionBecomesTheLifecycleParent(t *testing.T) {
	event, _, err := Record("codex", bytes.NewBufferString(
		`{"session_id":"lead-1","hook_event_name":"SubagentStart","agent_id":"specialist-1","agent_type":"code-reviewer"}`))
	if err != nil {
		t.Fatal(err)
	}
	if event.Provider != "codex" || event.ParentID != "lead-1" || event.SessionID != "lead-1" {
		t.Fatalf("Codex event = %+v, want provider and parent session", event)
	}
}

// A hook outside the pair, and an event naming no agent, still report their
// name: the caller signals attention off that even where no lifecycle follows.
func TestUnstampedEventsAreStillNamed(t *testing.T) {
	for _, tc := range []struct{ name, input, hook string }{
		{"another hook", `{"hook_event_name":"Notification","agent_id":"agent-1"}`, "Notification"},
		{"no agent id", `{"hook_event_name":"SubagentStart","agent_type":"Explore"}`, HookSubagentStart},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event, hook, err := Record("claude", bytes.NewBufferString(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			if hook != tc.hook || event.HookEventName != tc.hook {
				t.Errorf("Record = %#v, hook %q, want hook %q", event, hook, tc.hook)
			}
			if event.Timestamp != "" {
				t.Errorf("event %#v was stamped, so a lifecycle would be derived from it", event)
			}
		})
	}
}

func TestMalformedEventIsAnError(t *testing.T) {
	if _, _, err := Record("claude", bytes.NewBufferString("not json")); err == nil {
		t.Fatal("expected a decode error")
	}
}
