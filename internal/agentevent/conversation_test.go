package agentevent

import (
	"bufio"
	"bytes"
	"os"
	"testing"
)

// Catches the root's conversation going unreported, or a subagent's hook
// passing itself off as the root's conversation.
func TestReduceNamesTheConversationOnlyOnTheRootsHooks(t *testing.T) {
	const (
		id         = "b8aaffa4-120a-4a6d-bf40-cc703dc147b8"
		transcript = "/home/user/.claude/projects/-tmp-spike/" + id + ".jsonl"
	)
	file, err := os.Open("testdata/claude-2.1.296/run7-interactive-resume-hooks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var roots, subagents int
	for scanner.Scan() {
		event, err := ReduceIn("claude", "", bytes.NewReader(scanner.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if event.AgentID != "" {
			subagents++
			if event.ConversationID != "" || event.TranscriptPath != "" {
				t.Fatalf("subagent %s %s names conversation %q at %q", event.AgentID, event.Hook, event.ConversationID, event.TranscriptPath)
			}
			continue
		}
		roots++
		if event.ConversationID != id || event.TranscriptPath != transcript {
			t.Fatalf("root %s names conversation %q at %q, want %q at %q", event.Hook, event.ConversationID, event.TranscriptPath, id, transcript)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if roots == 0 || subagents == 0 {
		t.Fatalf("fixture replayed %d root and %d subagent hooks, want both", roots, subagents)
	}
}
