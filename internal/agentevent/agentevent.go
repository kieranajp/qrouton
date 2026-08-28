package agentevent

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type Event struct {
	Provider      string `json:"provider"`
	SessionID     string `json:"session_id,omitempty"`
	HookEventName string `json:"hook_event_name"`
	AgentID       string `json:"agent_id"`
	AgentType     string `json:"agent_type"`
	ParentID      string `json:"parent_agent_id,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}

// Record appends one subagent hook event read from input to the session's log
// and returns the decoded live event even if the append fails.
func Record(root, provider string, input io.Reader) (Event, string, error) {
	var event Event
	if err := json.NewDecoder(input).Decode(&event); err != nil {
		return Event{}, "", err
	}
	event.Provider = provider
	if provider == providerCodex && event.ParentID == "" {
		event.ParentID = event.SessionID
	}
	if event.AgentID == "" || (event.HookEventName != HookSubagentStart && event.HookEventName != HookSubagentStop) {
		return event, event.HookEventName, nil
	}
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(event)
	if err != nil {
		return event, event.HookEventName, err
	}
	f, err := os.OpenFile(sessionpaths.AgentEventLog(root), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return event, event.HookEventName, err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return event, event.HookEventName, err
}
