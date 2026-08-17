package desktop

import (
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/config"
)

// FirstRunInput is the two answers the flow collects: the owners to search and
// where sessions live.
type FirstRunInput struct {
	Orgs []string `json:"orgs"`
	Root string   `json:"root"`
}

// FirstRunResult reports whether the workbench is being replaced. Nothing is
// required of the user either way, which is why this is not Settings' SaveResult.
type FirstRunResult struct {
	Relaunching bool `json:"relaunching"`
}

// FirstRun is the gate's service: one save, and the two things the screens
// promise but config cannot answer.
type FirstRun struct {
	cfg      *config.Config
	reg      *Sessions
	relaunch func() error
	quit     func()
}

func newFirstRun(cfg *config.Config, reg *Sessions, relaunch func() error, quit func()) *FirstRun {
	return &FirstRun{cfg: cfg, reg: reg, relaunch: relaunch, quit: quit}
}

// Save writes orgs, root and the welcomed marker together, then either drops the
// gate or replaces the workbench. A changed root cannot take effect in this
// process — the rail's scanner and boot path closed over the boot value — so the
// successor is what reaches the new root, and the marker only goes on the live
// pointer when this process is the one carrying on.
func (f *FirstRun) Save(in FirstRunInput) (FirstRunResult, error) {
	expanded, err := validateRoot(in.Root)
	if err != nil {
		return FirstRunResult{}, err
	}
	changed := expanded != filepath.Clean(f.cfg.Root)

	next := *f.cfg
	next.Orgs, next.Root, next.Welcomed = dedupOrgs(in.Orgs), in.Root, true
	if err := config.Save(&next); err != nil {
		return FirstRunResult{}, err
	}

	if !changed {
		f.cfg.Orgs, f.cfg.Welcomed = next.Orgs, true
		// Without the touch the overlay stays up until the next chrome tick.
		f.reg.touch()
		return FirstRunResult{}, nil
	}

	if f.relaunch == nil {
		return FirstRunResult{}, ErrNoRelaunch
	}
	if err := f.relaunch(); err != nil {
		return FirstRunResult{}, err
	}
	f.quit()
	return FirstRunResult{Relaunching: true}, nil
}
