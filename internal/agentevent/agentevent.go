package agentevent

import (
	"encoding/json"
	"io"
	"time"
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

// Record decodes one subagent hook event from input and returns it alongside
// the hook name, which the caller maps to an attention state even for the hooks
// no lifecycle is derived from.
func Record(provider string, input io.Reader) (Event, string, error) {
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
	return event, event.HookEventName, nil
}
