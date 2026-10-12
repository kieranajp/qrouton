package desktop

import (
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/placement"
	"github.com/kieranajp/qrouton/internal/status"
)

type placementStubs struct {
	report placement.Report
	scans  int
	moved  [][]string
}

func newTestPlacement(t *testing.T, cfg *config.Config, report placement.Report) (*Placement, *Sessions, *placementStubs) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	reg := newSessions()
	stubs := &placementStubs{report: report}
	p := newPlacement(cfg, reg)
	p.scan = func(*config.Config) (placement.Report, error) { stubs.scans++; return stubs.report, nil }
	p.move = func(_ *config.Config, slugs []string) []placement.Outcome {
		stubs.moved = append(stubs.moved, slugs)
		return []placement.Outcome{{Slug: slugs[0], Result: placement.ResultMoved}}
	}
	return p, reg, stubs
}

func misplacedReport() placement.Report {
	return placement.Report{Items: []placement.Item{{
		Slug: "kraken", Name: "Kraken", State: placement.StateMisplaced,
		From: placement.Root{ID: "work", Path: "/work"},
		To:   placement.Root{ID: config.DefaultThoughtsID, Path: "/default"},
	}}}
}

func TestAPlacementCheckAlreadyDoneIsNotRunAgain(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true, ThoughtsPlacementChecked: true}
	p, reg, stubs := newTestPlacement(t, cfg, misplacedReport())
	p.checkOnce()
	if stubs.scans != 0 || reg.placing() {
		t.Fatalf("scans %d, placing %v", stubs.scans, reg.placing())
	}
}

func TestAPlacementCheckWaitsForFirstRun(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	p, reg, stubs := newTestPlacement(t, cfg, misplacedReport())
	p.checkOnce()
	if stubs.scans != 0 || reg.placing() || cfg.ThoughtsPlacementChecked {
		t.Fatalf("scans %d, placing %v, checked %v", stubs.scans, reg.placing(), cfg.ThoughtsPlacementChecked)
	}
}

func TestNothingMisplacedSetsTheFlagWithoutAnOverlay(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true}
	p, reg, _ := newTestPlacement(t, cfg, placement.Report{})
	p.checkOnce()
	if reg.placing() {
		t.Fatal("overlay raised with nothing misplaced")
	}
	if !savedConfig(t).ThoughtsPlacementChecked || !cfg.Snapshot().ThoughtsPlacementChecked {
		t.Fatal("flag not recorded in the file and the live config")
	}
}

func TestOnlySkippedSessionsStillRaiseTheOverlay(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true}
	skipped := placement.Report{Items: []placement.Item{
		{Slug: "far", State: placement.StateSkipped, Reason: placement.ReasonCrossDevice},
		{Slug: "taken", State: placement.StateSkipped, Reason: placement.ReasonConflict},
	}}
	p, reg, _ := newTestPlacement(t, cfg, skipped)
	p.checkOnce()
	if !reg.placing() {
		t.Fatal("overlay not raised for sessions that are misplaced but cannot move")
	}
	if savedConfig(t).ThoughtsPlacementChecked {
		t.Fatal("flag set before the user answered")
	}
	if view := p.Report(); len(view.Misplaced) != 0 || len(view.Skipped) != 2 {
		t.Fatalf("view = %+v", view)
	}
}

func TestAMisplacedSessionRaisesTheOverlayAndLeavesTheFlagUnset(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true}
	p, reg, _ := newTestPlacement(t, cfg, misplacedReport())
	p.checkOnce()
	if !reg.placing() {
		t.Fatal("overlay not raised")
	}
	if savedConfig(t).ThoughtsPlacementChecked {
		t.Fatal("flag set before the user answered")
	}
	view := p.Report()
	if len(view.Misplaced) != 1 || !view.Misplaced[0].Leaving || view.LeavingNote == "" {
		t.Fatalf("view = %+v", view)
	}
}

