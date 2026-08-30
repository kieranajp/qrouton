package update

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// Installer is the framework's updater, narrowed to the three calls the policy
// drives. Narrowing it is what lets the policy below be exercised without a
// release, a network or a webview.
type Installer interface {
	Check(context.Context) (*updater.Release, error)
	DownloadAndInstall(context.Context) error
	Restart(context.Context) error
}

// Keeper keeps an install on the latest tagged release, and there is nothing
// in it for a user to dismiss.
//
// A release found is downloaded and verified straight away, then applied at
// the first moment a relaunch costs nothing: a workbench holding no
// conversation. It never takes one away mid-turn — qrouton owns the surface,
// not the exchange on it, and an update that kills an agent's terminal is a
// worse outcome than an install one version behind.
//
// The floor is what stops that politeness from becoming a way to stay behind.
// A release may advertise the oldest version it will talk to; an install below
// it is Held, and the workbench refuses to assemble a session until the swap
// happens. Everything else fails open — an unreachable feed leaves the install
// working on what it has.
type Keeper struct {
	// Installer performs the check, the download and the swap. A nil one makes
	// the Keeper inert, which is how an install that cannot replace itself —
	// an unbundled build, a platform with no release — carries no policy.
	Installer Installer
	// Feed answers the floor the latest release advertises.
	Feed Feed
	// Current is the version this build reports.
	Current string
	// Idle reports whether a relaunch would cost the user nothing. A nil one
	// never applies a staged release, which is not a state the workbench wires.
	Idle func() bool
	// Changed is called whenever Held flips, so the window can redraw around a
	// gate that has just gone up.
	Changed func()

	// Every bounds how often a long-lived window re-checks; Settle bounds how
	// often a staged release asks whether the workbench has gone quiet.
	Every  time.Duration
	Settle time.Duration

	held   atomic.Bool
	staged atomic.Bool
}

// Held reports whether this install is below the floor the latest release
// advertises, and so may not assemble a session until it updates.
func (k *Keeper) Held() bool { return k.held.Load() }

// Run drives the policy until ctx is done. It returns immediately for an
// install with no Installer, so a caller need not decide whether to start one.
func (k *Keeper) Run(ctx context.Context) {
	if k.Installer == nil || k.Idle == nil {
		return
	}
	every, settle := k.Every, k.Settle
	if every <= 0 {
		every = checkInterval
	}
	if settle <= 0 {
		settle = settleInterval
	}
	for {
		wait := every
		if k.round(ctx) {
			// A staged release waits on a quiet workbench, not on the next check.
			wait = settle
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}

// round is one pass of the policy. It reports whether a release is staged and
// still waiting for a quiet moment to be applied.
func (k *Keeper) round(ctx context.Context) bool {
	if k.staged.Load() {
		return !k.apply(ctx)
	}
	release, err := k.check(ctx)
	if err != nil || release == nil {
		return false
	}
	k.hold(k.floor(ctx))
	if err := k.Installer.DownloadAndInstall(ctx); err != nil {
		return false
	}
	k.staged.Store(true)
	return !k.apply(ctx)
}

// apply swaps the staged release in and relaunches, and reports whether it
// did. A workbench holding a conversation is left alone; the caller asks again.
func (k *Keeper) apply(ctx context.Context) bool {
	if !k.Idle() {
		return false
	}
	// Restart hands off to a helper that waits for this process to exit, so a
	// failure here leaves the install running exactly as it was.
	return k.Installer.Restart(ctx) == nil
}

func (k *Keeper) check(ctx context.Context) (*updater.Release, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	return k.Installer.Check(ctx)
}

// floor reads the marker the latest release publishes. A feed that cannot
// answer imposes no floor: an install is held by a version it has been told
// about, never by a network it could not reach.
func (k *Keeper) floor(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	floor, err := k.Feed.Floor(ctx)
	if err != nil {
		return ""
	}
	return floor
}

func (k *Keeper) hold(floor string) {
	held := Held(k.Current, floor)
	if k.held.Swap(held) == held || k.Changed == nil {
		return
	}
	k.Changed()
}
