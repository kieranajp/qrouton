package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// Runner ids as qrouton launches them: a misspelling reads as a runner it does
// not launch, and the capability tests below stop passing.
const (
	agentProviderClaude   = "claude"
	agentProviderCodex    = "codex"
	agentProviderOpenCode = "opencode"
	agentProviderAgy      = "agy"
)

type activityClock struct{ at time.Time }

func (c *activityClock) now() time.Time { return c.at }

func TestDelegatedLifecycleIsTerminalAndOutOfOrderStopsMakeTombstones(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now)
	if !tracker.begin(agentProviderClaude, 1) {
		t.Fatal("first generation was rejected")
	}
	start := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "agent-1", Type: "qrouton-research-lead", Timestamp: clock.now(),
	}
	if !tracker.lifecycle(start) || tracker.snapshot().Active != 2 {
		t.Fatalf("start left %d active records, want root and child", tracker.snapshot().Active)
	}
	if tracker.lifecycle(start) {
		t.Fatal("duplicate start changed the tracker")
	}
	if record := delegatedRecord(t, tracker.snapshot(), "agent-1"); !record.ParentKnown || record.ParentID != agentRootID {
		t.Fatalf("known parent was lost: %+v", record)
	}
	clock.at = clock.at.Add(time.Second)
	stop := start
	stop.Kind = workbench.LifecycleStop
	if !tracker.lifecycle(stop) || tracker.snapshot().Active != 1 {
		t.Fatalf("stop left %d active records, want only root", tracker.snapshot().Active)
	}
	finishedAt := delegatedRecord(t, tracker.snapshot(), "agent-1").FinishedAt
	clock.at = clock.at.Add(time.Second)
	if tracker.lifecycle(stop) {
		t.Fatal("duplicate stop changed the tracker")
	}
	if got := delegatedRecord(t, tracker.snapshot(), "agent-1").FinishedAt; !got.Equal(finishedAt) {
		t.Fatalf("duplicate stop moved the finish from %s to %s", finishedAt, got)
	}
	if tracker.lifecycle(start) {
		t.Fatal("a delayed start resurrected a stopped agent")
	}

	tombstone := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStop,
		ID: "agent-2", Type: "Explore",
	}
	if !tracker.lifecycle(tombstone) {
		t.Fatal("unmatched stop did not create a tombstone")
	}
	record := delegatedRecord(t, tracker.snapshot(), "agent-2")
	if record.State != agentStateDone || !record.StartedAt.IsZero() {
		t.Fatalf("tombstone = %+v", record)
	}
	tombstone.Kind = workbench.LifecycleStart
	if tracker.lifecycle(tombstone) {
		t.Fatal("late start resurrected an unmatched stop")
	}
}

func TestFinishedAgentsStayForTheRunAndTheOneAfter(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 12, 30, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderCodex, 1)
	event := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderCodex, Generation: 1, Kind: workbench.LifecycleStart, ID: "agent-1", Type: "explorer",
	}
	tracker.lifecycle(event)
	event.Kind = workbench.LifecycleStop
	tracker.lifecycle(event)
	clock.at = clock.at.Add(time.Hour)
	if _, ok := findRecord(tracker.snapshot(), "agent-1", 1); !ok {
		t.Fatal("a finished agent left the tree within its run")
	}
	tracker.begin(agentProviderCodex, 2)
	if _, ok := findRecord(tracker.snapshot(), "agent-1", 1); !ok {
		t.Fatal("the previous run's tree went at the first relaunch")
	}
	tracker.begin(agentProviderCodex, 3)
	if _, ok := findRecord(tracker.snapshot(), "agent-1", 1); ok {
		t.Fatal("a run two relaunches old is still kept")
	}
}