func TestMoveForwardsTheTickedSlugs(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true}
	p, _, stubs := newTestPlacement(t, cfg, misplacedReport())
	out := p.Move([]string{"kraken"})
	if len(stubs.moved) != 1 || stubs.moved[0][0] != "kraken" || out[0].Result != placement.ResultMoved {
		t.Fatalf("moved %v, outcomes %v", stubs.moved, out)
	}
}

func TestClosingRecordsTheCheckUnlessAMoveIsStillPending(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true}
	pending := placement.Report{Items: []placement.Item{{Slug: "kraken", State: placement.StatePending}}}
	p, reg, stubs := newTestPlacement(t, cfg, pending)
	reg.setPlacing(true)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if reg.placing() || savedConfig(t).ThoughtsPlacementChecked {
		t.Fatalf("placing %v, flag %v", reg.placing(), savedConfig(t).ThoughtsPlacementChecked)
	}
	stubs.report = misplacedReport()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if !savedConfig(t).ThoughtsPlacementChecked {
		t.Fatal("closing with no pending move left the flag unset")
	}
}

func TestMarkingTheCheckKeepsAFolderChangeWaitingForARelaunch(t *testing.T) {
	root := t.TempDir()
	live := &config.Config{Root: root, Welcomed: true}
	p, _, _ := newTestPlacement(t, live, placement.Report{})
	saved := &config.Config{Root: root, Welcomed: true, Thoughts: config.Thoughts{Roots: []config.ThoughtsRoot{
		{ID: "work", Path: t.TempDir(), Orgs: []string{"acme"}},
	}}}
	if err := config.Save(saved); err != nil {
		t.Fatal(err)
	}
	if !p.relaunchPending() {
		t.Fatal("a saved shared folder was not reported as waiting")
	}
	if err := p.markChecked(); err != nil {
		t.Fatal(err)
	}
	if got := savedConfig(t); len(got.Thoughts.Roots) != 1 || !got.ThoughtsPlacementChecked {
		t.Fatalf("saved = %+v", got)
	}
	if len(live.Snapshot().Thoughts.Roots) != 0 {
		t.Fatal("the live routing changed before a relaunch")
	}
	if p.Report().RelaunchNote == "" {
		t.Fatal("the overlay does not mention the pending relaunch")
	}
}

func TestWelcomingHidesThePlacementOverlay(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	reg := newSessions()
	reg.setPlacing(true)
	var fields status.Fields
	pushChrome(reg, root, &config.Config{Root: root}, nil, nil, func(_ string, payload any) { fields = payload.(status.Fields) })
	if !fields.Welcoming || fields.Placing {
		t.Fatalf("welcoming %v, placing %v", fields.Welcoming, fields.Placing)
	}
	pushChrome(reg, root, &config.Config{Root: root, Welcomed: true}, nil, nil, func(_ string, payload any) { fields = payload.(status.Fields) })
	if fields.Welcoming || !fields.Placing {
		t.Fatalf("welcoming %v, placing %v", fields.Welcoming, fields.Placing)
	}
}

func TestAManualCheckOffersWhatTheAutomaticRunSkipped(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true, ThoughtsPlacementChecked: true}
	skipped := placement.Report{Items: []placement.Item{{Slug: "kraken", State: placement.StateSkipped, Reason: placement.ReasonAgentRunning}}}
	p, reg, stubs := newTestPlacement(t, cfg, skipped)
	p.checkOnce()
	if reg.placing() {
		t.Fatal("the automatic run ran again")
	}
	stubs.report = misplacedReport()
	if err := p.Check(); err != nil {
		t.Fatal(err)
	}
	if !reg.placing() || len(p.Report().Misplaced) != 1 {
		t.Fatalf("placing %v, view %+v", reg.placing(), p.Report())
	}
}

func TestClosingAManualCheckLeavesTheFlagAlone(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true}
	p, reg, _ := newTestPlacement(t, cfg, misplacedReport())
	if err := p.Check(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if reg.placing() || savedConfig(t).ThoughtsPlacementChecked || cfg.Snapshot().ThoughtsPlacementChecked {
		t.Fatalf("placing %v, flag %v", reg.placing(), savedConfig(t).ThoughtsPlacementChecked)
	}
}
