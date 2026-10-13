package desktop

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type FirstRunInput struct {
	Orgs     []string `json:"orgs"`
	Root     string   `json:"root"`
	Thoughts string   `json:"thoughts"`
}

// FirstRunResult reports whether the workbench is being replaced. Nothing is
// required of the user either way, which is why this is not Settings' SaveResult.
type FirstRunResult struct {
	Relaunching bool `json:"relaunching"`
}

type FirstRun struct {
	cfg         *config.Config
	reg         *Sessions
	relaunch    func() error
	quit        func()
	pinnedRoot  string
	choose      func() (string, error)
	reconfigure func(*config.Config) error
	onWelcomed  func()
}

func newFirstRun(cfg *config.Config, reg *Sessions, relaunch func() error, quit func(),
	choose func() (string, error), reconfigure func(*config.Config) error) *FirstRun {
	return &FirstRun{cfg: cfg, reg: reg, relaunch: relaunch, quit: quit, pinnedRoot: pinnedFileRoot(),
		choose: choose, reconfigure: reconfigure}
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
	thoughts := strings.TrimSpace(in.Thoughts)
	if filepath.Clean(config.ExpandHome(thoughts)) == filepath.Join(expanded, sessionpaths.ThoughtsDirName) {
		thoughts = ""
	}
	changed := false
	err = f.cfg.Transact(func(current *config.Config) error {
		next, live, restart, err := stageConfig(current, f.pinnedRoot, false, func(next, live *config.Config) {
			next.Orgs, next.Root, next.Welcomed = orgs, root, true
			next.Thoughts.Default = thoughts
			if !config.OrgsOverridden() {
				live.Orgs = orgs
			}
			live.Welcomed = true
			live.Thoughts.Default = config.ExpandHome(thoughts)
		})
		if err != nil {
			return err
		}
		changed = restart
		if err := config.CheckThoughts(next); err != nil {
			return fmt.Errorf(settingsWrappedFormat, settingsFieldThoughts, err)
		}
		if !changed {
			if err := checkLiveThoughts(live); err != nil {
				return err
			}
			return commitLive(f.cfg, current, next, live, f.reconfigure)
		}
		if err := config.Save(next); err != nil {
			return err
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
		if f.onWelcomed != nil {
			f.onWelcomed()
		}
		return FirstRunResult{}, nil
	}

	f.quit()
	return FirstRunResult{Relaunching: true}, nil
}
