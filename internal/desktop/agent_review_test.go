package desktop

import (
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/kinds"
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

// A completed background child the user messages again works again, and hooks
// stamped before it finished never reopen it.
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

	before := finished.FinishedAt.Sub(last)
	tracker.apply(at(before-time.Second, agentevent.HookPostToolUse, "late-post"))
	tracker.apply(at(before-2*time.Second, agentevent.HookPreToolUse, "toolu_01Fg8Hhh4PXjSNUf3sBWMgaH"))
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

func resumedChild(t *testing.T) (*agentActivity, agentRecord, func(time.Duration, string, string) workbench.AgentEventRequest) {
	t.Helper()
	events := spikeEvents(t, "run4-bg-hooks")
	tracker := replayed(t, events)
	const id = "a5438536af5bc5c5e"
	finished := stateOf(t, tracker, id)
	at := func(offset time.Duration, hook, toolUse string) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.AgentID, e.AgentType, e.Tool, e.ToolUseID, e.Target = id, "spike-a", "Bash", toolUse, "echo again"
			e.Timestamp = finished.FinishedAt.Add(offset)
		})
	}
	return tracker, finished, at
}

// A resumed call whose result reaches the workbench before its start still
// reopens the child: both were stamped after it finished.
func TestAgentStateAResumedCallReopensWhicheverHookArrivesFirst(t *testing.T) {
	tracker, _, at := resumedChild(t)
	tracker.apply(at(2*time.Second, agentevent.HookPostToolUse, "resume-1"))
	tracker.apply(at(time.Second, agentevent.HookPreToolUse, "resume-1"))
	if c := stateOf(t, tracker, "a5438536af5bc5c5e"); c.State != agentStateWorking {
		t.Fatalf("resumed child with its result first = %+v", c)
	}
	if live := tracker.snapshot().LiveTypes; len(live) != 1 {
		t.Fatalf("live types = %v", live)
	}
}

// The previous activation's stop, held back until after the child resumed,
// must not finish the resumed activation.
func TestAgentStateAStaleStopDoesNotFinishAResumedChild(t *testing.T) {
	events := spikeEvents(t, "run4-bg-hooks")
	var stop workbench.AgentEventRequest
	kept := events[:0:0]
	for _, e := range events {
		if e.Hook == agentevent.HookSubagentStop {
			stop = e
			continue
		}
		kept = append(kept, e)
	}
	tracker := replayed(t, kept)
	const id = "a5438536af5bc5c5e"
	finished := stateOf(t, tracker, id)
	tracker.apply(claudeEvent(agentevent.HookPreToolUse, func(e *workbench.AgentEventRequest) {
		e.AgentID, e.AgentType, e.Tool, e.ToolUseID = id, "spike-a", "Bash", "resume-1"
		e.Timestamp = finished.FinishedAt.Add(time.Second)
	}))
	tracker.apply(stop)
	stale := claudeEvent(agentevent.HookUserPromptSubmit, func(e *workbench.AgentEventRequest) {
		e.Timestamp = finished.FinishedAt
		e.Notifications = []workbench.TaskNotification{{TaskID: id, Outcome: workbench.AgentOutcome{Status: "completed"}}}
	})
	tracker.apply(stale)
	if c := stateOf(t, tracker, id); c.State != agentStateWorking || c.Tool != "Bash" {
		t.Fatalf("resumed child after its old stop and outcome = %+v", c)
	}
	if live := tracker.snapshot().LiveTypes; len(live) != 1 {
		t.Fatalf("live types = %v", live)
	}
}

