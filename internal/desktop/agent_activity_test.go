package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type activityClock struct{ at time.Time }

func (c *activityClock) now() time.Time { return c.at }

func TestDelegatedLifecycleIsTerminalAndOutOfOrderStopsMakeTombstones(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now, 10*time.Second)
	if !tracker.begin(agentProviderClaude, 1) {
		t.Fatal("first generation was rejected")
	}
	start := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "agent-1", Type: "qrouton-research-lead", Timestamp: clock.now(),
	}
	if !tracker.lifecycle(start) || tracker.activeCount() != 2 {
		t.Fatalf("start left %d active records, want root and child", tracker.activeCount())
	}
	if tracker.lifecycle(start) {
		t.Fatal("duplicate start changed the tracker")
	}
	if record := delegatedRecord(t, tracker.snapshot(), "agent-1"); !record.ParentKnown || record.ParentID != agentRootID {
		t.Fatalf("known parent was lost: %+v", record)
	}
	if tracker.snapshot().Capabilities.Parents {
		t.Fatal("one known parent was promoted to provider-wide parent coverage")
	}
	clock.at = clock.at.Add(time.Second)
	stop := start
	stop.Kind = workbench.LifecycleStop
	if !tracker.lifecycle(stop) || tracker.activeCount() != 1 {
		t.Fatalf("stop left %d active records, want only root", tracker.activeCount())
	}
	finishedAt := delegatedRecord(t, tracker.snapshot(), "agent-1").FinishedAt
	clock.at = clock.at.Add(time.Second)
	if tracker.lifecycle(stop) {
		t.Fatal("duplicate stop changed the tracker")
	}
	if got := delegatedRecord(t, tracker.snapshot(), "agent-1").FinishedAt; !got.Equal(finishedAt) {
		t.Fatalf("duplicate stop extended retention from %s to %s", finishedAt, got)
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
	if record.State != agentStateFinished || record.Role != agentRoleSpecialist || !record.StartedAt.IsZero() {
		t.Fatalf("tombstone = %+v", record)
	}
	tombstone.Kind = workbench.LifecycleStart
	if tracker.lifecycle(tombstone) {
		t.Fatal("late start resurrected an unmatched stop")
	}
}

func TestDelegatedStopRemainsTerminalAfterItsDisplayRetentionExpires(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 12, 30, 0, 0, time.UTC)}
	retention := 10 * time.Second
	tracker := newAgentActivity(clock.now, retention)
	tracker.begin(agentProviderClaude, 1)
	event := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "agent-1", Type: "explorer",
	}
	if !tracker.lifecycle(event) {
		t.Fatal("start was rejected")
	}
	event.Kind = workbench.LifecycleStop
	if !tracker.lifecycle(event) {
		t.Fatal("stop was rejected")
	}
	clock.at = clock.at.Add(retention)
	if _, ok := findRecord(tracker.snapshot(), event.ID, event.Generation); ok {
		t.Fatal("finished record survived its display retention")
	}
	event.Kind = workbench.LifecycleStart
	if tracker.lifecycle(event) {
		t.Fatal("a delayed start resurrected an expired stopped agent")
	}
	event.Kind = workbench.LifecycleStop
	if tracker.lifecycle(event) {
		t.Fatal("a duplicate stop recreated an expired tombstone")
	}
	if got := tracker.activeCount(); got != 1 {
		t.Fatalf("active records = %d, want only the root", got)
	}
	if _, ok := findRecord(tracker.snapshot(), event.ID, event.Generation); ok {
		t.Fatal("late lifecycle events restarted display retention")
	}
}

func TestSpecialistUsesTheSoleActiveLeadWithoutInventingAnAmbiguousParent(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now, time.Minute)
	tracker.begin(agentProviderClaude, 1)
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "lead-1", Type: "qrouton-research-lead",
	})
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "specialist-1", Type: "explorer",
	})
	if record := delegatedRecord(t, tracker.snapshot(), "specialist-1"); !record.ParentKnown || record.ParentID != "lead-1" {
		t.Fatalf("specialist under one active lead = %+v", record)
	}

	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "lead-2", Type: "qrouton-planning-lead",
	})
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "specialist-2", Type: "reviewer",
	})
	if record := delegatedRecord(t, tracker.snapshot(), "specialist-2"); record.ParentKnown || record.ParentID != "" {
		t.Fatalf("specialist under two active leads = %+v", record)
	}
}

func TestGenerationAdvanceFinishesTheOldRunAndRejectsItsLateEvents(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now, time.Minute)
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
	if !view.Running || view.Attention || tracker.activeCount() != 1 {
		t.Fatalf("new generation snapshot = %+v, active %d", view, tracker.activeCount())
	}
	for _, id := range []string{agentRootID, "agent-1"} {
		old := recordForRun(t, view, id, 1)
		if old.State != agentStateFinished || old.FinishedAt.IsZero() {
			t.Fatalf("old %s = %+v", id, old)
		}
	}
	late := workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart, ID: "ghost",
	}
	if tracker.lifecycle(late) || tracker.attention(1, status.ActivityWaiting) {
		t.Fatal("old-generation hook was accepted")
	}
	if _, ok := findRecord(tracker.snapshot(), "ghost", 1); ok {
		t.Fatal("old-generation start created a ghost record")
	}
	if tracker.begin(agentProviderClaude, 2) {
		t.Fatal("duplicate generation announcement restarted the run")
	}
}

