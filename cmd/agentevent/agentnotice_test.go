package agentevent

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/urfave/cli/v2"
)

func runNotice(t *testing.T, root, payload string) string {
	t.Helper()
	feedStdin(t, payload)
	var out bytes.Buffer
	app := &cli.App{Commands: []*cli.Command{NoticeCommand}, Writer: &out}
	if err := app.Run([]string{"qrouton", noticeCommandName, "--" + sessionRootFlag, root}); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// A subagent taking the notice would hide it from the orchestrator, which is
// the agent that re-reads qrouton.json and routes the work.
func TestTheNoticeCommandSkipsSubagentEvents(t *testing.T) {
	root := t.TempDir()
	if err := session.QueueAgentNotice(root, "added org/svc"); err != nil {
		t.Fatal(err)
	}
	subagent := `{"session_id":"01a1","agent_id":"01a2","agent_type":"default","hook_event_name":"PostToolUse","tool_name":"Bash"}`
	if out := runNotice(t, root, subagent); out != "" {
		t.Fatalf("subagent event printed %q", out)
	}
	if notices, err := session.TakeAgentNotices(root); err != nil || len(notices) != 1 {
		t.Fatalf("queue after a subagent event = %q, %v; want the notice still there", notices, err)
	}
}

// A runner reads the context under the event that fired the hook, so the output
// names the event it was given.
func TestTheNoticeCommandEchoesTheHookEvent(t *testing.T) {
	for _, event := range []string{"PostToolUse", "UserPromptSubmit"} {
		t.Run(event, func(t *testing.T) {
			root := t.TempDir()
			for _, notice := range []string{"added org/svc", "added org/web"} {
				if err := session.QueueAgentNotice(root, notice); err != nil {
					t.Fatal(err)
				}
			}
			out := runNotice(t, root, `{"session_id":"01a1","hook_event_name":"`+event+`"}`)
			var got noticeOutput
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("output %q: %v", out, err)
			}
			if got.HookSpecificOutput.HookEventName != event ||
				got.HookSpecificOutput.AdditionalContext != "added org/svc\n\nadded org/web" {
				t.Fatalf("output = %+v", got)
			}
			if out := runNotice(t, root, `{"hook_event_name":"`+event+`"}`); out != "" {
				t.Fatalf("an empty queue printed %q", out)
			}
		})
	}
}
