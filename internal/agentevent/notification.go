package agentevent

import (
	"strconv"
	"strings"

	"github.com/kieranajp/qrouton/internal/workbench"
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
			TaskID:    truncate(tag(block, tagTaskID, false), IDLimit),
			ToolUseID: truncate(tag(block, tagToolUseID, false), IDLimit),
			Result:    truncate(tag(block, tagResult, true), MessageLimit),
			Outcome:   workbench.AgentOutcome{Status: truncate(tag(block, tagStatus, false), IDLimit)},
		}
		usage := tag(block, tagUsage, false)
		n.Outcome.Tokens = int(number(usage, tagSubagentTokens, tagTotalTokens))
		n.Outcome.ToolUses = int(number(usage, tagToolUses))
		n.Outcome.DurationMs = number(usage, tagDurationMs)
		if n.TaskID != "" && len(out) < ListLimit {
			out = append(out, n)
		}
	}
}

// tag is the text inside a tag's first opening and its first closing. A
// result runs to its last closing instead, so one quoting markup stays whole;
// it comes after every other tag, so their first closings are their own.
func tag(text, name string, last bool) string {
	open, close := "<"+name+">", "</"+name+">"
	start := strings.Index(text, open)
	if start < 0 {
		return ""
	}
	rest := text[start+len(open):]
	end := strings.Index(rest, close)
	if last {
		end = strings.LastIndex(rest, close)
	}
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func number(text string, names ...string) int64 {
	for _, name := range names {
		if value, err := strconv.ParseInt(tag(text, name, false), 10, 64); err == nil {
			return value
		}
	}
	return 0
}
