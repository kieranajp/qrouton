package desktop

import (
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func stateOf(t *testing.T, tracker *agentActivity, id string) agentRecord {
	t.Helper()
	view := tracker.snapshot()
	if id == agentRootID {
		return recordForRun(t, view, agentRootID, 1)
	}
	return delegatedRecord(t, view, id)
}

// stepTo replays a spike up to and including the nth event of a hook, and
// reports whether any step asked for the chime.
func stepTo(tracker *agentActivity, events []workbench.AgentEventRequest, from int, hook string, nth int) (int, bool) {
	alerted := false
	for i := from; i < len(events); i++ {
		_, alert := tracker.apply(events[i])
		alerted = alerted || alert
		if events[i].Hook == hook {
			if nth--; nth == 0 {
				return i + 1, alerted
			}
		}
	}
	return len(events), alerted
}

func TestAgentStateFollowsAForegroundSpawn(t *testing.T) {
	events := spikeEvents(t, "run3-fg-hooks")
	tracker := replayed(t, nil)
	const child = "ab76fea07a9b71550"

	if got := stateOf(t, tracker, agentRootID).State; got != agentStateYourTurn {
		t.Fatalf("a root nothing has been asked of = %q", got)
	}
	at, _ := stepTo(tracker, events, 0, agentevent.HookUserPromptSubmit, 1)
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateWorking || root.Tool != "" {
		t.Fatalf("after the prompt the root = %q tool %q, want thinking", root.State, root.Tool)
	}
	at, _ = stepTo(tracker, events, at, agentevent.HookPreToolUse, 2)
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWaitingAgents {
		t.Fatalf("with only its Agent call open the root = %q", got)
	}
	if c := stateOf(t, tracker, child); c.State != agentStateWorking || c.Tool != "Bash" || c.Target == "" {
		t.Fatalf("the child mid-Bash = %+v", c)
	}
	at, _ = stepTo(tracker, events, at, agentevent.HookSubagentStop, 1)
	if c := stateOf(t, tracker, child); c.State != agentStateDone || c.Outcome != "" {
		t.Fatalf("a stopped child awaiting its result = %+v", c)
	}
	at, alerted := stepTo(tracker, events, at, agentevent.HookStop, 1)
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateYourTurn || !alerted {
		t.Fatalf("after Stop the root = %q, alerted %v", got, alerted)
	}
	if c := stateOf(t, tracker, child); c.Outcome != "completed" || c.Tokens != 42191 {
		t.Fatalf("the child's outcome = %+v", c)
	}
	_ = at
}

// Claude stops its turn before a background child finishes. The root waits on
// its agents until the child's result arrives, and only the turn after that
// hands back to the user.
func TestAgentStateWaitsOnABackgroundChildUntilItsOutcome(t *testing.T) {
	events := spikeEvents(t, "run4-bg-hooks")
	tracker := replayed(t, nil)

	at, alerted := stepTo(tracker, events, 0, agentevent.HookStop, 1)
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWaitingAgents || alerted {
		t.Fatalf("after the launching turn's Stop the root = %q, alerted %v", got, alerted)
	}
	at, alerted = stepTo(tracker, events, at, agentevent.HookSubagentStop, 1)
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWaitingAgents || alerted {
		t.Fatalf("with the child stopped but unreported the root = %q, alerted %v", got, alerted)
	}
	at, _ = stepTo(tracker, events, at, agentevent.HookUserPromptSubmit, 1)
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWorking {
		t.Fatalf("while the root reads the result it = %q", got)
	}
	_, alerted = stepTo(tracker, events, at, agentevent.HookStop, 1)
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateYourTurn || !alerted {
		t.Fatalf("after the last Stop the root = %q, alerted %v", got, alerted)
	}
}

func claudeEvent(hook string, fill func(*workbench.AgentEventRequest)) workbench.AgentEventRequest {
	event := workbench.AgentEventRequest{Provider: agentProviderClaude, Generation: 1, Hook: hook, Timestamp: time.Now()}
	if fill != nil {
		fill(&event)
	}
	return event
}

func TestAgentStateAskingAQuestionNeedsYouUntilItIsAnswered(t *testing.T) {
	tracker := replayed(t, []workbench.AgentEventRequest{claudeEvent(agentevent.HookUserPromptSubmit, nil)})
	ask := func(hook string) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.Tool, e.ToolUseID, e.Target = agentevent.ToolAskUserQuestion, "ask-1", "Which branch?"
		})
	}
	if _, alert := tracker.apply(ask(agentevent.HookPreToolUse)); !alert {
		t.Fatal("asking a question did not ask for the chime")
	}
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateNeedsYou || root.Reason != "Which branch?" {
		t.Fatalf("asking root = %+v", root)
	}
	if state := tracker.state(); state != status.ActivityWaiting {
		t.Fatalf("the rail reads %q while the root needs the user", state)
	}
	tracker.apply(ask(agentevent.HookPostToolUse))
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWorking {
		t.Fatalf("after the answer the root = %q", got)
	}

	tracker.apply(claudeEvent(agentevent.HookPreToolUse, func(e *workbench.AgentEventRequest) {
		e.Tool, e.ToolUseID = agentevent.ToolExitPlanMode, "plan-1"
	}))
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateNeedsYou || root.Reason != agentReasonPlan {
		t.Fatalf("plan review root = %+v", root)
	}
	tracker.input()
	if got := stateOf(t, tracker, agentRootID).State; got == agentStateNeedsYou {
		t.Fatal("a keystroke left the root needing the user")
	}
}

