package desktop

import (
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// Only the orchestrator talks to the user: a child's question is the root's
// to show, and the child just shows the tool it is in.
func TestAgentStateAChildsQuestionReachesTheUserThroughTheRoot(t *testing.T) {
	child := func(hook string, fill func(*workbench.AgentEventRequest)) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.AgentID, e.AgentType = "c1", "explorer"
			if fill != nil {
				fill(e)
			}
		})
	}
	ask := func(hook string) workbench.AgentEventRequest {
		return child(hook, func(e *workbench.AgentEventRequest) {
			e.Tool, e.ToolUseID, e.Target = agentevent.ToolAskUserQuestion, "ask-1", "Which branch?"
		})
	}
	tracker := replayed(t, []workbench.AgentEventRequest{
		claudeEvent(agentevent.HookUserPromptSubmit, nil),
		claudeEvent(agentevent.HookPreToolUse, func(e *workbench.AgentEventRequest) {
			e.Tool, e.ToolUseID, e.Spawn = "Agent", "spawn-1", &workbench.AgentSpawn{Type: "explorer"}
		}),
		child(agentevent.HookSubagentStart, nil),
	})
	tracker.seen()

	_, alert := tracker.apply(ask(agentevent.HookPreToolUse))
	if c := stateOf(t, tracker, "c1"); c.State != agentStateWorking || c.Tool != agentevent.ToolAskUserQuestion {
		t.Fatalf("asking child = %+v, want working in the ask tool", c)
	}
	root := stateOf(t, tracker, agentRootID)
	summary := agentSummary(tracker.snapshot())
	if root.State != agentStateNeedsYou || root.Reason != "Which branch?" || !alert || !tracker.snapshot().Unread ||
		summary.State != agentStateNeedsYou || summary.Reason != "Which branch?" || tracker.state() != status.ActivityWaiting {
		t.Fatalf("root %+v, alert %v, summary %+v, rail %q", root, alert, summary, tracker.state())
	}

	tracker.apply(ask(agentevent.HookPostToolUse))
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWaitingAgents {
		t.Fatalf("after the answer the root = %q", got)
	}
}

// A completed background child the user messages again works again, and the
// late hooks of the activation that finished never reopen it.
func TestAgentStateAResumedChildWorksAgain(t *testing.T) {
	events := spikeEvents(t, "run4-bg-hooks")
	tracker := replayed(t, events)
	const id = "a5438536af5bc5c5e"
	last := events[len(events)-1].Timestamp
	at := func(offset time.Duration, hook, toolUse string) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.AgentID, e.AgentType, e.Tool, e.ToolUseID, e.Target, e.Timestamp = id, "spike-a", "Bash", toolUse, "echo again", last.Add(offset)
		})
	}
	finished := stateOf(t, tracker, id)

	tracker.apply(at(time.Second, agentevent.HookPostToolUse, "late-post"))
	tracker.apply(at(time.Second, agentevent.HookPreToolUse, "toolu_01Fg8Hhh4PXjSNUf3sBWMgaH"))
	stale := claudeEvent(agentevent.HookSubagentStart, func(e *workbench.AgentEventRequest) {
		e.AgentID, e.AgentType, e.Timestamp = id, "spike-a", finished.FinishedAt.Add(-time.Second)
	})
	tracker.apply(stale)
	if c := stateOf(t, tracker, id); c.State != agentStateDone {
		t.Fatalf("a late hook reopened the finished child: %+v", c)
	}

	tracker.apply(at(2*time.Second, agentevent.HookPreToolUse, "resume-1"))
	c := stateOf(t, tracker, id)
	if c.State != agentStateWorking || c.Tool != "Bash" || !c.FinishedAt.IsZero() || c.Outcome != "" ||
		c.Tokens != finished.Tokens || c.ToolCount <= finished.ToolCount {
		t.Fatalf("resumed child = %+v (finished as %+v)", c, finished)
	}
	if live := tracker.snapshot().LiveTypes; len(live) != 1 || live[0] != "spike-a" {
		t.Fatalf("live types = %v", live)
	}
	restart := claudeEvent(agentevent.HookSubagentStart, func(e *workbench.AgentEventRequest) {
		e.AgentID, e.AgentType, e.Timestamp = id, "spike-a", last.Add(time.Hour)
	})
	tracker.apply(claudeEvent(agentevent.HookSubagentStop, func(e *workbench.AgentEventRequest) {
		e.AgentID, e.AgentType, e.Timestamp = id, "spike-a", last.Add(time.Minute)
	}))
	tracker.apply(restart)
	if c := stateOf(t, tracker, id); c.State == agentStateDone {
		t.Fatalf("a newer SubagentStart left the child done: %+v", c)
	}
}
