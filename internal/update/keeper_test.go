package update

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/updater"
)

// fakeInstaller records the calls the policy makes and answers with whatever
// the test has staged.
type fakeInstaller struct {
	mu        sync.Mutex
	release   *updater.Release
	checkErr  error
	installEr error
	restartEr error

	checks    int
	installs  int
	restarts  int
	restarted chan struct{}
}

func newFake(release *updater.Release) *fakeInstaller {
	return &fakeInstaller{release: release, restarted: make(chan struct{}, 1)}
}

func (f *fakeInstaller) Check(context.Context) (*updater.Release, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	return f.release, f.checkErr
}

func (f *fakeInstaller) DownloadAndInstall(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.installs++
	return f.installEr
}

func (f *fakeInstaller) Restart(context.Context) error {
	f.mu.Lock()
	f.restarts++
	err := f.restartEr
	f.mu.Unlock()
	if err == nil {
		select {
		case f.restarted <- struct{}{}:
		default:
		}
	}
	return err
}

func (f *fakeInstaller) counts() (checks, installs, restarts int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.checks, f.installs, f.restarts
}

// floorFeed serves a release advertising floor, and points the package at it.
func floorFeed(t *testing.T, floor string) Feed {
	t.Helper()
	return feed(t, floorAsset, floor)
}

// run starts the policy and makes its exit observable: the test server and the
// package's API base are torn down after it, and a goroutine still reading
// either one is a race rather than a flake.
func run(t *testing.T, k *Keeper) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		<-done
	})
	go func() {
		defer close(done)
		k.Run(ctx)
	}()
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// The whole point: a release found on an idle workbench is downloaded and
// swapped in without anyone being asked.
func TestAnIdleWorkbenchTakesTheReleaseWithoutAsking(t *testing.T) {
	fake := newFake(&updater.Release{Version: "0.5.0"})
	k := &Keeper{
		Installer: fake, Feed: floorFeed(t, ""), Current: "0.4.0",
		Idle: func() bool { return true }, Every: time.Millisecond, Settle: time.Millisecond,
	}
	run(t, k)

	select {
	case <-fake.restarted:
	case <-time.After(2 * time.Second):
		t.Fatal("an idle workbench never applied the release")
	}
	if _, installs, _ := fake.counts(); installs == 0 {
		t.Error("the release was applied without being downloaded")
	}
	if k.Held() {
		t.Error("an install above the floor was held")
	}
}

// A live conversation is never taken away mid-turn: the release stays staged
// and the swap waits, however many rounds that takes.
func TestABusyWorkbenchIsNeverRestartedUnderTheUser(t *testing.T) {
	fake := newFake(&updater.Release{Version: "0.5.0"})
	var idle flag
	k := &Keeper{
		Installer: fake, Feed: floorFeed(t, ""), Current: "0.4.0",
		Idle: idle.get, Every: time.Millisecond, Settle: time.Millisecond,
	}
	run(t, k)

	waitFor(t, "the release to be staged", func() bool {
		_, installs, _ := fake.counts()
		return installs > 0
	})
	time.Sleep(50 * time.Millisecond)
	if _, _, restarts := fake.counts(); restarts != 0 {
		t.Fatalf("a busy workbench was restarted %d times", restarts)
	}
	// It is downloaded once and only once, however long the wait runs.
	if _, installs, _ := fake.counts(); installs != 1 {
		t.Errorf("the release was downloaded %d times while waiting", installs)
	}

	idle.set(true)
	select {
	case <-fake.restarted:
	case <-time.After(2 * time.Second):
		t.Fatal("the staged release was never applied once the workbench went quiet")
	}
}

// Below the floor the latest release advertises, the install is held — which
// is what the workbench refuses to assemble a session against.
func TestAnInstallBelowTheFloorIsHeld(t *testing.T) {
	fake := newFake(&updater.Release{Version: "0.5.0"})
	changed := make(chan struct{}, 1)
	k := &Keeper{
		Installer: fake, Feed: floorFeed(t, "0.5.0"), Current: "0.4.0",
		Idle:    func() bool { return false },
		Changed: func() { changed <- struct{}{} },
		Every:   time.Millisecond, Settle: time.Millisecond,
	}
	run(t, k)

	select {
	case <-changed:
	case <-time.After(2 * time.Second):
		t.Fatal("falling below the floor was never reported")
	}
	if !k.Held() {
		t.Error("an install below the floor was not held")
	}
}

