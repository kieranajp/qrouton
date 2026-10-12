package desktop

import (
	"reflect"
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/placement"
)

// PlacementRow is one misplaced session as the overlay draws it. Leaving marks
// a move out of a shared folder, which cannot recall teammates' copies.
type PlacementRow struct {
	placement.Item
	Leaving bool `json:"leaving"`
}

// PlacementView is what the overlay shows. LeavingNote and RelaunchNote are
// empty when they do not apply.
type PlacementView struct {
	Misplaced    []PlacementRow   `json:"misplaced"`
	Skipped      []placement.Item `json:"skipped"`
	LeavingNote  string           `json:"leavingNote"`
	RelaunchNote string           `json:"relaunchNote"`
}

// Placement checks where sessions keep their thoughts and moves the ones kept
// in the wrong root.
type Placement struct {
	cfg        *config.Config
	reg        *Sessions
	pinnedRoot string

	mu     sync.Mutex
	report placement.Report
	manual bool
	scan   func(*config.Config) (placement.Report, error)
	move   func(*config.Config, []string) []placement.Outcome
}

func newPlacement(cfg *config.Config, reg *Sessions) *Placement {
	return &Placement{cfg: cfg, reg: reg, pinnedRoot: pinnedFileRoot(), scan: placement.Scan, move: placement.Move}
}

// checkOnce is the automatic run: once per install, after first run, and only
// loud when a session is misplaced.
func (p *Placement) checkOnce() {
	snapshot := p.cfg.Snapshot()
	if snapshot.ThoughtsPlacementChecked || !snapshot.Welcomed {
		return
	}
	report, err := p.scan(p.cfg)
	if err != nil {
		return
	}
	if len(report.Misplaced()) == 0 {
		if !report.Pending() {
			_ = p.markChecked()
		}
		return
	}
	p.show(report, false)
}

// Check is the user's own run from Settings. It offers every session that is
// misplaced now and never touches the record of the automatic check.
func (p *Placement) Check() error {
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

// Report is the overlay's content.
func (p *Placement) Report() PlacementView {
	p.mu.Lock()
	report := p.report
	p.mu.Unlock()
	view := PlacementView{Misplaced: []PlacementRow{}, Skipped: report.Skipped()}
	if view.Skipped == nil {
		view.Skipped = []placement.Item{}
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

// Move moves each named session on its own and reports an outcome for each.
func (p *Placement) Move(slugs []string) []placement.Outcome {
	return p.move(p.cfg, slugs)
}

// Close dismisses the overlay. After the automatic check it records that check
// as done, unless an interrupted move is still waiting to be finished.
func (p *Placement) Close() error {
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

// markChecked writes the flag through the file's own values, so a saved but
// not yet applied Folders change is not overwritten by the live config.
func (p *Placement) markChecked() error {
	return p.cfg.Transact(func(current *config.Config) error {
		next, live, _, err := stageConfig(current, p.pinnedRoot, func(next, live *config.Config) {
			next.ThoughtsPlacementChecked, live.ThoughtsPlacementChecked = true, true
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
	if restartRequired(live, p.pinnedRoot, file) {
		return true
	}
	file.Root = live.Root
	file.Thoughts.Default = config.ExpandHome(file.Thoughts.Default)
	for i := range file.Thoughts.Roots {
		file.Thoughts.Roots[i].Path = config.ExpandHome(file.Thoughts.Roots[i].Path)
	}
	return !reflect.DeepEqual(file.ThoughtsRoots(), live.ThoughtsRoots())
}
