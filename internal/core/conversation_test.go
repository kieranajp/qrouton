package core

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

const recordedID = "b8aaffa4-120a-4a6d-bf40-cc703dc147b8"

func conversationState(t *testing.T) *SessionState {
	t.Helper()
	state := &SessionState{agents: newAgentActivity(time.Now), sessionRoot: t.TempDir()}
	state.agents.begin(agentProviderClaude, 1)
	return state
}

func conversationEvent(generation uint64, id string) workbench.AgentEventRequest {
	return claudeEvent(agentevent.HookUserPromptSubmit, func(e *workbench.AgentEventRequest) {
		e.Generation = generation
		e.ConversationID = id
		e.TranscriptPath = "/home/user/.claude/projects/-tmp-s/" + id + ".jsonl"
	})
}

// Catches the root's conversation never reaching disk, which leaves every resume on --continue.
func TestACurrentRootHookRecordsTheConversation(t *testing.T) {
	state := conversationState(t)
	event := conversationEvent(1, recordedID)
	state.keepConversation(event)
	got, ok := session.ReadConversation(state.Root())
	if !ok || got.ID != recordedID || got.Transcript != event.TranscriptPath {
		t.Fatalf("recorded %+v, %v, want %s at %s", got, ok, recordedID, event.TranscriptPath)
	}
}

// Catches a replaced runner's late hook rewriting the record its successor's launch cleared.
func TestAStaleGenerationHookLeavesTheRecordAlone(t *testing.T) {
	state := conversationState(t)
	state.keepConversation(conversationEvent(1, recordedID))
	state.agents.begin(agentProviderClaude, 2)
	state.keepConversation(conversationEvent(1, "c0ffee00-0000-4000-8000-000000000000"))
	if got, _ := session.ReadConversation(state.Root()); got.ID != recordedID {
		t.Fatalf("a stale hook rewrote the record to %q", got.ID)
	}
}

// Catches a subagent's hook naming a conversation of its own and replacing the root's.
func TestASubagentHookLeavesTheRecordAlone(t *testing.T) {
	state := conversationState(t)
	state.keepConversation(conversationEvent(1, recordedID))
	subagent := conversationEvent(1, "c0ffee00-0000-4000-8000-000000000000")
	subagent.AgentID, subagent.AgentType = "a0c0c0d7f2323c2a8", "spike"
	state.keepConversation(subagent)
	if got, _ := session.ReadConversation(state.Root()); got.ID != recordedID {
		t.Fatalf("a subagent hook rewrote the record to %q", got.ID)
	}
}

// Catches every hook rewriting the same record, which is a disk write per tool call.
func TestARepeatedPairIsWrittenOnce(t *testing.T) {
	state := conversationState(t)
	state.keepConversation(conversationEvent(1, recordedID))
	if err := os.Remove(sessionpaths.Conversation(state.Root())); err != nil {
		t.Fatal(err)
	}
	state.keepConversation(conversationEvent(1, recordedID))
	if _, err := os.Stat(sessionpaths.Conversation(state.Root())); !os.IsNotExist(err) {
		t.Fatalf("the same pair was written twice: %v", err)
	}
}

// Catches the reducer or the recorder passing over a SessionStart, so a conversation
// picked with /resume stays unrecorded until its first prompt.
func TestASessionStartMovesTheRecordWithoutAPrompt(t *testing.T) {
	const picked = "c0ffee00-0000-4000-8000-000000000000"
	state := conversationState(t)
	state.keepConversation(conversationEvent(1, recordedID))
	payload := `{"session_id":"` + picked + `","transcript_path":"/home/user/.claude/projects/-tmp-s/` + picked +
		`.jsonl","cwd":"/tmp/s","hook_event_name":"SessionStart","source":"resume"}`
	event, err := agentevent.ReduceIn(agentProviderClaude, state.Root(), strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	event.Generation = 1
	state.keepConversation(event)
	if got, _ := session.ReadConversation(state.Root()); got.ID != picked {
		t.Fatalf("the record names %q after SessionStart, want %q", got.ID, picked)
	}
}