// A lead that launches background specialists ends its own turn while they
// work. It waits on its agents, not done, until a later stop with nothing of
// its own still running.
func TestAgentStateALeadWaitingOnBackgroundChildrenIsNotDone(t *testing.T) {
	start := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	step := 0
	ev := func(hook, agent, typ string, fill func(*workbench.AgentEventRequest)) workbench.AgentEventRequest {
		step++
		e := workbench.AgentEventRequest{
			Provider: agentProviderClaude, Generation: 1, Hook: hook, AgentID: agent, AgentType: typ,
			Timestamp: start.Add(time.Duration(step) * time.Second),
		}
		if fill != nil {
			fill(&e)
		}
		return e
	}
	spawn := func(hook, caller, callerType, toolUse, typ, child string) workbench.AgentEventRequest {
		return ev(hook, caller, callerType, func(e *workbench.AgentEventRequest) {
			e.Tool, e.ToolUseID, e.ChildID = "Agent", toolUse, child
			e.Spawn = &workbench.AgentSpawn{Type: typ, Async: true}
		})
	}
	const lead, leadType, child = "lead-1", "qrouton-research-lead", "spec-1"
	tracker := replayed(t, []workbench.AgentEventRequest{
		ev(agentevent.HookUserPromptSubmit, "", "", nil),
		spawn(agentevent.HookPreToolUse, "", "", "t0", leadType, ""),
		spawn(agentevent.HookPostToolUse, "", "", "t0", leadType, lead),
		ev(agentevent.HookSubagentStart, lead, leadType, nil),
		spawn(agentevent.HookPreToolUse, lead, leadType, "t1", "codebase-researcher", ""),
		spawn(agentevent.HookPostToolUse, lead, leadType, "t1", "codebase-researcher", child),
		ev(agentevent.HookSubagentStart, child, "codebase-researcher", nil),
		ev(agentevent.HookSubagentStop, lead, leadType, func(e *workbench.AgentEventRequest) {
			e.Message, e.BackgroundTasks = "Launched the specialist.", []string{lead, child}
		}),
	})
	if l := stateOf(t, tracker, lead); l.State != agentStateWaitingAgents || !l.FinishedAt.IsZero() {
		t.Fatalf("a lead with a specialist still running = %+v", l)
	}
	if live := tracker.snapshot().LiveTypes; len(live) != 2 {
		t.Fatalf("live types = %v, want the lead and its specialist", live)
	}

	tracker.apply(ev(agentevent.HookPreToolUse, child, "codebase-researcher", func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Bash", "t2" }))
	tracker.apply(ev(agentevent.HookPostToolUse, child, "codebase-researcher", func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Bash", "t2" }))
	tracker.apply(ev(agentevent.HookSubagentStop, child, "codebase-researcher", nil))
	if l := stateOf(t, tracker, lead); l.State != agentStateWaitingAgents {
		t.Fatalf("a lead not yet woken by its specialist's result = %q", l.State)
	}

	tracker.apply(ev(agentevent.HookPreToolUse, lead, leadType, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Write", "t3" }))
	if l := stateOf(t, tracker, lead); l.State != agentStateWorking || l.Tool != "Write" {
		t.Fatalf("the woken lead = %+v", l)
	}
	tracker.apply(ev(agentevent.HookPostToolUse, lead, leadType, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Write", "t3" }))
	tracker.apply(ev(agentevent.HookSubagentStop, lead, leadType, func(e *workbench.AgentEventRequest) {
		e.BackgroundTasks = []string{lead}
	}))
	if l := stateOf(t, tracker, lead); l.State != agentStateDone {
		t.Fatalf("the lead after its last stop = %+v", l)
	}
}

// Thinking means between tool calls, which only a runner reporting its tool
// calls can tell; the page reads that from the tree and the row summary.
func TestTheTreeAndSummarySayWhetherTheRunnerReportsTools(t *testing.T) {
	for provider, want := range map[string]bool{
		agentProviderClaude: true, agentProviderCodex: false, agentProviderOpenCode: false, agentProviderAgy: false,
	} {
		tracker := newAgentActivity(time.Now)
		tracker.begin(provider, 1)
		if got := agentTree(tracker.snapshot(), "", "").Tools; got != want {
			t.Errorf("%s tree tools = %v, want %v", provider, got, want)
		}
		if got := agentSummary(tracker.snapshot()).Tools; got != want {
			t.Errorf("%s summary tools = %v, want %v", provider, got, want)
		}
	}
}

// The Overview names each lead's stage, and the workflow documents it wrote
// inside the session, which it opens from the lead's row.
func TestTheTreeNamesALeadsStageAndTheDocumentsItWrote(t *testing.T) {
	root := t.TempDir()
	tracker := replayed(t, nil)
	lead := func(hook, tool, id, target string) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.AgentID, e.AgentType, e.Tool, e.ToolUseID, e.Target = "lead-1", "qrouton-research-lead", tool, id, target
		})
	}
	for i, path := range []string{
		root + "/thoughts/shared/research/R3-2026-10-11-retry.md",
		root + "/src/web/notes.md",
		"/elsewhere/thoughts/shared/plans/P9-other.md",
	} {
		id := string(rune('a' + i))
		tracker.apply(lead(agentevent.HookPreToolUse, "Write", id, path))
		tracker.apply(lead(agentevent.HookPostToolUse, "Write", id, path))
	}
	var node status.AgentNode
	for _, n := range agentTree(tracker.snapshot(), "", root).Nodes {
		if n.ID == "lead-1" {
			node = n
		}
	}
	if node.Stage != status.StageResearch || len(node.Documents) != 1 ||
		node.Documents[0].ID != "R3" || node.Documents[0].Kind != kinds.Research.Label() ||
		node.Documents[0].Path != "thoughts/shared/research/R3-2026-10-11-retry.md" {
		t.Fatalf("lead node = stage %q documents %+v", node.Stage, node.Documents)
	}
}

// A denied tool call fires PreToolUse and nothing after it. The turn ending
// abandons it, so the root reads your turn, and later calls show normally.
func TestAgentStateATurnEndAbandonsACallThatNeverReturned(t *testing.T) {
	start := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	at := func(s int, hook string, fill func(*workbench.AgentEventRequest)) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.Timestamp = start.Add(time.Duration(s) * time.Second)
			if fill != nil {
				fill(e)
			}
		})
	}
	bash := func(id, target string) func(*workbench.AgentEventRequest) {
		return func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID, e.Target = "Bash", id, target }
	}
	tracker := replayed(t, []workbench.AgentEventRequest{
		at(0, agentevent.HookUserPromptSubmit, nil),
		at(1, agentevent.HookPreToolUse, bash("denied", "make front")),
		at(2, agentevent.HookStop, nil),
	})
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateYourTurn || root.Tool != "" {
		t.Fatalf("after the denied call's turn ended the root = %q tool %q", root.State, root.Tool)
	}

	tracker.apply(at(3, agentevent.HookUserPromptSubmit, nil))
	tracker.apply(at(4, agentevent.HookPreToolUse, bash("next", "go test ./...")))
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateWorking || root.Target != "go test ./..." {
		t.Fatalf("the next call = %q %q", root.State, root.Target)
	}
	tracker.apply(at(5, agentevent.HookPostToolUse, bash("next", "go test ./...")))
	tracker.apply(at(6, agentevent.HookPostToolUse, bash("denied", "make front")))
	root := stateOf(t, tracker, agentRootID)
	if root.Tool != "" {
		t.Fatalf("a late result reopened a call: %+v", root.Tool)
	}
	for _, call := range root.Tools {
		if call.ID == "denied" && (call.Abandoned || !call.Closed) {
			t.Fatalf("the late result was not recorded: %+v", call)
		}
	}
}