// A lifecycle-only child hangs off the agent its parent id names when the
// tracker knows that agent, and off the root otherwise. Its type is never read
// for rank.
func TestLifecycleChildHangsOffAKnownParentOrTheRoot(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderCodex, 1)
	for _, event := range []workbench.DelegatedLifecycleRequest{
		{ID: "lead-1", Type: "qrouton-research-lead", ParentID: "session-1"},
		{ID: "specialist-1", Type: "explorer", ParentID: "lead-1"},
		{ID: "specialist-2", Type: "reviewer-lead", ParentID: "unknown"},
	} {
		event.Provider, event.Generation, event.Kind = agentProviderCodex, 1, workbench.LifecycleStart
		tracker.lifecycle(event)
	}
	for id, parent := range map[string]string{"lead-1": agentRootID, "specialist-1": "lead-1", "specialist-2": agentRootID} {
		if record := delegatedRecord(t, tracker.snapshot(), id); !record.ParentKnown || record.ParentID != parent {
			t.Fatalf("%s = %+v, want parent %s", id, record, parent)
		}
	}
}

func TestGenerationAdvanceFinishesTheOldRunAndRejectsItsLateEvents(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderClaude, 1)
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "agent-1", Type: "qrouton-planning-lead",
	})
	tracker.attention(1, status.ActivityWaiting)
	clock.at = clock.at.Add(time.Second)
	if !tracker.begin(agentProviderClaude, 2) {
		t.Fatal("new generation was rejected")
	}
	view := tracker.snapshot()
	if !view.Running || view.Unread || tracker.snapshot().Active != 1 {
		t.Fatalf("new generation snapshot = %+v, active %d", view, tracker.snapshot().Active)
	}
	for _, id := range []string{agentRootID, "agent-1"} {
		old := recordForRun(t, view, id, 1)
		if old.State != agentStateEnded || old.FinishedAt.IsZero() {
			t.Fatalf("old %s = %+v", id, old)
		}
	}
	late := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart, ID: "ghost",
	}
	if changed, _ := tracker.attention(1, status.ActivityWaiting); tracker.lifecycle(late) || changed {
		t.Fatal("old-generation hook was accepted")
	}
	if _, ok := findRecord(tracker.snapshot(), "ghost", 1); ok {
		t.Fatal("old-generation start created a ghost record")
	}
	if tracker.begin(agentProviderClaude, 2) {
		t.Fatal("duplicate generation announcement restarted the run")
	}
}

func TestRootExitEndsItsChildrenAndKeepsThem(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderClaude, 1)
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "agent-1", Type: "Explore",
	})
	tracker.attention(1, status.ActivityWaiting)
	clock.at = clock.at.Add(time.Second)
	if !tracker.exitWithProvider("", 9) {
		t.Fatal("root exit was ignored")
	}
	view := tracker.snapshot()
	if view.Running || view.Unread || tracker.snapshot().Active != 0 {
		t.Fatalf("exit snapshot = %+v, active %d", view, tracker.snapshot().Active)
	}
	if root := recordForRun(t, view, agentRootID, 1); root.State != agentStateFailed {
		t.Fatalf("failed root = %+v", root)
	}
	if child := delegatedRecord(t, view, "agent-1"); child.State != agentStateEnded {
		t.Fatalf("a child the exit closed = %+v, want ended with the run", child)
	}
	clock.at = clock.at.Add(time.Hour)
	if got := len(tracker.snapshot().Records); got != 2 {
		t.Fatalf("records an hour after exit = %d, want both kept", got)
	}
}

func TestNewSupervisorRunCoexistsWithRetainedPriorGeneration(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderCodex, 88)
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderCodex, Generation: 88, Kind: workbench.LifecycleStart, ID: "reused-id",
	})
	tracker.exitWithProvider("", 0)
	clock.at = clock.at.Add(time.Second)
	if !tracker.begin(agentProviderCodex, 7) {
		t.Fatal("a new supervisor generation was mistaken for an old numeric generation")
	}
	if !tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderCodex, Generation: 7, Kind: workbench.LifecycleStart, ID: "reused-id",
	}) {
		t.Fatal("provider ID could not be reused in the new run")
	}
	view := tracker.snapshot()
	if len(view.Records) != 4 {
		t.Fatalf("coexisting runs have %d records, want two roots and two children: %+v", len(view.Records), view.Records)
	}
	if recordForRun(t, view, agentRootID, 7).State != agentStateIdle ||
		recordForRun(t, view, agentRootID, 88).State != agentStateDone {
		t.Fatalf("coexisting roots = %+v", view.Records)
	}
}