func TestAgentStatePermissionAndElicitationNeedYouButAnIdleNagDoesNot(t *testing.T) {
	tracker := replayed(t, []workbench.AgentEventRequest{claudeEvent(agentevent.HookUserPromptSubmit, nil)})
	notify := func(kind string) workbench.AgentEventRequest {
		return claudeEvent(agentevent.HookNotification, func(e *workbench.AgentEventRequest) {
			e.NotificationType, e.Message = kind, "Claude needs your permission to use Bash"
		})
	}
	if changed, _ := tracker.apply(notify("idle_prompt")); changed {
		t.Fatal("an idle nag changed the tree")
	}
	tracker.apply(notify(notificationPermission))
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateNeedsYou || root.Reason != "Claude needs your permission to use Bash" {
		t.Fatalf("permission root = %+v", root)
	}
	tracker.apply(claudeEvent(agentevent.HookUserPromptSubmit, nil))
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWorking {
		t.Fatalf("a prompt left the root at %q", got)
	}
}

func TestAgentStateStopFailureKeepsItsReason(t *testing.T) {
	tracker := replayed(t, []workbench.AgentEventRequest{claudeEvent(agentevent.HookUserPromptSubmit, nil)})
	_, alert := tracker.apply(claudeEvent(agentevent.HookStopFailure, func(e *workbench.AgentEventRequest) {
		e.Message = "rate_limit: 429 from the API"
	}))
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateNeedsYou || root.Reason != "rate_limit: 429 from the API" || !alert {
		t.Fatalf("after StopFailure the root = %+v, alert %v", root, alert)
	}
}

func TestAgentStateAFailedOutcomeKeepsItsStatus(t *testing.T) {
	events := spikeEvents(t, "run3-fg-hooks")
	for i := range events {
		if events[i].Outcome != nil {
			outcome := *events[i].Outcome
			outcome.Status = "error_max_turns"
			events[i].Outcome = &outcome
		}
	}
	child := stateOf(t, replayed(t, events), "ab76fea07a9b71550")
	if child.State != agentStateFailed || child.Reason != "error_max_turns" {
		t.Fatalf("failed child = %+v", child)
	}
}

func TestAgentStateARelaunchEndsWhatWasRunning(t *testing.T) {
	events := spikeEvents(t, "run4-bg-hooks")
	tracker := replayed(t, nil)
	stepTo(tracker, events, 0, agentevent.HookStop, 1)
	tracker.begin(agentProviderClaude, 2)
	view := tracker.snapshot()
	for _, id := range []string{agentRootID, "a5438536af5bc5c5e"} {
		if got := recordForRun(t, view, id, 1).State; got != agentStateEnded {
			t.Fatalf("%s after a relaunch = %q, want ended", id, got)
		}
	}
}