func TestRootExitFinalizesChildrenAndRetentionPrunesAtTheExactBoundary(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)}
	retention := 10 * time.Second
	tracker := newAgentActivity(clock.now, retention)
	tracker.begin(agentProviderClaude, 1)
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "agent-1", Type: "Explore",
	})
	tracker.attention(1, status.ActivityWaiting)
	clock.at = clock.at.Add(time.Second)
	if !tracker.exit(9) {
		t.Fatal("root exit was ignored")
	}
	view := tracker.snapshot()
	if view.Running || view.Attention || tracker.activeCount() != 0 {
		t.Fatalf("exit snapshot = %+v, active %d", view, tracker.activeCount())
	}
	if root := recordForRun(t, view, agentRootID, 1); root.State != agentStateFailed {
		t.Fatalf("failed root = %+v", root)
	}
	if child := delegatedRecord(t, view, "agent-1"); child.State != agentStateFinished {
		t.Fatalf("child outcome was inferred as %+v", child)
	}
	wantExpiry := clock.now().Add(retention)
	if got := tracker.earliestExpiry(); !got.Equal(wantExpiry) {
		t.Fatalf("earliest expiry = %s, want %s", got, wantExpiry)
	}
	clock.at = wantExpiry.Add(-time.Millisecond)
	if got := len(tracker.snapshot().Records); got != 2 {
		t.Fatalf("records at 9.999s = %d, want 2", got)
	}
	clock.at = wantExpiry
	if got := len(tracker.snapshot().Records); got != 0 {
		t.Fatalf("records at 10s = %d, want 0", got)
	}
}

func TestNewSupervisorRunCoexistsWithRetainedPriorGeneration(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now, time.Minute)
	tracker.begin(agentProviderClaude, 88)
	tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 88, Kind: workbench.LifecycleStart, ID: "reused-id",
	})
	tracker.exit(0)
	clock.at = clock.at.Add(time.Second)
	if !tracker.begin(agentProviderClaude, 7) {
		t.Fatal("a new supervisor generation was mistaken for an old numeric generation")
	}
	if !tracker.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 7, Kind: workbench.LifecycleStart, ID: "reused-id",
	}) {
		t.Fatal("provider ID could not be reused in the new run")
	}
	view := tracker.snapshot()
	if len(view.Records) != 4 {
		t.Fatalf("coexisting runs have %d records, want two roots and two children: %+v", len(view.Records), view.Records)
	}
	if recordForRun(t, view, agentRootID, 7).State != agentStateIdle ||
		recordForRun(t, view, agentRootID, 88).State != agentStateFinished {
		t.Fatalf("coexisting roots = %+v", view.Records)
	}
}

func TestPreGenerationExitRetainsAnExplicitSetupRunAlongsideTheLaterRun(t *testing.T) {
	clock := &activityClock{at: time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)}
	tracker := newAgentActivity(clock.now, time.Minute)
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

func TestTrackerMapsRolesCapabilitiesAndRootOutputWithoutParsingIt(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now, time.Minute)
	tracker.begin(agentProviderClaude, 1)
	tracker.output()
	root := recordForRun(t, tracker.snapshot(), agentRootID, 1)
	if root.State != agentStateWorking || root.Role != agentRoleOrchestrator {
		t.Fatalf("root after output = %+v", root)
	}
	clock.at = clock.at.Add(activityQuiet)
	if root := recordForRun(t, tracker.snapshot(), agentRootID, 1); root.State != agentStateIdle {
		t.Fatalf("root at quiet boundary = %+v", root)
	}
	if caps := tracker.snapshot().Capabilities; !caps.Attention || !caps.Children || caps.Parents || caps.Outcomes {
		t.Fatalf("Claude capabilities = %+v", caps)
	}
	tracker.begin(agentProviderCodex, 2)
	if caps := tracker.snapshot().Capabilities; caps.Attention || !caps.Children || caps.Parents || caps.Outcomes {
		t.Fatalf("Codex capabilities = %+v", caps)
	}
	for agentType, role := range map[string]string{
		"qrouton-implementation-lead": agentRoleLead,
		"explorer":                  agentRoleSpecialist,
		"":                          agentRoleUnavailable,
	} {
		if got := delegatedRole(agentType); got != role {
			t.Errorf("delegatedRole(%q) = %q, want %q", agentType, got, role)
		}
	}
}

func TestSessionSwitchKeepsRecentActivityAndCleanupDropsIt(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now, time.Minute)
	reg.boot.root = func(slug string) string { return filepath.Join(root, slug) }
	reg.boot.cleanup = func(string) error { return nil }
	reg.boot.teardown = func(*sessionState) {}
	alpha := reg.add(filepath.Join(root, "alpha"), []string{"/bin/cat"}, os.Environ())
	beta := reg.add(filepath.Join(root, "beta"), []string{"/bin/cat"}, os.Environ())
	alpha.agents.begin(agentProviderClaude, 1)
	reg.reveal(alpha)
	reg.reveal(beta)
	if got := reg.agentActivity("alpha"); got != alpha.agents || !got.snapshot().Running {
		t.Fatal("switching sessions reset alpha activity")
	}
	reg.retire(alpha)
	if got := reg.agentActivity("alpha"); got != alpha.agents {
		t.Fatal("retirement discarded retained activity")
	}
	if err := reg.Cleanup("alpha"); err != nil {
		t.Fatal(err)
	}
	if got := reg.agentActivity("alpha"); got != nil {
		t.Fatal("cleanup retained session activity")
	}
}

func TestLifecycleSocketMutatesOnlyItsOwningSession(t *testing.T) {
	windows, _ := testWindows(t)
	clock := &activityClock{at: time.Now()}
	trackers := []*agentActivity{
		newAgentActivity(clock.now, time.Minute),
		newAgentActivity(clock.now, time.Minute),
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
	if trackers[0].activeCount() != 2 || trackers[1].activeCount() != 0 {
		t.Fatalf("active counts = %d, %d", trackers[0].activeCount(), trackers[1].activeCount())
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
