package desktop

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
)

type FirstRunInput struct {
	Orgs []string `json:"orgs"`
	Root string   `json:"root"`
}

// FirstRunResult reports whether the workbench is being replaced. Nothing is
// required of the user either way, which is why this is not Settings' SaveResult.
type FirstRunResult struct {
	Relaunching bool `json:"relaunching"`
}

type FirstRun struct {
	cfg      *config.Config
	reg      *Sessions
	relaunch func() error
	quit     func()
	choose   func() (string, error)
}

func newFirstRun(cfg *config.Config, reg *Sessions, relaunch func() error, quit func(),
	choose func() (string, error)) *FirstRun {
	return &FirstRun{cfg: cfg, reg: reg, relaunch: relaunch, quit: quit, choose: choose}
}

// Login is the GitHub account the owners screen names. No account is an answer
// rather than an error: the screen has a sentence for it, and a wrong or absent
// account is exactly how the owners silently fail to resolve.
func (f *FirstRun) Login() string {
	token, err := github.Token()
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()
	login, err := github.AuthenticatedLogin(ctx, &http.Client{Timeout: loginTimeout}, token)
	if err != nil {
		return ""
	}
	return login
}

// ChooseRoot asks for a directory. Cancelling answers the empty string, so the
// field keeps what it had.
func (f *FirstRun) ChooseRoot() (string, error) {
	if f.choose == nil {
		return "", ErrNoDirectoryPicker
	}
	return f.choose()
}

// Save writes both answers and the welcomed marker together, then either drops
// the gate or replaces the workbench. The marker reaches the live pointer only
// when this process is carrying on, so a failed relaunch leaves the gate up
// rather than a workbench on the old root.
func (f *FirstRun) Save(in FirstRunInput) (FirstRunResult, error) {
	orgs, root, expanded, err := validateOwnersAndRoot(in.Orgs, in.Root)
	if err != nil {
		return FirstRunResult{}, err
	}
	changed := false
	err = f.cfg.Transact(func(current *config.Config) error {
		changed = expanded != filepath.Clean(current.Root)
		next := current.Snapshot()
		next.Orgs, next.Root, next.Welcomed = orgs, root, true
		if err := config.Save(next); err != nil {
			return err
		}
		if !changed {
			live := next.Snapshot()
			live.Root = current.Root
			f.cfg.Replace(live)
			return nil
		}
		if f.relaunch == nil || f.quit == nil {
			return ErrNoRelaunch
		}
		return f.relaunch()
	})
	if err != nil {
		return FirstRunResult{}, err
	}

	if !changed {
		// Without the touch the overlay stays up until the next chrome tick.
		f.reg.touch()
		return FirstRunResult{}, nil
	}

	f.quit()
	return FirstRunResult{Relaunching: true}, nil
}
