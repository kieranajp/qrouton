package desktop

import (
	"bufio"
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func resumeFixture(t *testing.T, name string) []workbench.AgentEventRequest {
	t.Helper()
	file, err := os.Open("../agentevent/testdata/claude-2.1.296/" + name + ".jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	var out []workbench.AgentEventRequest
	for scanner.Scan() {
		event, err := agentevent.Reduce(agentProviderClaude, bytes.NewReader(scanner.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		event.Generation = 1
		event.Timestamp = start.Add(time.Duration(len(out)) * time.Second)
		out = append(out, event)
	}
	return out
}

// The orchestrator resumes a finished background agent with SendMessage. The
// resume lands on the agent's own row; it never adds a sibling.
func TestAgentTreeAResumeByMessageReopensTheOriginalAgent(t *testing.T) {
	events := resumeFixture(t, "run6-resume-hooks")
	const id = "a50ab93914355af81"
	var resumed int
	for i, event := range events {
		if event.Hook != agentevent.HookSubagentStart || event.AgentID != id {
			continue
		}
		if resumed++; resumed == 2 {
			tracker := replayed(t, events[:i+1])
			if c := stateOf(t, tracker, id); c.State != agentStateWorking {
				t.Fatalf("the resumed agent = %+v", c)
			}
		}
	}
	kids := children(replayed(t, events).snapshot())
	if len(kids) != 1 || kids[0].ID != id || kids[0].Type != "spike-a" || kids[0].Description != "Run echo one" {
		t.Fatalf("children after a resume = %+v", kids)
	}
	if kids[0].State != agentStateDone || kids[0].LastMessage == "" {
		t.Fatalf("the agent after its resumed turn = %+v", kids[0])
	}
}

// Interactive Claude Code runs helper agents of its own: hooks under a fresh
// id that carry no agent type. They are no agent qrouton should draw, so a
// helper that also announces a start adds no typeless row beside the agent
// the orchestrator resumed.
func TestAgentTreeIgnoresTypelessHelperAgents(t *testing.T) {
	events := resumeFixture(t, "run7-interactive-resume-hooks")
	last := events[len(events)-1].Timestamp
	helper := func(offset time.Duration, hook string, fill func(*workbench.AgentEventRequest)) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.AgentID, e.Timestamp = "a1eb18c5a0c8b95f2", last.Add(offset)
			if fill != nil {
				fill(e)
			}
		})
	}
	events = append(events,
		helper(time.Second, agentevent.HookSubagentStart, nil),
		helper(2*time.Second, agentevent.HookPreToolUse, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Read", "h1" }),
		helper(3*time.Second, agentevent.HookSubagentStop, nil),
	)
	kids := children(replayed(t, events).snapshot())
	if len(kids) != 1 || kids[0].ID != "a09da8d3bc8106298" || kids[0].Type != "spike-a" {
		t.Fatalf("children = %+v", kids)
	}
}
