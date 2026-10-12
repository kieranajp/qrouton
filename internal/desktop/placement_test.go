package desktop

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/placement"
	"github.com/kieranajp/qrouton/internal/session"
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

func TestAPlacementCheckWaitsForFirstRun(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	p, reg, stubs := newTestPlacement(t, cfg, misplacedReport())
	p.checkOnce()
	if reg.placing() || cfg.ThoughtsPlacementChecked || stubs.scans > 1 {
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

func TestPlacementOperationsNeverOverlap(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true}
	p, _, _ := newTestPlacement(t, cfg, placement.Report{})
	var active, peak atomic.Int32
	enter := func() {
		n := active.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
	}
	p.scan = func(*config.Config) (placement.Report, error) { enter(); return placement.Report{}, nil }
	p.move = func(_ *config.Config, slugs []string) []placement.Outcome { enter(); return nil }
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(4)
		go func() { defer wg.Done(); p.checkOnce() }()
		go func() { defer wg.Done(); _ = p.Check() }()
		go func() { defer wg.Done(); p.Move([]string{"a"}) }()
		go func() { defer wg.Done(); _ = p.Close() }()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatalf("%d operations ran at once", peak.Load())
	}
}

func TestRecoveryScansEveryLaunchEvenAfterTheFlagIsSet(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir(), Welcomed: true, ThoughtsPlacementChecked: true}
	p, reg, stubs := newTestPlacement(t, cfg, misplacedReport())
	p.checkOnce()
	if stubs.scans != 1 {
		t.Fatalf("scans = %d, want the recovery scan to run", stubs.scans)
	}
	if reg.placing() {
		t.Fatal("a misplaced session raised the popup a second time")
	}
}

func TestAPendingRecoveryRaisesThePopupOnAnyLaunch(t *testing.T) {
	pending := placement.Report{Items: []placement.Item{{Slug: "crashed", Name: "Crashed", State: placement.StatePending, Detail: "denied"}}}
	for name, cfg := range map[string]*config.Config{
		"flag set":      {Root: t.TempDir(), Welcomed: true, ThoughtsPlacementChecked: true},
		"automatic run": {Root: t.TempDir(), Welcomed: true},
	} {
		t.Run(name, func(t *testing.T) {
			p, reg, _ := newTestPlacement(t, cfg, pending)
			p.checkOnce()
			if !reg.placing() {
				t.Fatal("popup not raised")
			}
			if view := p.Report(); len(view.Pending) != 1 || view.Pending[0].Detail != "denied" {
				t.Fatalf("view = %+v", view)
			}
		})
	}
}

func TestACrashedManualMoveIsRepairedOnTheNextLaunchWithTheFlagSet(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, work := filepath.Join(base, "sessions"), filepath.Join(base, "work")
	def := filepath.Join(root, "thoughts")
	for _, d := range []string{def, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{Root: root, Welcomed: true, ThoughtsPlacementChecked: true,
		Thoughts: config.Thoughts{Roots: []config.ThoughtsRoot{{ID: "work", Path: work, Orgs: []string{"acme"}}}}}
	created := time.Now()
	dir := filepath.Join(root, "kraken")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := session.Manifest{SchemaVersion: 2, Name: "kraken", Slug: "kraken", CreatedAt: created,
		Repos: []session.ManifestRepo{{Org: "acme", Name: "svc", WorktreePath: "src/svc"}}}
	if err := session.WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "thoughts", "kraken"), filepath.Join(dir, "thoughts")); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(work, "kraken")
	if err := os.MkdirAll(moved, 0o755); err != nil {
		t.Fatal(err)
	}
	mark := "kraken " + created.UTC().Format(time.RFC3339Nano)
	if err := os.WriteFile(filepath.Join(moved, ".thoughtsmove"), []byte(mark), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	reg := newSessions()
	newPlacement(cfg, reg).checkOnce()

	if target, err := os.Readlink(filepath.Join(dir, "thoughts")); err != nil || target != moved {
		t.Fatalf("link = %q, %v, want %q", target, err, moved)
	}
	if _, err := os.Stat(filepath.Join(moved, ".thoughtsmove")); err == nil {
		t.Fatal("marker left behind")
	}
	if reg.placing() {
		t.Fatal("a repaired move raised the popup")
	}
}
