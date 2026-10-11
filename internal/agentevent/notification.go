package agentevent

import (
	"strconv"
	"strings"

	"github.com/kieranajp/qrouton/internal/workbench"
)

const (
	notificationOpen  = "<task-notification>"
	notificationClose = "</task-notification>"
)

// TaskNotifications reads each background agent result Claude Code delivers to
// its caller as a prompt. The result is kept as text to show; only the status
// and the usage figures are read.
func TaskNotifications(prompt string) []workbench.TaskNotification {
	var out []workbench.TaskNotification
	for {
		start := strings.Index(prompt, notificationOpen)
		if start < 0 {
			return out
		}
		prompt = prompt[start+len(notificationOpen):]
		end := strings.LastIndex(prompt, notificationClose)
		if next := strings.Index(prompt, notificationOpen); next >= 0 {
			end = strings.LastIndex(prompt[:next], notificationClose)
		}
		if end < 0 {
			return out
		}
		block := prompt[:end]
		prompt = prompt[end+len(notificationClose):]
		n := workbench.TaskNotification{
			TaskID:    tag(block, "task-id"),
			ToolUseID: tag(block, "tool-use-id"),
			Result:    truncate(tag(block, "result"), MessageLimit),
			Outcome:   workbench.AgentOutcome{Status: tag(block, "status")},
		}
		usage := tag(block, "usage")
		n.Outcome.Tokens = int(number(usage, "subagent_tokens", "total_tokens"))
		n.Outcome.ToolUses = int(number(usage, "tool_uses"))
		n.Outcome.DurationMs = number(usage, "duration_ms")
		if n.TaskID != "" {
			out = append(out, n)
		}
	}
}

// tag is the text between a tag's first opening and its last closing, so a
// result quoting markup of its own stays whole.
func tag(text, name string) string {
	open, close := "<"+name+">", "</"+name+">"
	start := strings.Index(text, open)
	if start < 0 {
		return ""
	}
	rest := text[start+len(open):]
	end := strings.LastIndex(rest, close)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func number(text string, names ...string) int64 {
	for _, name := range names {
		if value, err := strconv.ParseInt(tag(text, name), 10, 64); err == nil {
			return value
		}
	}
	return 0
}
