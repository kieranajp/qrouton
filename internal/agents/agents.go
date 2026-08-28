package agents

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type Event struct {
	HookEventName string `json:"hook_event_name"`
	AgentID       string `json:"agent_id"`
	AgentType     string `json:"agent_type"`
	ParentID      string `json:"parent_agent_id,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}

// RecordEvent appends one Claude subagent hook event read from input to the
// session's log and returns the decoded live event even if the append fails.
func RecordEvent(root string, input io.Reader) (Event, string, error) {
	var event Event
	if err := json.NewDecoder(input).Decode(&event); err != nil {
		return Event{}, "", err
	}
	if event.AgentID == "" || (event.HookEventName != hookSubagentStart && event.HookEventName != hookSubagentStop) {
		return event, event.HookEventName, nil
	}
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(event)
	if err != nil {
		return event, event.HookEventName, err
	}
	f, err := os.OpenFile(sessionpaths.ClaudeAgentLog(root), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return event, event.HookEventName, err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return event, event.HookEventName, err
}