// A release that publishes no floor holds nobody: the marker is opt-in, and a
// release cut before it existed must not lock every install out.
func TestAReleaseWithoutAFloorHoldsNobody(t *testing.T) {
	fake := newFake(&updater.Release{Version: "0.5.0"})
	k := &Keeper{
		Installer: fake, Feed: feed(t, "", ""), Current: "0.1.0",
		Idle: func() bool { return false }, Every: time.Millisecond, Settle: time.Millisecond,
	}
	run(t, k)
	waitFor(t, "the first round", func() bool { checks, _, _ := fake.counts(); return checks > 0 })
	time.Sleep(20 * time.Millisecond)
	if k.Held() {
		t.Error("a release advertising no floor held an install anyway")
	}
}

// Fail open. An install that cannot reach GitHub keeps working on what it has
// rather than being gated by a network it could not reach.
func TestAnUnreachableFeedNeitherHoldsNorInstalls(t *testing.T) {
	fake := newFake(nil)
	fake.checkErr = errors.New("dial tcp: no route to host")
	k := &Keeper{
		Installer: fake, Feed: floorFeed(t, "9.9.9"), Current: "0.1.0",
		Idle: func() bool { return true }, Every: time.Millisecond, Settle: time.Millisecond,
	}
	run(t, k)
	waitFor(t, "the first round", func() bool { checks, _, _ := fake.counts(); return checks > 0 })
	time.Sleep(20 * time.Millisecond)
	if k.Held() {
		t.Error("an unreachable feed held the install")
	}
	if _, installs, restarts := fake.counts(); installs != 0 || restarts != 0 {
		t.Errorf("a failed check installed %d and restarted %d times", installs, restarts)
	}
}

// An install already on the latest release does nothing at all.
func TestAnUpToDateInstallDoesNothing(t *testing.T) {
	fake := newFake(nil)
	k := &Keeper{
		Installer: fake, Feed: floorFeed(t, "0.5.0"), Current: "0.5.0",
		Idle: func() bool { return true }, Every: time.Millisecond, Settle: time.Millisecond,
	}
	run(t, k)
	waitFor(t, "the first round", func() bool { checks, _, _ := fake.counts(); return checks > 0 })
	time.Sleep(20 * time.Millisecond)
	if _, installs, restarts := fake.counts(); installs != 0 || restarts != 0 {
		t.Errorf("an up-to-date install downloaded %d and restarted %d times", installs, restarts)
	}
	if k.Held() {
		t.Error("an up-to-date install was held")
	}
}

// A download that fails is retried on the next round rather than leaving the
// Keeper convinced it has something staged.
func TestAFailedDownloadIsRetried(t *testing.T) {
	fake := newFake(&updater.Release{Version: "0.5.0"})
	fake.installEr = errors.New("digest mismatch")
	k := &Keeper{
		Installer: fake, Feed: floorFeed(t, ""), Current: "0.4.0",
		Idle: func() bool { return true }, Every: time.Millisecond, Settle: time.Millisecond,
	}
	run(t, k)
	waitFor(t, "the download to be retried", func() bool {
		_, installs, _ := fake.counts()
		return installs > 1
	})
	if _, _, restarts := fake.counts(); restarts != 0 {
		t.Errorf("a release that never verified was applied %d times", restarts)
	}
}

// An install that cannot replace itself carries no policy at all, so nothing
// runs and nothing is held.
func TestAnInstallThatCannotReplaceItselfIsInert(t *testing.T) {
	k := &Keeper{Feed: floorFeed(t, "9.9.9"), Current: "0.1.0", Idle: func() bool { return true }}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { k.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("a Keeper with no installer did not return")
	}
	if k.Held() {
		t.Error("an install with no updater was held")
	}
}

// flag is a bool a test flips from one goroutine and the policy reads from
// another.
type flag struct {
	mu sync.Mutex
	v  bool
}

func (a *flag) get() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v
}

func (a *flag) set(v bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.v = v
}