func TestPreGenerationExitRetainsAnExplicitSetupRunAlongsideTheLaterRun(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now)
	if !tracker.exitWithProvider(agentProviderCodex, 7) {
		t.Fatal("pre-generation exit was ignored")
	}
	setup := recordForRunID(t, tracker.snapshot(), agentRootID, agentSetupRunPrefix+"1")
	if setup.Provider != agentProviderCodex || setup.Generation != 0 || setup.State != agentStateFailed ||
		setup.StartedAt.IsZero() || !setup.FinishedAt.Equal(setup.StartedAt) {
		t.Fatalf("setup root = %+v", setup)
	}

	clock.at = clock.at.Add(time.Second)
	if !tracker.begin(agentProviderCodex, 42) {
		t.Fatal("later announced run was rejected")
	}
	view := tracker.snapshot()
	if !view.Running || view.Provider != agentProviderCodex || len(view.Records) != 2 {
		t.Fatalf("coexisting setup and announced runs = %+v", view)
	}
	if current := recordForRun(t, view, agentRootID, 42); current.State != agentStateIdle {
		t.Fatalf("announced root = %+v", current)
	}
	if retained := recordForRunID(t, view, agentRootID, agentSetupRunPrefix+"1"); retained.State != agentStateFailed {
		t.Fatalf("retained setup root = %+v", retained)
	}
}

func TestTrackerMapsCapabilitiesAndRootOutputWithoutParsingIt(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderClaude, 1)
	tracker.output()
	if root := recordForRun(t, tracker.snapshot(), agentRootID, 1); root.State != agentStateYourTurn {
		t.Fatalf("a Claude root's terminal output moved it off your turn: %+v", root)
	}
	tracker.begin(agentProviderOpenCode, 9)
	tracker.output()
	root := recordForRun(t, tracker.snapshot(), agentRootID, 9)
	if root.State != agentStateWorking {
		t.Fatalf("root after output = %+v", root)
	}
	clock.at = clock.at.Add(activityQuiet)
	if root := recordForRun(t, tracker.snapshot(), agentRootID, 9); root.State != agentStateIdle {
		t.Fatalf("root at quiet boundary = %+v", root)
	}
	if caps := capabilitiesFor(agentProviderClaude); !caps.Attention || !caps.Children || !caps.Parents || !caps.Outcomes {
		t.Fatalf("Claude capabilities = %+v", caps)
	}
	tracker.begin(agentProviderCodex, 2)
	if caps := tracker.snapshot().Capabilities; caps.Attention || !caps.Children || caps.Parents || caps.Outcomes {
		t.Fatalf("Codex capabilities = %+v", caps)
	}
	tracker.begin(agentProviderOpenCode, 3)
	if caps := tracker.snapshot().Capabilities; caps.Attention || caps.Children || caps.Parents || caps.Outcomes {
		t.Fatalf("OpenCode capabilities = %+v", caps)
	}
	tracker.begin(agentProviderAgy, 4)
	if caps := tracker.snapshot().Capabilities; caps.Attention || caps.Children || caps.Parents || caps.Outcomes {
		t.Fatalf("Agy capabilities = %+v", caps)
	}
}

