package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/vault"
)

// shutdownCore is a core whose writes land in this test's XDG directories.
func shutdownCore(t *testing.T, set *vault.Set) *Core {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	base, _ := testOptions(t)
	base.Options.Vault = set
	base.Options.Config = &config.Config{Orgs: []string{"acme"}}
	base.SessionRoot = ""
	c := testWorkbench(t, newFakeClient(), base)
	startWorkbench(t, c, base)
	return c
}

// slowRefresh is a repository fetch that is still finishing, which is when the
// live one writes the repository cache, for a moment after its context ends.
func slowRefresh(started chan<- struct{}, finished *atomic.Bool, linger time.Duration) func(context.Context, string, []string, []github.Repo) <-chan github.RefreshMsg {
	return func(ctx context.Context, _ string, _ []string, _ []github.Repo) <-chan github.RefreshMsg {
		out := make(chan github.RefreshMsg, 2)
		out <- github.RefreshMsg{Owner: "acme", State: github.RefreshStarted}
		go func() {
			defer close(out)
			started <- struct{}{}
			<-ctx.Done()
			time.Sleep(20 * time.Millisecond)
			out <- github.RefreshMsg{State: github.RefreshComplete}
			time.Sleep(linger)
			finished.Store(true)
		}()
		return out
	}
}

// Catches Shutdown returning while a refresh can still write the repository
// cache, which after a test's env restore is the user's own.
func TestShutdownWaitsForARepositoryRefresh(t *testing.T) {
	c := shutdownCore(t, nil)
	started := make(chan struct{}, 1)
	var finished atomic.Bool
	c.Repositories.gh = gh{token: func(context.Context) (string, error) { return "t", nil }, all: slowRefresh(started, &finished, 20*time.Millisecond)}

	c.Repositories.Refresh()
	<-started
	c.Shutdown()
	if !finished.Load() {
		t.Fatal("the repository refresh was still running after Shutdown")
	}
}

// Catches a superseded refresh that stops reading its fetch early, leaving the
// fetch to write the cache after Shutdown.
func TestShutdownWaitsForASupersededRepositoryFetch(t *testing.T) {
	c := shutdownCore(t, nil)
	started := make(chan struct{}, 2)
	var first, second atomic.Bool
	// The superseded fetch lingers past the live one, so only waiting on it covers it.
	fetches := []struct {
		finished *atomic.Bool
		linger   time.Duration
	}{{&first, 300 * time.Millisecond}, {&second, 0}}
	var calls atomic.Int32
	c.Repositories.gh = gh{
		token: func(context.Context) (string, error) { return "t", nil },
		all: func(ctx context.Context, token string, orgs []string, cached []github.Repo) <-chan github.RefreshMsg {
			f := fetches[calls.Add(1)-1]
			return slowRefresh(started, f.finished, f.linger)(ctx, token, orgs, cached)
		},
	}

	c.Repositories.Refresh()
	<-started
	c.Repositories.Refresh()
	<-started
	c.Shutdown()
	if !first.Load() || !second.Load() {
		t.Fatalf("a repository fetch was still running after Shutdown: first %v, second %v", first.Load(), second.Load())
	}
}

// blockingEmbedder holds the warm in its model probe until the warm is
// cancelled, then takes a moment to let go, as a cache save would.
type blockingEmbedder struct {
	probing  chan struct{}
	finished atomic.Bool
}

func (e *blockingEmbedder) Model(ctx context.Context) (vault.Model, error) {
	e.probing <- struct{}{}
	<-ctx.Done()
	time.Sleep(20 * time.Millisecond)
	e.finished.Store(true)
	return vault.Model{}, ctx.Err()
}

func (e *blockingEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errors.New("not embedding")
}

// Catches Shutdown returning while the thoughts index is still warming, which
// saves the index under XDG_CACHE_HOME when it stops.
func TestShutdownWaitsForTheVaultWarm(t *testing.T) {
	embedder := &blockingEmbedder{probing: make(chan struct{}, 1)}
	set := vault.NewSet(embedder, t.TempDir())
	if err := set.Configure([]vault.Profile{{ID: "default", Root: t.TempDir()}}); err != nil {
		t.Fatal(err)
	}
	c := shutdownCore(t, set)

	<-embedder.probing
	c.Shutdown()
	if !embedder.finished.Load() {
		t.Fatal("the vault warm was still running after Shutdown")
	}
}

// Catches Shutdown waiting out a credential lookup that a stopping refresh
// cannot cancel.
func TestShutdownCancelsACredentialLookup(t *testing.T) {
	c := shutdownCore(t, nil)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	// Frees a lookup that was never cancelled, so a failure reports rather than hangs.
	t.Cleanup(func() { close(release) })
	c.Repositories.gh = gh{token: func(ctx context.Context) (string, error) {
		close(started)
		select {
		case <-ctx.Done():
			close(cancelled)
			return "", ctx.Err()
		case <-release:
			return "", errors.New("released")
		}
	}}

	c.Repositories.Refresh()
	<-started
	stopped := make(chan struct{})
	go func() {
		c.Repositories.stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("stop did not return: the credential lookup was not cancelled")
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("stop returned without the credential lookup seeing its context end")
	}
}
