package desktop

import (
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/placement"
)

type PlacementRow struct {
	placement.Item
	Leaving bool `json:"leaving"`
}

// PlacementView is what the overlay shows. LeavingNote and RelaunchNote are
// empty when they do not apply.
type PlacementView struct {
	Misplaced    []PlacementRow   `json:"misplaced"`
	Skipped      []placement.Item `json:"skipped"`
	Pending      []placement.Item `json:"pending"`
	LeavingNote  string           `json:"leavingNote"`
	RelaunchNote string           `json:"relaunchNote"`
}

// Placement checks where sessions keep their thoughts and moves the ones kept
// in the wrong root.
type Placement struct {
	cfg        *config.Config
	reg        *Sessions
	pinnedRoot string

	// op keeps the automatic check, the Settings check, Move and Close from overlapping.
	op     sync.Mutex
	mu     sync.Mutex
	report placement.Report
	manual bool
	scan   func(*config.Config) (placement.Report, error)
	move   func(*config.Config, []string) []placement.Outcome
}

func newPlacement(cfg *config.Config, reg *Sessions) *Placement {
	return &Placement{cfg: cfg, reg: reg, pinnedRoot: pinnedFileRoot(), scan: placement.Scan, move: placement.Move}
}

// checkOnce runs at every launch to recover interrupted moves. The popup is
// automatic after first run whenever the thoughts folders differ from the ones
// last checked, if any session is misplaced, skipped or pending; a recovery
// that fails raises it on any launch.
func (p *Placement) checkOnce() {
	p.op.Lock()
	defer p.op.Unlock()
	snapshot := p.cfg.Snapshot()
	report, err := p.scan(p.cfg)
	if err != nil {
		return
	}
	if snapshot.ThoughtsPlacement == "" && snapshot.ThoughtsPlacementChecked {
		_ = p.markChecked()
		snapshot.ThoughtsPlacement = snapshot.ThoughtsFingerprint()
	}
	if !snapshot.Welcomed || snapshot.ThoughtsPlacement == snapshot.ThoughtsFingerprint() {
		if report.Pending() {
			p.show(report, true)
		}
		return
	}
	if len(report.Misplaced()) == 0 && len(report.Skipped()) == 0 && !report.Pending() {
		_ = p.markChecked()
		return
	}
	p.show(report, false)
}

// Check is the user's own run from Settings. It offers every session that is
// misplaced now and never touches the record of the automatic check.
func (p *Placement) Check() error {
	p.op.Lock()
	defer p.op.Unlock()
	report, err := p.scan(p.cfg)
	if err != nil {
		return err
	}
	p.show(report, true)
	return nil
}

func (p *Placement) show(report placement.Report, manual bool) {
	p.mu.Lock()
	p.report, p.manual = report, manual
	p.mu.Unlock()
	p.reg.setPlacing(true)
}

func (p *Placement) Report() PlacementView {
	p.mu.Lock()
	report := p.report
	p.mu.Unlock()
	view := PlacementView{Misplaced: []PlacementRow{}, Skipped: report.Skipped(), Pending: report.PendingItems()}
	if view.Skipped == nil {
		view.Skipped = []placement.Item{}
	}
	if view.Pending == nil {
		view.Pending = []placement.Item{}
	}
	for _, item := range report.Misplaced() {
		leaving := item.From.ID != config.DefaultThoughtsID
		view.Misplaced = append(view.Misplaced, PlacementRow{Item: item, Leaving: leaving})
		if leaving {
			view.LeavingNote = placementLeavingNote
		}
	}
	if p.relaunchPending() {
		view.RelaunchNote = placementRelaunchNote
	}
	return view
}

func (p *Placement) Move(slugs []string) []placement.Outcome {
	p.op.Lock()
	defer p.op.Unlock()
	return p.move(p.cfg, slugs)
}

// Close dismisses the overlay. After the automatic check it records that check
// as done, unless an interrupted move is still waiting to be finished.
func (p *Placement) Close() error {
	p.op.Lock()
	defer p.op.Unlock()
	p.reg.setPlacing(false)
	p.mu.Lock()
	manual := p.manual
	p.mu.Unlock()
	if manual {
		return nil
	}
	report, err := p.scan(p.cfg)
	if err != nil || report.Pending() {
		return nil
	}
	return p.markChecked()
}

// markChecked records the fingerprint of the folders the check ran against,
// through the file's own values, so a saved but not yet applied Folders change
// is not overwritten by the live config.
func (p *Placement) markChecked() error {
	return p.cfg.Transact(func(current *config.Config) error {
		fingerprint := current.ThoughtsFingerprint()
		next, live, _, err := stageConfig(current, p.pinnedRoot, false, func(next, live *config.Config) {
			next.ThoughtsPlacement, live.ThoughtsPlacement = fingerprint, fingerprint
			next.ThoughtsPlacementChecked, live.ThoughtsPlacementChecked = false, false
		})
		if err != nil {
			return err
		}
		if err := config.Save(next); err != nil {
			return err
		}
		p.cfg.Replace(live)
		return nil
	})
}

// relaunchPending reports a saved root or shared-folder change the running
// process has not picked up.
func (p *Placement) relaunchPending() bool {
	live := p.cfg.Snapshot()
	file, err := config.ReadFile()
	if err != nil {
		return false
	}
	return restartRequired(live, p.pinnedRoot, file) || thoughtsRoutingChanged(live, file)
}
