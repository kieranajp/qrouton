package agentevent

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// claudePayload is the part of a Claude Code hook payload the tree reads. The
// tool's input and response stay raw until the tool is known.
type claudePayload struct {
	HookEventName        string           `json:"hook_event_name"`
	AgentID              string           `json:"agent_id"`
	AgentType            string           `json:"agent_type"`
	ToolName             string           `json:"tool_name"`
	ToolUseID            string           `json:"tool_use_id"`
	ToolInput            json.RawMessage  `json:"tool_input"`
	ToolResponse         json.RawMessage  `json:"tool_response"`
	Prompt               string           `json:"prompt"`
	Message              string           `json:"message"`
	NotificationType     string           `json:"notification_type"`
	LastAssistantMessage string           `json:"last_assistant_message"`
	Error                json.RawMessage  `json:"error"`
	ErrorDetails         string           `json:"error_details"`
	BackgroundTasks      []backgroundTask `json:"background_tasks"`
}

type backgroundTask struct {
	ID string `json:"id"`
}

type spawnInput struct {
	SubagentType    string `json:"subagent_type"`
	Description     string `json:"description"`
	RunInBackground bool   `json:"run_in_background"`
}

type spawnResponse struct {
	Status            string `json:"status"`
	IsAsync           bool   `json:"isAsync"`
	AgentID           string `json:"agentId"`
	TotalTokens       int    `json:"totalTokens"`
	TotalDurationMs   int64  `json:"totalDurationMs"`
	TotalToolUseCount int    `json:"totalToolUseCount"`
	Content           []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// Reduce decodes one Claude Code hook payload into the event the workbench is
// sent. It keeps ids, the tool and its target, the spawn and its outcome, and
// one truncated message; tool inputs and responses go no further.
func Reduce(provider string, input io.Reader) (workbench.AgentEventRequest, error) {
	at := time.Now().UTC()
	var p claudePayload
	if err := json.NewDecoder(input).Decode(&p); err != nil {
		return workbench.AgentEventRequest{}, err
	}
	event := workbench.AgentEventRequest{
		Provider: provider, Hook: p.HookEventName, Timestamp: at,
		AgentID: p.AgentID, AgentType: p.AgentType,
		ToolUseID: p.ToolUseID, Tool: p.ToolName,
		NotificationType: p.NotificationType,
	}
	if p.ToolName != "" {
		event.Target = truncate(Target(p.ToolName, p.ToolInput), TargetLimit)
	}
	for _, task := range p.BackgroundTasks {
		if task.ID != "" {
			event.BackgroundTasks = append(event.BackgroundTasks, task.ID)
		}
	}
	switch p.HookEventName {
	case HookPreToolUse, HookPostToolUse, HookPostToolUseFailure:
		if spawnTool(p.ToolName) {
			reduceSpawn(&event, p)
		}
		if p.HookEventName == HookPostToolUseFailure {
			event.Message = errorText(p.Error, p.ErrorDetails)
		}
	case HookSubagentStop, HookStop:
		event.Message = p.LastAssistantMessage
	case HookNotification:
		event.Message = p.Message
	case HookStopFailure:
		event.Message = errorText(p.Error, p.ErrorDetails)
	case HookUserPromptSubmit:
		event.Notifications = TaskNotifications(p.Prompt)
	}
	event.Message = truncate(event.Message, MessageLimit)
	return event, nil
}

func spawnTool(name string) bool { return name == ToolAgent || name == ToolTaskLegacy }

func reduceSpawn(event *workbench.AgentEventRequest, p claudePayload) {
	var in spawnInput
	_ = json.Unmarshal(p.ToolInput, &in)
	event.Spawn = &workbench.AgentSpawn{
		Type: in.SubagentType, Description: truncate(in.Description, TargetLimit), Async: in.RunInBackground,
	}
	if p.HookEventName != HookPostToolUse || len(p.ToolResponse) == 0 {
		return
	}
	var out spawnResponse
	if json.Unmarshal(p.ToolResponse, &out) != nil {
		return
	}
	event.ChildID = out.AgentID
	if out.IsAsync || out.Status == "" || out.Status == spawnAsyncStatus {
		event.Spawn.Async = true
		return
	}
	event.Outcome = &workbench.AgentOutcome{
		Status: out.Status, Tokens: out.TotalTokens,
		DurationMs: out.TotalDurationMs, ToolUses: out.TotalToolUseCount,
	}
	texts := make([]string, 0, len(out.Content))
	for _, part := range out.Content {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	event.Message = strings.Join(texts, "\n")
}

// errorText reads an error that may arrive as a string or as an object.
func errorText(raw json.RawMessage, details string) string {
	var text string
	if json.Unmarshal(raw, &text) != nil && len(raw) > 0 {
		text = string(raw)
	}
	return strings.TrimSpace(strings.Join(nonEmpty(text, details), ": "))
}

func nonEmpty(values ...string) []string {
	out := values[:0]
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func truncate(s string, limit int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + truncationMark
}
