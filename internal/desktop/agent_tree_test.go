package desktop

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/workbench"
)

const claudeSpike = "../agentevent/testdata/claude-2.1.286"

// spikeEvents is one recorded Claude Code run, reduced as the hook binary
// reduces it and stamped for generation 1.
func spikeEvents(t *testing.T, name string) []workbench.AgentEventRequest {
	t.Helper()
	file, err := os.Open(filepath.Join(claudeSpike, name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	start := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
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

func replayed(t *testing.T, events []workbench.AgentEventRequest) *agentActivity {
	t.Helper()
	clock := &activityClock{at: time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderClaude, 1)
	for _, event := range events {
		tracker.apply(event)
	}
	return tracker
}

func children(snapshot agentActivitySnapshot) []agentRecord {
	var out []agentRecord
	for _, record := range snapshot.Records {
		if !record.Root {
			out = append(out, record)
		}
	}
	return out
}

func TestAgentTreePlacesADepthTwoChildUnderItsCaller(t *testing.T) {
	view := replayed(t, spikeEvents(t, "run2-hooks")).snapshot()
	if got := len(children(view)); got != 2 {
		t.Fatalf("children = %+v", children(view))
	}
	lead := delegatedRecord(t, view, "a08c4b226d2052caf")
	inner := delegatedRecord(t, view, "ac6bcc1d7a22d6069")
	if lead.ParentID != agentRootID || !lead.ParentKnown || inner.ParentID != lead.ID || !inner.ParentKnown {
		t.Fatalf("depth-1 %+v, depth-2 %+v", lead, inner)
	}
	if inner.Description == "" || inner.State != agentStateDone {
		t.Fatalf("depth-2 child = %+v", inner)
	}
}

func TestAgentTreeTakesAnEarlyPostToolUseAsTheChild(t *testing.T) {
	view := replayed(t, spikeEvents(t, "run1-hooks")).snapshot()
	kids := children(view)
	if len(kids) != 1 || kids[0].ID != "a3068ab302d038a35" || kids[0].ParentID != agentRootID ||
		!kids[0].ParentKnown || kids[0].Type != "spike-a" {
		t.Fatalf("children = %+v", kids)
	}
}

func TestAgentTreeDropsStrayStopsWithNoTypeOrStart(t *testing.T) {
	view := replayed(t, spikeEvents(t, "run5-interactive-hooks")).snapshot()
	if kids := children(view); len(kids) != 1 || kids[0].ID != "a11ef90b0e408b6d5" {
		t.Fatalf("children = %+v", kids)
	}
}

func TestAgentTreeReadsABackgroundOutcomeFromItsNotification(t *testing.T) {
	view := replayed(t, spikeEvents(t, "run4-bg-hooks")).snapshot()
	child := delegatedRecord(t, view, "a5438536af5bc5c5e")
	if child.Outcome != "completed" || child.Tokens != 40927 || child.DurationMs != 9040 ||
		child.State != agentStateDone || child.LastMessage == "" || child.ParentID != agentRootID {
		t.Fatalf("background child = %+v", child)
	}
}

func TestAgentTreeReadsAForegroundOutcomeFromItsResult(t *testing.T) {
	view := replayed(t, spikeEvents(t, "run3-fg-hooks")).snapshot()
	child := delegatedRecord(t, view, "ab76fea07a9b71550")
	if child.Outcome != "completed" || child.Tokens != 42191 || child.ToolCount != 1 || child.State != agentStateDone {
		t.Fatalf("foreground child = %+v", child)
	}
}

func TestAgentTreeToleratesReversedDelivery(t *testing.T) {
	events := spikeEvents(t, "run3-fg-hooks")
	slices.Reverse(events)
	view := replayed(t, events).snapshot()
	for _, record := range view.Records {
		for _, call := range record.Tools {
			if !call.Closed {
				t.Fatalf("%s left %s open: %+v", record.ID, call.Tool, call)
			}
		}
	}
	child := delegatedRecord(t, view, "ab76fea07a9b71550")
	if child.ParentID != agentRootID || !child.ParentKnown || child.State != agentStateDone || len(child.Tools) != 1 {
		t.Fatalf("child = %+v", child)
	}
	root := recordForRun(t, view, agentRootID, 1)
	if len(root.Tools) != 1 || root.ToolCount != 1 {
		t.Fatalf("root tools = %+v", root.Tools)
	}
}

// Two callers spawn one type at once. Each child is placed under the oldest
// waiting call first, and each call's own result moves a swapped pair back.
func TestAgentTreeMatchesSameTypeSpawnsInOrderAndCorrectsASwap(t *testing.T) {
	at := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	step := func(event workbench.AgentEventRequest) workbench.AgentEventRequest {
		event.Provider, event.Generation, event.Timestamp = agentProviderClaude, 1, at
		at = at.Add(time.Second)
		return event
	}
	spawn := func(hook, caller, toolUse, child string) workbench.AgentEventRequest {
		return step(workbench.AgentEventRequest{
			Hook: hook, AgentID: caller, AgentType: "lead", Tool: "Agent", ToolUseID: toolUse,
			Spawn: &workbench.AgentSpawn{Type: "explorer", Description: "for " + caller}, ChildID: child,
		})
	}
	start := func(id, typ string) workbench.AgentEventRequest {
		return step(workbench.AgentEventRequest{Hook: agentevent.HookSubagentStart, AgentID: id, AgentType: typ})
	}
	tracker := replayed(t, []workbench.AgentEventRequest{
		start("lead-1", "lead"), start("lead-2", "lead"),
		spawn(agentevent.HookPreToolUse, "lead-1", "t1", ""),
		spawn(agentevent.HookPreToolUse, "lead-2", "t2", ""),
		start("child-a", "explorer"), start("child-b", "explorer"),
	})
	parents := func() (string, string) {
		view := tracker.snapshot()
		return delegatedRecord(t, view, "child-a").ParentID, delegatedRecord(t, view, "child-b").ParentID
	}
	if a, b := parents(); a != "lead-1" || b != "lead-2" {
		t.Fatalf("first in, first out placed child-a under %s and child-b under %s", a, b)
	}
	tracker.apply(spawn(agentevent.HookPostToolUse, "lead-1", "t1", "child-b"))
	if a, b := parents(); a != "lead-2" || b != "lead-1" {
		t.Fatalf("after t1 named child-b: child-a under %s, child-b under %s", a, b)
	}
	tracker.apply(spawn(agentevent.HookPostToolUse, "lead-2", "t2", "child-a"))
	if a, b := parents(); a != "lead-2" || b != "lead-1" {
		t.Fatalf("after both results: child-a under %s, child-b under %s", a, b)
	}
}

func TestAgentTreeCapsTheSessionByDroppingOldFinishedLeaves(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderCodex, 1)
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderCodex, Generation: 1, Kind: workbench.LifecycleStart, ID: "live",
	})
	for i := range agentRecordCap + 10 {
		clock.at = clock.at.Add(time.Second)
		event := workbench.DelegatedLifecycleRequest{
			Provider: agentProviderCodex, Generation: 1, Kind: workbench.LifecycleStop, ID: "done-" + time.Duration(i).String(),
		}
		tracker.lifecycle(event)
	}
	view := tracker.snapshot()
	if len(view.Records) != agentRecordCap {
		t.Fatalf("records = %d, want the cap of %d", len(view.Records), agentRecordCap)
	}
	if _, ok := findRecord(view, "live", 1); !ok {
		t.Fatal("the cap dropped a live agent")
	}
	if _, ok := findRecord(view, "done-0s", 1); ok {
		t.Fatal("the cap kept the oldest finished leaf")
	}
}

func TestAgentTreeLeavesAChildNoSpawnNamesUnplaced(t *testing.T) {
	view := replayed(t, []workbench.AgentEventRequest{{
		Provider: agentProviderClaude, Generation: 1, Hook: agentevent.HookSubagentStart,
		AgentID: "orphan", AgentType: "explorer", Timestamp: time.Now(),
	}}).snapshot()
	if orphan := delegatedRecord(t, view, "orphan"); orphan.ParentID != agentRootID || orphan.ParentKnown {
		t.Fatalf("orphan = %+v, want under the root and unplaced", orphan)
	}
}

func TestAgentTreeArrivesOverTheControlSocket(t *testing.T) {
	windows, _ := testWindows(t)
	tracker := newAgentActivity(time.Now)
	tracker.begin(agentProviderClaude, 1)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{
		agentEvent: func(req workbench.AgentEventRequest) { tracker.apply(req) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	handle := workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}
	if err := handle.AgentEvent(context.Background(), workbench.AgentEventRequest{
		Provider: agentProviderClaude, Generation: 1, Hook: agentevent.HookSubagentStart,
		AgentID: "agent-1", AgentType: "explorer", Timestamp: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	delegatedRecord(t, tracker.snapshot(), "agent-1")
}
