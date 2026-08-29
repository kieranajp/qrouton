package desktop

import (
	"testing"

	"github.com/kieranajp/qrouton/internal/status"
)

// The gate is only ever a live answer: it goes up seconds after launch, once
// the release feed has been read, so the window has to redraw around it.
func TestChromeReportsTheUpdateGate(t *testing.T) {
	root := t.TempDir()
	for _, held := range []bool{false, true} {
		r := newFakeRenderer()
		pushChrome(newSessions(), root, nil, func() bool { return held }, nil, nil, r.Emit)
		r.mu.Lock()
		fields, ok := r.events[chromeEvent].(status.Fields)
		r.mu.Unlock()
		if !ok {
			t.Fatalf("no chrome pushed at the window: %v", r.events)
		}
		if fields.Outdated != held {
			t.Errorf("outdated = %v, want %v", fields.Outdated, held)
		}
	}
}

// A window with no gate wired — every test driving run against a fake renderer,
// and any install that cannot replace itself — is never outdated.
func TestChromeWithoutAKeeperIsNeverOutdated(t *testing.T) {
	r := newFakeRenderer()
	pushChrome(newSessions(), t.TempDir(), nil, nil, nil, nil, r.Emit)
	r.mu.Lock()
	defer r.mu.Unlock()
	fields := r.events[chromeEvent].(status.Fields)
	if fields.Outdated {
		t.Error("a window with no update policy reported itself outdated")
	}
}

// The swap only happens where it costs the user nothing. A conversation is the
// obvious case; a half-filled session draft is the one that is easy to miss,
// and a relaunch would throw the answers away.
func TestARelaunchWaitsForAConversationAndForADraft(t *testing.T) {
	root := t.TempDir()
	empty := newSessions()

	if !idleFor(empty, func() bool { return false })() {
		t.Error("a workbench holding nothing was not idle")
	}
	if idleFor(empty, func() bool { return true })() {
		t.Error("a workbench with a draft open was treated as idle")
	}
	if idleFor(testRegistry(t, root), func() bool { return false })() {
		t.Error("a workbench holding a conversation was treated as idle")
	}
}

// Nothing asks the assembly service whether a draft is open, so a workbench
// wired without one still answers.
func TestIdleWithoutADraftSourceOnlyAsksTheRegistry(t *testing.T) {
	if !idleFor(newSessions(), nil)() {
		t.Error("a workbench with no draft source was not idle")
	}
	if idleFor(testRegistry(t, t.TempDir()), nil)() {
		t.Error("a conversation did not hold off the relaunch")
	}
}

// An install that cannot replace itself carries no policy: a `go build` on a
// developer's path has no bundle the helper could swap, so it gets a Keeper
// that runs nothing rather than one that downloads a release it cannot apply.
func TestAnUnbundledBuildCarriesNoInstaller(t *testing.T) {
	reg := newSessions()
	keeper := newKeeper(nil, reg, nil, "0.4.0")
	if keeper.Installer != nil {
		t.Error("a build outside an app bundle was given an installer")
	}
	if keeper.Held() {
		t.Error("a build that cannot update itself was held")
	}
	if keeper.Idle == nil || !keeper.Idle() {
		t.Error("the keeper was not wired to the registry it must wait on")
	}
}