func TestSessionSwitchKeepsRecentActivityAndCleanupDropsIt(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now)
	reg.boot.root = func(slug string) string { return filepath.Join(root, slug) }
	reg.boot.cleanup = func(string) error { return nil }
	reg.boot.teardown = func(*sessionState) {}
	alpha := reg.add(filepath.Join(root, "alpha"), []string{"/bin/cat"}, os.Environ())
	beta := reg.add(filepath.Join(root, "beta"), []string{"/bin/cat"}, os.Environ())
	alpha.agents.begin(agentProviderClaude, 1)
	reg.reveal(alpha)
	reg.reveal(beta)
	if got, ok := reg.agentActivitySnapshots()["alpha"]; !ok || !got.Running {
		t.Fatalf("switching sessions reset alpha activity: %+v", got)
	}
	reg.retire(alpha)
	if got, ok := reg.agentActivitySnapshots()["alpha"]; !ok || !got.Running {
		t.Fatalf("retirement discarded retained activity: %+v", got)
	}
	if err := reg.Cleanup("alpha"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.agentActivitySnapshots()["alpha"]; ok {
		t.Fatal("cleanup retained session activity")
	}
}

func TestLifecycleSocketMutatesOnlyItsOwningSession(t *testing.T) {
	windows, _ := testWindows(t)
	clock := &activityClock{at: time.Now()}
	trackers := []*agentActivity{
		newAgentActivity(clock.now),
		newAgentActivity(clock.now),
	}
	for i, tracker := range trackers {
		socket, err := workbench.NewSocketPath()
		if err != nil {
			t.Fatal(err)
		}
		owner := windows.sessions.add(filepath.Join(t.TempDir(), "session"), []string{"/bin/cat"}, os.Environ())
		server, err := serveControl(socket, windows, owner, controlHooks{
			generation: tracker.beginRequest,
			lifecycle:  func(req workbench.DelegatedLifecycleRequest) { tracker.lifecycle(req) },
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { server.Close() })
		if i == 0 {
			handle := workbench.Handle{Socket: socket, SessionRoot: owner.root()}
			if err := handle.RunnerGeneration(context.Background(), agentProviderClaude, 1); err != nil {
				t.Fatal(err)
			}
			if err := handle.DelegatedLifecycle(context.Background(), workbench.DelegatedLifecycleRequest{
				Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart, ID: "agent-1",
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if trackers[0].snapshot().Active != 2 || trackers[1].snapshot().Active != 0 {
		t.Fatalf("active counts = %d, %d", trackers[0].snapshot().Active, trackers[1].snapshot().Active)
	}
}

func (a *agentActivity) beginRequest(req workbench.RunnerGenerationRequest) {
	a.begin(req.Provider, req.Generation)
}

func delegatedRecord(t *testing.T, snapshot agentActivitySnapshot, id string) agentRecord {
	t.Helper()
	for _, record := range snapshot.Records {
		if !record.Root && record.ID == id {
			return record
		}
	}
	t.Fatalf("no delegated record %q in %+v", id, snapshot.Records)
	return agentRecord{}
}

func recordForRun(t *testing.T, snapshot agentActivitySnapshot, id string, generation uint64) agentRecord {
	t.Helper()
	if record, ok := findRecord(snapshot, id, generation); ok {
		return record
	}
	t.Fatalf("no record %q generation %d in %+v", id, generation, snapshot.Records)
	return agentRecord{}
}

func findRecord(snapshot agentActivitySnapshot, id string, generation uint64) (agentRecord, bool) {
	for _, record := range snapshot.Records {
		if record.ID == id && record.Generation == generation {
			return record, true
		}
	}
	return agentRecord{}, false
}

func recordForRunID(t *testing.T, snapshot agentActivitySnapshot, id, runID string) agentRecord {
	t.Helper()
	for _, record := range snapshot.Records {
		if record.ID == id && record.RunID == runID {
			return record
		}
	}
	t.Fatalf("no record %q run %q in %+v", id, runID, snapshot.Records)
	return agentRecord{}
}

func TestRootActivityRanksTheHookOverTheClock(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now)
	if got := tracker.state(); got != status.ActivityIdle {
		t.Fatalf("a terminal that has never spoken = %q, want idle", got)
	}

	tracker.output()
	if got := tracker.state(); got != status.ActivityWorking {
		t.Fatalf("output before any generation = %q, want working", got)
	}

	tracker.begin(agentProviderCodex, 1)
	if got := tracker.state(); got != status.ActivityIdle {
		t.Fatalf("a new run left stale state %q, want idle", got)
	}

	// An agent drawing a permission prompt is producing output, so waiting has
	// to beat working rather than merely arrive after it.
	tracker.attention(1, status.ActivityWaiting)
	tracker.output()
	if got := tracker.state(); got != status.ActivityWaiting {
		t.Fatalf("hook says waiting but state = %q", got)
	}

	tracker.input()
	if got := tracker.state(); got != status.ActivityWorking {
		t.Fatalf("after typing = %q, want working", got)
	}

	tracker.attention(1, status.ActivityWorking)
	if got := tracker.state(); got != status.ActivityWorking {
		t.Fatalf("a subagent starting = %q, want working", got)
	}

	clock.at = clock.at.Add(activityQuiet)
	if got := tracker.state(); got != status.ActivityIdle {
		t.Fatalf("after silence = %q, want idle", got)
	}

	tracker.output()
	tracker.attention(1, status.ActivityWaiting)
	tracker.exitWithProvider(agentProviderCodex, 0)
	if got := tracker.state(); got != status.ActivityIdle {
		t.Fatalf("a root exit left stale state %q, want idle", got)
	}

	tracker.output()
	if got := tracker.state(); got != status.ActivityWorking {
		t.Fatalf("a terminal still writing after its supervisor exited = %q, want working", got)
	}
}

func TestTheControlSocketCarriesTheAttentionSignal(t *testing.T) {
	windows, _ := testWindows(t)
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now)
	tracker.begin(agentProviderClaude, 1)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{
		attention: func(value string, generation uint64) { tracker.attention(generation, value) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	handle := workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}
	if err := handle.Attention(context.Background(), status.ActivityWaiting, 1); err != nil {
		t.Fatal(err)
	}
	if got := tracker.state(); got != status.ActivityWaiting {
		t.Fatalf("state after the hook = %q, want waiting", got)
	}
}

// A session's tracker outlives the session, so what the rail says about a
// reopened one is the new run's activity and never the retired run's.
func TestAReopenedSessionRejoinsTheRailIdle(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now)
	reg.boot.teardown = func(*sessionState) {}
	first := reg.add(filepath.Join(root, "alpha"), []string{"/bin/cat"}, os.Environ())
	first.agents.begin(agentProviderClaude, 1)
	first.agents.output()
	first.agents.exitWithProvider(agentProviderClaude, 0)
	reg.retire(first)

	second := reg.add(filepath.Join(root, "alpha"), []string{"/bin/cat"}, os.Environ())
	if second.agents != first.agents {
		t.Fatal("a reopened session was given a tracker without its retained runs")
	}
	if got := second.agents.state(); got != status.ActivityIdle {
		t.Fatalf("a reopened session rejoins the rail as %q, want idle", got)
	}
}

type chimes struct{ rung []string }

func recordChimes(t *testing.T) *chimes {
	t.Helper()
	c := &chimes{}
	original := chime
	chime = func(script string) { c.rung = append(c.rung, script) }
	t.Cleanup(func() { chime = original })
	return c
}

func chimingSession(t *testing.T, cfg *config.Config, watched func(*sessionState) bool) (*sessionState, func(string, uint64), func()) {
	t.Helper()
	reg := newSessionsWithActivity((&activityClock{at: time.Now()}).now)
	state := reg.add(sessionDir(t, t.TempDir(), "octopus"), []string{"/bin/cat"}, os.Environ())
	state.agents.begin(agentProviderCodex, 1)
	ring := ringer(cfg, state, watched)
	notify := attend(state, reg.current, func() {}, ring)
	return state, func(value string, generation uint64) { notify(state.agents.attention(generation, value)) }, ring
}

// An orchestrator ends a turn each time a background lead reports back, so only
// the turn that leaves nothing delegated running is worth a chime.
func TestATurnEndingChimesOnlyOnceNothingDelegatedIsRunning(t *testing.T) {
	chimed := recordChimes(t)
	state, hook, _ := chimingSession(t, &config.Config{}, unwatched)
	lead := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderCodex, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "agent-1", Type: "qrouton-research-lead",
	}
	state.agents.lifecycle(lead)

	hook(status.ActivityTurnEnded, 1)
	if len(chimed.rung) != 0 || state.agents.state() == status.ActivityWaiting {
		t.Fatalf("a turn ending with a lead still running chimed %v and reads %q", chimed.rung, state.agents.state())
	}
	lead.Kind = workbench.LifecycleStop
	state.agents.lifecycle(lead)
	hook(status.ActivityTurnEnded, 1)
	if want := sessionpaths.NotifyScript(state.root()); len(chimed.rung) != 1 || chimed.rung[0] != want {
		t.Fatalf("chimed %v, want %s once", chimed.rung, want)
	}
	if got := state.agents.state(); got != status.ActivityWaiting {
		t.Fatalf("a settled turn reads %q, want waiting", got)
	}
}

// A turn ending, Claude's idle nag after it and a notify tab all mean the same
// wait, so the session chimes once until the user types.
func TestASessionChimesOnceUntilTheUserTypes(t *testing.T) {
	chimed := recordChimes(t)
	state, hook, ring := chimingSession(t, &config.Config{}, unwatched)

	hook(status.ActivityTurnEnded, 1)
	hook(status.ActivityWaiting, 1)
	ring()
	if len(chimed.rung) != 1 {
		t.Fatalf("one wait chimed %d times", len(chimed.rung))
	}
	state.agents.input()
	hook(status.ActivityWaiting, 1)
	if len(chimed.rung) != 2 {
		t.Fatalf("a permission prompt after the user typed chimed %d times in all, want 2", len(chimed.rung))
	}
}

func unwatched(*sessionState) bool { return false }

// The session in front of the user needs no chime to be noticed; the wait still
// registers, so the rail and the next unwatched wait behave as before.
func TestTheWatchedSessionDoesNotChime(t *testing.T) {
	chimed := recordChimes(t)
	watching := true
	state, hook, _ := chimingSession(t, &config.Config{}, func(*sessionState) bool { return watching })

	hook(status.ActivityTurnEnded, 1)
	if len(chimed.rung) != 0 {
		t.Fatalf("the watched session chimed %v", chimed.rung)
	}
	if got := state.agents.state(); got != status.ActivityWaiting {
		t.Fatalf("a watched wait reads %q, want waiting", got)
	}
	watching = false
	hook(status.ActivityWaiting, 1)
	if len(chimed.rung) != 1 {
		t.Fatalf("a wait after looking away chimed %d times, want 1", len(chimed.rung))
	}
}
func TestAQuietConfigNeverChimes(t *testing.T) {
	chimed := recordChimes(t)
	state, hook, ring := chimingSession(t, &config.Config{Quiet: true}, unwatched)

	hook(status.ActivityTurnEnded, 1)
	ring()
	if len(chimed.rung) != 0 {
		t.Fatalf("a quiet config chimed %v", chimed.rung)
	}
	if got := state.agents.state(); got != status.ActivityWaiting {
		t.Fatalf("quiet also dropped the waiting marker: %q", got)
	}
}

func TestASessionIsUnreadUntilShownAndNeverWhileShown(t *testing.T) {
	reg := newSessionsWithActivity((&activityClock{at: time.Now()}).now)
	background := reg.add(sessionDir(t, t.TempDir(), "background"), []string{"/bin/cat"}, os.Environ())
	onscreen := reg.add(sessionDir(t, t.TempDir(), "onscreen"), []string{"/bin/cat"}, os.Environ())
	background.agents.begin(agentProviderCodex, 1)
	onscreen.agents.begin(agentProviderCodex, 1)
	reg.reveal(onscreen)
	hookFor := func(state *sessionState) func(string, uint64) {
		notify := attend(state, reg.current, func() {}, func() {})
		return func(value string, generation uint64) { notify(state.agents.attention(generation, value)) }
	}

	hookFor(background)(status.ActivityTurnEnded, 1)
	hookFor(onscreen)(status.ActivityTurnEnded, 1)
	if !background.agents.snapshot().Unread {
		t.Fatal("a background turn ending left the session read")
	}
	if onscreen.agents.snapshot().Unread {
		t.Fatal("the session on screen was marked unread")
	}
	if got := onscreen.agents.state(); got != status.ActivityWaiting {
		t.Fatalf("the session on screen reads %q, want waiting", got)
	}

	reg.reveal(background)
	if background.agents.snapshot().Unread {
		t.Fatal("showing the session left it unread")
	}
	if got := background.agents.state(); got != status.ActivityWaiting {
		t.Fatalf("showing the session changed its activity to %q", got)
	}
}

func TestOpeningAnAttentionTabRingsTheSession(t *testing.T) {
	windows, _ := testWindows(t)
	rung := 0
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{ring: func() { rung++ }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	handle := workbench.Handle{Socket: socket, SessionRoot: windows.shown().root()}
	for _, attention := range []bool{false, true} {
		if _, err := handle.WindowHost().Open(context.Background(), workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "🔔", Content: "done", Attention: attention,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if rung != 1 {
		t.Fatalf("rang %d times, want once for the attention tab alone", rung)
	}
}

func TestTerminalRepliesAreNotTheUserTyping(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  string
		reply bool
	}{
		{"focus in", "\x1b[I", true},
		{"focus out", "\x1b[O", true},
		{"mouse move", "\x1b[<35;10;5M", true},
		{"scroll", "\x1b[<64;10;5M", true},
		{"click release", "\x1b[<0;10;5m", true},
		{"x10 click", "\x1b[M !!", true},
		{"cursor position", "\x1b[24;80R", true},
		{"status report", "\x1b[0n", true},
		{"primary attributes", "\x1b[?1;2c", true},
		{"secondary attributes", "\x1b[>0;276;0c", true},
		{"mode report", "\x1b[?2004;1$y", true},
		{"window size", "\x1b[8;24;80t", true},
		{"background colour", "\x1b]11;rgb:1e1e/1e1e/2e2e\x1b\\", true},
		{"colour by bell", "\x1b]10;rgb:cdcd/d6d6/f4f4\x07", true},
		{"xtversion", "\x1bP>|xterm.js(5.5.0)\x1b\\", true},
		{"two replies at once", "\x1b[O\x1b[24;80R", true},
		{"a letter", "y", false},
		{"enter", "\r", false},
		{"an arrow key", "\x1b[A", false},
		{"escape", "\x1b", false},
		{"a paste", "\x1b[200~ls\x1b[201~", false},
		{"a reply then a keystroke", "\x1b[Oy", false},
	} {
		if got := terminalReplies.MatchString(tc.data); got != tc.reply {
			t.Errorf("%s %q: reply = %v, want %v", tc.name, tc.data, got, tc.reply)
		}
	}
}

// Switching away from a window whose runner asked for focus reports writes to
// the PTY, but it is not the user answering the chime.
func TestAFocusReportLeavesTheChimeClaimed(t *testing.T) {
	chimed := recordChimes(t)
	state, hook, _ := chimingSession(t, &config.Config{}, unwatched)

	hook(status.ActivityTurnEnded, 1)
	_ = state.write([]byte("\x1b[O"))
	hook(status.ActivityWaiting, 1)
	if len(chimed.rung) != 1 {
		t.Fatalf("a focus report re-armed the chime: rang %d times", len(chimed.rung))
	}
	if got := state.agents.state(); got != status.ActivityWaiting {
		t.Fatalf("a focus report cleared the waiting marker: %q", got)
	}
	_ = state.write([]byte("y"))
	hook(status.ActivityWaiting, 1)
	if len(chimed.rung) != 2 {
		t.Fatalf("a keystroke did not re-arm the chime: rang %d times", len(chimed.rung))
	}
}
