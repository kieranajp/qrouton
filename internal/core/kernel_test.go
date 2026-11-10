package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// The page resizes on every reflow, so a resize that stops short of the PTY
// leaves the runner drawing for a window the user no longer has.
func TestTermResizeReachesThePTY(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "while :; do stty size; sleep 0.05; done"}, os.Environ())
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the starting size", func() bool { return strings.Contains(rec.output(), "24 80") })

	if err := term.Resize(state.terminal, 132, 50); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child to see the new size", func() bool { return strings.Contains(rec.output(), "50 132") })
	if err := term.Resize("term-99", 100, 40); err != nil {
		t.Fatalf("a resize of a terminal nobody registered returned %v, want it ignored", err)
	}
}

// Quitting must take a session's tabs down while the session they belong to is
// still there, so a tab never outlives its owner's supervisor.
func TestQuittingStopsTabsBeforeSessions(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	dir := t.TempDir()
	order := filepath.Join(dir, "order")
	traps := func(name string) []string {
		return []string{"/bin/sh", "-c", "trap 'echo " + name + " >> " + order + "; exit 0' TERM; " +
			"touch " + filepath.Join(dir, name) + "; while :; do sleep 0.05; done"}
	}
	boot.argv = traps("session")
	c := testWorkbench(t, r, opts)
	reg, term, windows := c.Sessions, c.Term, c.Windows

	startWorkbench(t, c, opts)
	state := shownSession(t, reg)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	tab, err := windows.openWindow(state, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "tab", Cwd: t.TempDir(), Command: traps("tab"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.Start(tab, 80, 24); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"session", "tab"} {
		waitFor(t, "the "+name+"'s trap", func() bool {
			_, err := os.Stat(filepath.Join(dir, name))
			return err == nil
		})
	}

	c.Shutdown()
	got, err := os.ReadFile(order)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(strings.Fields(string(got)), " ") != "tab session" {
		t.Fatalf("quitting stopped %q, want the tab before the session", strings.Fields(string(got)))
	}
}

// A failed supervisor's session stays registered and on screen, so the page
// keeps drawing the terminal that holds its error.
func TestAFailedSupervisorKeepsItsTerminalReadable(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	boot.argv = []string{"/bin/sh", "-c", "echo boom; exit 3"}
	c := testWorkbench(t, r, opts)
	reg, term := c.Sessions, c.Term

	startWorkbench(t, c, opts)
	state := shownSession(t, reg)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the failure to be recorded", func() bool {
		view := state.agents.snapshot()
		return len(view.Records) == 1 && view.Records[0].State == agentStateFailed
	})
	// Retirement follows the recording in the same callback, so a brief hold
	// is enough to see a retirement that should not happen.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if live, ok := reg.ByTerminal(state.terminal); !ok || live != state || reg.Current() != state {
			t.Fatal("a failed supervisor retired its session and took its terminal off the screen")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The Overview tab asks for its tree on load, which can be before the