func TestAgentStateRecordsFilesWritten(t *testing.T) {
	tracker := replayed(t, nil)
	for i, tool := range []string{"Edit", "Write", "Edit", "Read"} {
		path := "/w/a.go"
		if tool == "Write" || tool == "Read" {
			path = "/w/b.go"
		}
		id := string(rune('a' + i))
		tracker.apply(claudeEvent(agentevent.HookPreToolUse, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID, e.Target = tool, id, path }))
		tracker.apply(claudeEvent(agentevent.HookPostToolUse, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID, e.Target = tool, id, path }))
	}
	if files := stateOf(t, tracker, agentRootID).Files; len(files) != 2 || files[0] != "/w/a.go" || files[1] != "/w/b.go" {
		t.Fatalf("files written = %v", files)
	}
}

type emitted struct {
	mu     sync.Mutex
	events []string
	last   status.AgentTree
}

func (e *emitted) emit(event string, payload any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, event)
	e.last = payload.(status.AgentTree)
}

func (e *emitted) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.events)
}

func TestAgentTreeEmitCoalescesABurstAndFlushesAfterIt(t *testing.T) {
	reg := testRegistry(t, t.TempDir())
	shown := reg.current()
	var mu sync.Mutex
	version := 0
	out := &emitted{}
	trees := newAgentTrees(out.emit, 50*time.Millisecond, reg.current, func(*sessionState) status.AgentTree {
		mu.Lock()
		defer mu.Unlock()
		return status.AgentTree{Provider: string(rune('a' + version))}
	})
	for range 20 {
		mu.Lock()
		version++
		mu.Unlock()
		trees.schedule(shown)
		time.Sleep(time.Millisecond)
	}
	waitFor(t, "the burst's one push", func() bool { return out.count() >= 1 })
	time.Sleep(150 * time.Millisecond)
	if got := out.count(); got != 1 {
		t.Fatalf("20 changes in a burst pushed %d trees", got)
	}
	if out.last.Provider != string(rune('a'+20)) || out.events[0] != agentTreeEvent+shown.slug() {
		t.Fatalf("pushed %q on %q, want the last change on the session's event", out.last.Provider, out.events[0])
	}

	trees.schedule(shown)
	time.Sleep(150 * time.Millisecond)
	if got := out.count(); got != 1 {
		t.Fatal("an unchanged tree was pushed again")
	}

	mu.Lock()
	version++
	mu.Unlock()
	trees.schedule(shown)
	waitFor(t, "the change after a quiet spell", func() bool { return out.count() == 2 })
}

func TestAgentTreeEmitSkipsASessionNotOnScreen(t *testing.T) {
	reg := testRegistry(t, t.TempDir())
	hidden := reg.add(t.TempDir(), []string{"/bin/cat"}, nil)
	out := &emitted{}
	trees := newAgentTrees(out.emit, time.Millisecond, reg.current, func(*sessionState) status.AgentTree { return status.AgentTree{} })
	trees.schedule(hidden)
	time.Sleep(30 * time.Millisecond)
	if out.count() != 0 {
		t.Fatal("a background session's tree was pushed")
	}
}

// Async hooks race: a Stop that reaches the workbench after a newer prompt
// must not end the turn the prompt began, nor ring for it.
func TestAgentStateIgnoresAStopOlderThanTheLatestPrompt(t *testing.T) {
	start := time.Now()
	at := func(offset time.Duration, hook string) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) { e.Timestamp = start.Add(offset) })
	}
	tracker := replayed(t, []workbench.AgentEventRequest{at(0, agentevent.HookUserPromptSubmit)})
	tracker.apply(at(2*time.Second, agentevent.HookUserPromptSubmit))
	if _, alert := tracker.apply(at(time.Second, agentevent.HookStop)); alert {
		t.Fatal("a stale Stop rang the chime")
	}
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateWorking {
		t.Fatalf("after a stale Stop the root = %q", got)
	}
	tracker.apply(at(3*time.Second, agentevent.HookStop))
	tracker.apply(at(2500*time.Millisecond, agentevent.HookUserPromptSubmit))
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateYourTurn {
		t.Fatalf("a stale prompt reopened the turn: %q", got)
	}
}

