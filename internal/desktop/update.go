package desktop

import (
	"github.com/kieranajp/qrouton/internal/update"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// gate reports whether the window must draw the update gate rather than let
// work start. A function rather than a bool because the answer arrives seconds
// after launch, once the release feed has been read.
type gate func() bool

// newKeeper builds the update policy for this install. An install that cannot
// replace itself — a `go build` on a developer's path, or a platform releases
// are not cut for — gets a Keeper with no Installer, which runs nothing and
// holds nobody. The workbench opens either way: being unable to update is not
// a reason to refuse to start.
func newKeeper(app *application.App, reg *Sessions, drafting func() bool, current string) *update.Keeper {
	keeper := &update.Keeper{
		Current: current,
		Idle:    idleFor(reg, drafting),
		// Waking the chrome poller puts a gate on screen at once rather than on
		// the next tick.
		Changed: reg.touch,
	}
	if app == nil || !update.Supported(current) {
		return keeper
	}
	cfg, err := update.Config(current)
	// A malformed config is this build's own bug rather than the user's
	// problem, so it costs the policy and nothing else.
	if err != nil || app.Updater.Init(cfg) != nil {
		return keeper
	}
	keeper.Installer = app.Updater
	return keeper
}

// idleFor is the moment a relaunch costs the user nothing: no conversation
// running, and no half-finished session draft on screen. Either one says wait.
func idleFor(reg *Sessions, drafting func() bool) func() bool {
	return func() bool {
		if drafting != nil && drafting() {
			return false
		}
		return len(reg.all()) == 0
	}
}