// workbench has wired the answer: that call must still get a drawable tree.
func TestChromeAgentTreeAnswersBeforeAndAfterTheTreesAreWired(t *testing.T) {
	r := newFakeClient()
	chrome := newChrome(r.Emit)
	before := chrome.AgentTree("octopus")
	if before.Provider != "" || before.Nodes == nil || len(before.Nodes) != 0 {
		t.Fatalf("an unwired tree = %+v, want no provider and an empty, non-nil node list", before)
	}

	opts, _ := testOptions(t)
	resting := filepath.Join(opts.Root, "kraken")
	if err := os.MkdirAll(resting, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(resting, session.Manifest{
		Slug: "kraken", Mode: session.ModeAssistant, Runner: agentProviderCodex,
	}); err != nil {
		t.Fatal(err)
	}
	c := testWorkbench(t, r, opts)
	reg := c.Sessions
	chrome = c.Chrome
	startWorkbench(t, c, opts)
	state := shownSession(t, reg)
	state.agents.begin(agentProviderClaude, 1)

	live := chrome.AgentTree(state.Slug())
	if live.Provider != agentProviderClaude || len(live.Nodes) != 1 || !live.Nodes[0].Root {
		t.Fatalf("a live session's tree = %+v, want its announced claude run", live)
	}
	idle := chrome.AgentTree("kraken")
	if idle.Provider != agentProviderCodex || idle.Nodes == nil || len(idle.Nodes) != 0 {
		t.Fatalf("an unbooted session's tree = %+v, want its manifest's runner and no nodes", idle)
	}
}

// A listener without the bug-report hook, or a request missing its payload, is
// refused by its guard rather than reaching a body that assumes both.
func TestDispatchGuardsBugReportsAgentEventsAndAssetFocusByScope(t *testing.T) {
	windows, _ := testWindows(t)
	owner := windows.shown()
	for _, tc := range []struct {
		name  string
		owner *SessionState
		req   workbench.Request
		want  error
	}{
		{name: "bug report status without the hook", owner: owner,
			req: workbench.Request{Op: workbench.OpBugReportStatus, ID: "one"}, want: ErrBugReportInvalid},
		{name: "bug report cancel without the hook", owner: owner,
			req: workbench.Request{Op: workbench.OpCancelBugReport, ID: "one"}, want: ErrBugReportInvalid},
		{name: "bug report queue on a session-less socket",
			req: workbench.Request{Op: workbench.OpQueueBugReport, ID: "one"}, want: ErrNoSession},
		{name: "agent event without a payload", owner: owner,
			req: workbench.Request{Op: workbench.OpAgentEvent}, want: ErrNoAgentEvent},
		{name: "agent event on a session-less socket",
			req: workbench.Request{Op: workbench.OpAgentEvent, AgentEvent: &workbench.AgentEventRequest{}}, want: ErrNoSession},
		{name: "asset focus without a payload", owner: owner,
			req: workbench.Request{Op: workbench.OpFocusAsset, ID: "window-1"}, want: ErrNoAssetFocus},
		{name: "asset focus on a session-less socket",
			req:  workbench.Request{Op: workbench.OpFocusAsset, ID: "window-1", AssetFocus: &workbench.AssetFocusRequest{Index: 1}},
			want: ErrNoSession},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control := &control{windows: windows, owner: tc.owner, hooks: controlHooks{
				agentEvent: func(workbench.AgentEventRequest) {},
			}}
			if res := control.dispatch(tc.req); res.Error != tc.want.Error() {
				t.Fatalf("dispatch(%+v) = %q, want %q", tc.req, res.Error, tc.want)
			}
		})
	}
}

// A deck's token answers only for media inside its own session, and a gallery
// entry whose file stops being media is refused rather than read.
func TestSessionMediaRoutesRefuseFilesOutsideTheSessionOrNotMedia(t *testing.T) {
	w, _ := testWindows(t)
	owner := w.shown()
	root := owner.Root()
	decks := filepath.Join(root, "thoughts", "decks")
	if err := os.MkdirAll(decks, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.png")
	for name, body := range map[string]string{
		filepath.Join(decks, "deck.md"):     "---\nmarp: true\n---\n",
		filepath.Join(decks, "shot.png"):    "picture",
		filepath.Join(decks, "capture.png"): "capture",
		filepath.Join(decks, "notes.md"):    "# not media",
		outside:                             "not the session's",
	} {
		if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(decks, "escape.png")); err != nil {
		t.Fatal(err)
	}
	deck, err := w.openWindow(owner, workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Deck: true, Source: "thoughts/decks/deck.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	deckWindow, _ := w.window(deck)
	gallery, err := w.openWindow(owner, workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatAssets,
		Assets: []workbench.AssetRef{{Source: "thoughts/decks/capture.png"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	page, _ := w.Content(gallery)

	if file, err := w.OpenDeckAsset(deckWindow.asset, "shot.png"); err != nil {
		t.Fatalf("the deck's own picture answered %v", err)
	} else {
		_ = file.Close()
	}
	if file, err := w.OpenDeckAsset(deckWindow.asset, "escape.png"); err == nil {
		_ = file.Close()
		t.Fatal("a deck asset outside its session opened")
	}
	climbed := filepath.Join(filepath.Dir(root), "climbed.png")
	if err := os.WriteFile(climbed, []byte("above the session"), 0o644); err != nil {
		t.Fatal(err)
	}
	if file, err := w.OpenDeckAsset(deckWindow.asset, "../../../climbed.png"); err == nil {
		_ = file.Close()
		t.Fatal("a deck path climbing out of its session opened")
	}
	if _, _, ok := readGallery(w, page.Assets[0].URL); !ok {
		t.Fatal("the gallery's capture did not open")
	}
	capture := filepath.Join(decks, "capture.png")
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(decks, "notes.md"), capture); err != nil {
		t.Fatal(err)
	}
	if body, _, ok := readGallery(w, page.Assets[0].URL); ok {
		t.Fatalf("a gallery asset that is not media opened as %q", body)
	}
}