func TestAgentStateAnInterruptEndsTheTurn(t *testing.T) {
	tracker := replayed(t, []workbench.AgentEventRequest{claudeEvent(agentevent.HookUserPromptSubmit, nil)})
	tracker.apply(claudeEvent(agentevent.HookPreToolUse, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Bash", "t1" }))
	tracker.apply(claudeEvent(agentevent.HookPostToolUseFailure, func(e *workbench.AgentEventRequest) {
		e.Tool, e.ToolUseID, e.Interrupt = "Bash", "t1", true
	}))
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateYourTurn {
		t.Fatalf("after an interrupt the root = %q", got)
	}
}

func TestAgentStateAFailedAgentCallFailsItsChild(t *testing.T) {
	spawn := func(hook string) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.Tool, e.ToolUseID, e.Spawn, e.Message = "Agent", "t1", &workbench.AgentSpawn{Type: "explorer"}, "tool failed"
		})
	}
	tracker := replayed(t, []workbench.AgentEventRequest{
		claudeEvent(agentevent.HookUserPromptSubmit, nil),
		spawn(agentevent.HookPreToolUse),
		claudeEvent(agentevent.HookSubagentStart, func(e *workbench.AgentEventRequest) { e.AgentID, e.AgentType = "c1", "explorer" }),
		spawn(agentevent.HookPostToolUseFailure),
		claudeEvent(agentevent.HookStop, nil),
	})
	if c := stateOf(t, tracker, "c1"); c.State != agentStateFailed || c.Reason != "tool failed" {
		t.Fatalf("child of a failed call = %+v", c)
	}
	if got := stateOf(t, tracker, agentRootID).State; got != agentStateYourTurn {
		t.Fatalf("the root waits on a child whose call failed: %q", got)
	}
}

// A permission prompt carrying a child's agent id is the root's to show; the
// child's next tool call, its permitted one returning, clears it.
func TestAgentStateAChildsPermissionPromptIsTheRoots(t *testing.T) {
	sub := func(hook string, fill func(*workbench.AgentEventRequest)) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) {
			e.AgentID, e.AgentType = "c1", "explorer"
			if fill != nil {
				fill(e)
			}
		})
	}
	tracker := replayed(t, []workbench.AgentEventRequest{
		claudeEvent(agentevent.HookUserPromptSubmit, nil),
		sub(agentevent.HookSubagentStart, nil),
		sub(agentevent.HookPreToolUse, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Bash", "t9" }),
	})
	_, alert := tracker.apply(sub(agentevent.HookNotification, func(e *workbench.AgentEventRequest) {
		e.NotificationType, e.Message = notificationPermission, "allow Bash?"
	}))
	if c := stateOf(t, tracker, "c1"); c.State != agentStateWorking || c.Tool != "Bash" {
		t.Fatalf("child awaiting permission = %+v, want working in Bash", c)
	}
	if root := stateOf(t, tracker, agentRootID); root.State != agentStateNeedsYou || root.Reason != "allow Bash?" || !alert {
		t.Fatalf("root = %+v, alert %v", root, alert)
	}
	tracker.apply(sub(agentevent.HookPostToolUse, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = "Bash", "t9" }))
	if got := stateOf(t, tracker, agentRootID).State; got == agentStateNeedsYou {
		t.Fatal("the permitted call returned and the root still needs the user")
	}
}

func TestAgentStateAnAskAnsweredBeforeItsStartArrivesNeedsNothing(t *testing.T) {
	ask := func(hook string) workbench.AgentEventRequest {
		return claudeEvent(hook, func(e *workbench.AgentEventRequest) { e.Tool, e.ToolUseID = agentevent.ToolAskUserQuestion, "ask-1" })
	}
	tracker := replayed(t, []workbench.AgentEventRequest{
		claudeEvent(agentevent.HookUserPromptSubmit, nil), ask(agentevent.HookPostToolUse), ask(agentevent.HookPreToolUse),
	})
	if got := stateOf(t, tracker, agentRootID).State; got == agentStateNeedsYou {
		t.Fatal("a late ask start left the root needing the user")
	}
}

func TestAgentStateDropsANotificationForNoKnownAgent(t *testing.T) {
	tracker := replayed(t, []workbench.AgentEventRequest{claudeEvent(agentevent.HookUserPromptSubmit, func(e *workbench.AgentEventRequest) {
		e.Notifications = []workbench.TaskNotification{{TaskID: "shell-1", Outcome: workbench.AgentOutcome{Status: "completed"}}}
	})})
	if kids := children(tracker.snapshot()); len(kids) != 0 {
		t.Fatalf("a notification for no known agent made %+v", kids)
	}
}
