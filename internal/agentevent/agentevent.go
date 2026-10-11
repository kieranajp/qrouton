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
	TaskName      string `json:"-"`
	ParentID      string `json:"parent_agent_id,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
}

// Record decodes one subagent hook event from input and returns it alongside
// the hook name, which the caller maps to an attention state even for the hooks
// no lifecycle is derived from.
func Record(provider string, input io.Reader) (Event, string, error) {
	var hook struct {
		Event
		TranscriptPath      string `json:"transcript_path"`
		AgentTranscriptPath string `json:"agent_transcript_path"`
	}
	if err := json.NewDecoder(input).Decode(&hook); err != nil {
		return Event{}, "", err
	}
	event := hook.Event
	event.Provider = provider
	if provider == providerCodex && event.ParentID == "" {
		event.ParentID = event.SessionID
	}
	if event.AgentID == "" || (event.HookEventName != HookSubagentStart && event.HookEventName != HookSubagentStop) {
		return event, event.HookEventName, nil
	}
	if provider == providerCodex {
		path := hook.TranscriptPath
		if event.HookEventName == HookSubagentStop {
			path = hook.AgentTranscriptPath
		}
		event.TaskName = codexTaskName(path, event.AgentID)
	}
	event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	return event, event.HookEventName, nil
}
