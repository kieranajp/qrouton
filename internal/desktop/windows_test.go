package desktop

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// testWindows is a window registry over one session, which w.shown() answers
// with and every open takes as owner.
func testWindows(t *testing.T) (*Windows, *fakeRenderer) {
	t.Helper()
	return newTestWindows(t, false)
}

// testDockedWindows sends the agent's windows to the tab strip.
func testDockedWindows(t *testing.T) (*Windows, *fakeRenderer) {
	t.Helper()
	return newTestWindows(t, true)
}

func newTestWindows(t *testing.T, dock bool) (*Windows, *fakeRenderer) {
	t.Helper()
	r := newFakeRenderer()
	w := newWindows(r, r.Emit, dock, testRegistry(t, t.TempDir()))
	t.Cleanup(w.stopAll)
	return w, r
}

// A terminal window's page is what starts its command, so the id has to be in
// the URL the renderer is given — the page has no other way to know which
// window it is.
func TestOpenTerminalWindowCarriesItsIDInThePageURL(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind:    workbench.KindTerminal,
		Label:   "▶ dev",
		Cwd:     t.TempDir(),
		Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	spec := <-r.opened
	if spec.Name != id {
		t.Fatalf("window name = %q, want the id %q", spec.Name, id)
	}
	if spec.URL != terminalPage+windowIDQuery+id {
		t.Fatalf("window URL = %q, want the terminal page carrying %q", spec.URL, id)
	}
	if spec.Title != "▶ dev" {
		t.Fatalf("window title = %q", spec.Title)
	}
	if spec.Focus {
		t.Fatal("an agent-opened window took focus from the conversation")
	}
	if !w.exists(id) || len(w.list()) != 1 {
		t.Fatalf("registry = %v after one open", w.list())
	}
}

// A document the agent opened used to render behind whatever tab was up, which
// for most of a session is the shell. Terminals stay put: the tab strip focuses
// the terminal it selects, and the keyboard belongs to the conversation.
func TestADockedDocumentAsksToBeSelectedAndADockedTerminalDoesNot(t *testing.T) {
	w, r := testDockedWindows(t)

	if _, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
	}); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	_, selected := r.events[selectEvent]
	r.mu.Unlock()
	if selected {
		t.Fatal("a docked terminal pulled the right pane over to itself")
	}

	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ P006", Source: "thoughts/shared/plans/P006.md",
		Content: "# P006\n", Format: workbench.FormatMarkdown,
	})
	if err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if got := r.events[selectEvent]; got != (selection{Session: w.shown().slug(), ID: id}) {
		t.Fatalf("selected %v, want the document %q", got, id)
	}
}

// escalate is the one caller that asks for focus: no agent is waiting for the
// keyboard back once the picker is up.
func TestOpenHonoursAFocusRequest(t *testing.T) {
	w, r := testWindows(t)
	if _, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "escalate", Cwd: t.TempDir(),
		Command: []string{"/bin/cat"}, Focus: true,
	}); err != nil {
		t.Fatal(err)
	}
	if spec := <-r.opened; !spec.Focus {
		t.Fatal("a focused window was opened without focus")
	}
}

func TestOpenRejectsATerminalWindowWithNoCommand(t *testing.T) {
	w, _ := testWindows(t)
	if _, err := w.openWindow(w.shown(), workbench.WindowOptions{Kind: workbench.KindTerminal, Label: "empty"}); err != ErrNoWindowCommand {
		t.Fatalf("open error = %v, want ErrNoWindowCommand", err)
	}
}

// A document has no command to run, so Start on one would index an empty argv
// and take the whole workbench with it. Under dock the agent's documents become
// tabs, which is exactly what the terminal page calls Start on.
func TestStartRefusesADocumentWindow(t *testing.T) {
	w, _ := testDockedWindows(t)

	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "🔔", Content: "build finished",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(id, 80, 24); err != ErrNotATerminal {
		t.Fatalf("Start on a document returned %v, want ErrNotATerminal", err)
	}
	if !w.exists(id) {
		t.Fatal("the refusal took the document with it")
	}
}

func TestDocumentWindowServesItsTextToThePageAndToTheAgent(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ diff", Content: "diff --git a/x b/x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec := <-r.opened; spec.URL != documentPage+windowIDQuery+id {
		t.Fatalf("window URL = %q, want the document page", spec.URL)
	}
	page, err := w.Content(id)
	if err != nil || page.Text != "diff --git a/x b/x" {
		t.Fatalf("Content = %+v, %v", page, err)
	}
	agent, err := w.readWindow(id, false)
	if err != nil || agent != "diff --git a/x b/x" {
		t.Fatalf("read = %q, %v", agent, err)
	}
}

// The page styles a diff by line prefix, so it has to be told it holds one —
// a document that merely quotes a diff must not be painted as one.
func TestADocumentWindowCarriesItsFormatToThePageAndNotToTheAgent(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ diff",
		Content: "@@ -1 +1 @@\n-old\n+new", Format: workbench.FormatDiff,
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened
	page, err := w.Content(id)
	if err != nil {
		t.Fatal(err)
	}
	if page.Format != string(workbench.FormatDiff) {
		t.Fatalf("Content format = %q, want %q", page.Format, workbench.FormatDiff)
	}
	agent, err := w.readWindow(id, true)
	if err != nil {
		t.Fatal(err)
	}
	if agent != "@@ -1 +1 @@\n-old\n+new" {
		t.Fatalf("the agent was given something other than the diff text: %q", agent)
	}

	plain, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "🔔", Content: "build finished",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened
	if page, err := w.Content(plain); err != nil || page.Format != "" || page.Source != "" {
		t.Fatalf("a plain document declared format %q from %q", page.Format, page.Source)
	}
}

// A pane draws the path its document came from, and only the registry knows the
// window it is drawing.
func TestContentReportsTheSessionFileTheDocumentCameFrom(t *testing.T) {
	w, r := testWindows(t)
	const source = "thoughts/shared/plans/P007-2026-08-11-document-panes.md"
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "P007", Source: source, Content: "# Document panes",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened
	page, err := w.Content(id)
	if err != nil {
		t.Fatal(err)
	}
	if page.Source != source {
		t.Fatalf("Content source = %q, want %q", page.Source, source)
	}
}

// The pane scrolls to the lines the agent aimed it at and marks them, so the
// span has to reach the page. A window nobody aimed reports no lines at all,
// which is how the page knows to stay at the top.
func TestContentCarriesTheMarkedLinesToThePage(t *testing.T) {
	w, r := testWindows(t)
	for _, tc := range []struct {
		name       string
		span       workbench.LineSpan
		line, want int
	}{
		{name: "a range", span: workbench.LineSpan{Line: 12, Through: 18}, line: 12, want: 18},
		{name: "one line", span: workbench.LineSpan{Line: 12}, line: 12, want: 12},
		{name: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, err := w.openWindow(w.shown(), workbench.WindowOptions{
				Kind: workbench.KindDocument, Label: "P007", Content: "# Plan", Span: tc.span,
			})
			if err != nil {
				t.Fatal(err)
			}
			<-r.opened
			page, err := w.Content(id)
			if err != nil {
				t.Fatal(err)
			}
			if page.Line != tc.line || page.To != tc.want {
				t.Fatalf("Content lines = %d to %d, want %d to %d", page.Line, page.To, tc.line, tc.want)
			}
		})
	}
}

// The toast is the only self-expiring window; nothing else is on a timer.
func TestADocumentWindowWithATTLClosesItself(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "🔔", Content: "build finished", TTL: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened
	// Leaving the registry and leaving the screen are two steps, so waiting on
	// the first and asserting the second is a race the loaded machine loses.
	waitFor(t, "the toast to leave the screen", func() bool { return r.wasClosed(id) })
	if w.exists(id) {
		t.Fatal("the toast left the screen but stayed in the registry")
	}
}

// A docked toast is a tab the user dismisses, not a clock the way a floating
// one is.
func TestADockedAttentionWindowReportsWaitingAndOutlivesItsTTL(t *testing.T) {
	w, _ := testDockedWindows(t)

	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "🔔", Content: "build finished",
		Attention: true, TTL: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.tabs(w.shown())[0].Status; got != tabStatusWaiting {
		t.Fatalf("an attention tab reports %q, want waiting", got)
	}
	time.Sleep(60 * time.Millisecond)
	if !w.exists(id) {
		t.Fatal("a docked attention window expired on its TTL")
	}
}

// A command that succeeds takes its window with it; one that fails leaves the
// error on screen for the user to read.
func TestATerminalWindowClosesOnACleanExitAndStaysOnAFailure(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command []string
		gone    bool
	}{
		{"clean exit", []string{"/bin/sh", "-c", "exit 0"}, true},
		{"failure", []string{"/bin/sh", "-c", "exit 3"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, r := testWindows(t)
			id, err := w.openWindow(w.shown(), workbench.WindowOptions{
				Kind: workbench.KindTerminal, Label: tc.name, Cwd: t.TempDir(),
				Command: tc.command, CloseOnExit: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			<-r.opened
			if err := w.Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			if tc.gone {
				waitFor(t, "the window to close itself", func() bool { return !w.exists(id) })
				return
			}
			waitFor(t, "the exit to be reported", func() bool {
				r.mu.Lock()
				defer r.mu.Unlock()
				_, reported := r.events[windowExitEvent+id]
				return reported
			})
			if !w.exists(id) {
				t.Fatal("a failed command's window closed, taking its error with it")
			}
		})
	}
}

// The agent reads text, not a terminal: escape sequences go, and a line
// rewritten by a carriage return reads as what it ended up saying.
func TestReadStripsEscapesAndCollapsesCarriageReturns(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "output", Cwd: t.TempDir(),
		Command: []string{"/bin/sh", "-c", `printf 'a\033[31mred\033[0m\nprogress\rdone\n'`},
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened
	if err := w.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the command's output", func() bool {
		text, _ := w.readWindow(id, true)
		return strings.Contains(text, "done")
	})
	text, err := w.readWindow(id, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "\x1b") || strings.Contains(text, "31m") {
		t.Fatalf("read output still carries escape sequences: %q", text)
	}
	if !strings.Contains(text, "ared") {
		t.Fatalf("read output lost the coloured text: %q", text)
	}
	if strings.Contains(text, "progress") {
		t.Fatalf("a line rewritten by a carriage return kept both versions: %q", text)
	}
}

func TestReadWithoutFullReturnsTheLastScreenful(t *testing.T) {
	buffer := &ring{limit: windowScrollback}
	for i := 0; i < windowScreenLines*3; i++ {
		buffer.write([]byte("line\n"))
	}
	if lines := strings.Count(buffer.text(false), "\n") + 1; lines > windowScreenLines {
		t.Fatalf("a screenful is %d lines", lines)
	}
	if lines := strings.Count(buffer.text(true), "\n") + 1; lines <= windowScreenLines {
		t.Fatalf("full read returned only %d lines", lines)
	}
}

func TestRingKeepsTheTailWithinItsLimit(t *testing.T) {
	buffer := &ring{limit: 8}
	buffer.write([]byte("0123456789abc"))
	if got := buffer.text(true); got != "56789abc" {
		t.Fatalf("ring text = %q, want the last bytes", got)
	}
}

// The registry is qrouton's own map and nothing in it learns that the user
// closed a window; the close handler is what keeps it honest.
func TestAWindowTheUserClosedLeavesTheRegistry(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	(<-r.opened).OnClose()
	if w.exists(id) {
		t.Fatal("the closed window is still registered")
	}
	if _, err := w.readWindow(id, false); err == nil {
		t.Fatal("reading a closed window should say it is gone")
	}
}

func TestCloseWindowRejectsAnUnknownID(t *testing.T) {
	w, _ := testWindows(t)
	if err := w.Close("window-99"); err == nil {
		t.Fatal("closed a window that was never open")
	}
}

// Closing the conversation ends the session, so nothing an agent opened may
// outlive it.
func TestStopAllTearsDownEveryWindow(t *testing.T) {
	w, r := testWindows(t)
	for range 3 {
		if _, err := w.openWindow(w.shown(), workbench.WindowOptions{
			Kind: workbench.KindTerminal, Label: "dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
		}); err != nil {
			t.Fatal(err)
		}
		<-r.opened
	}
	w.stopAll()
	if got := w.list(); len(got) != 0 {
		t.Fatalf("windows survived the session: %v", got)
	}
}

func TestDockDecidesTheSurfaceAndNotTheRegistry(t *testing.T) {
	for _, tc := range []struct {
		name string
		dock bool
	}{
		{"float", false},
		{"dock", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, r := newTestWindows(t, tc.dock)

			id, err := w.openWindow(w.shown(), workbench.WindowOptions{
				Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: t.TempDir(),
				Command: []string{"/bin/sh", "-c", "echo docked"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !w.exists(id) || len(w.list()) != 1 {
				t.Fatalf("registry = %v", w.list())
			}
			drawn := w.surfaces(w.shown())
			if tc.dock {
				if len(drawn.Tabs) != 1 || drawn.Tabs[0].ID != id || drawn.Tabs[0].Label != "▶ dev" {
					t.Fatalf("tabs = %+v, want the window", drawn.Tabs)
				}
				if len(drawn.Floating) != 0 {
					t.Fatalf("a docked window also reached the tray: %+v", drawn.Floating)
				}
				select {
				case spec := <-r.opened:
					t.Fatalf("a docked window opened an OS window: %+v", spec)
				default:
				}
			} else {
				if spec := <-r.opened; spec.Name != id {
					t.Fatalf("window name = %q, want %q", spec.Name, id)
				}
				if len(drawn.Floating) != 1 || len(drawn.Tabs) != 0 {
					t.Fatalf("surfaces = %+v, want the tray only", drawn)
				}
			}

			if err := w.Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			waitFor(t, "the command's output", func() bool {
				text, _ := w.readWindow(id, true)
				return strings.Contains(text, "docked")
			})
			if err := w.Close(id); err != nil {
				t.Fatal(err)
			}
			if w.exists(id) {
				t.Fatal("close left the window in the registry")
			}
		})
	}
}

// A tab reports the state of its process, which is what AGENTS.md requires
// before a tab may stand in for a window.
func TestADockedTabReportsItsProcessWithoutBeingFocused(t *testing.T) {
	w, _ := testDockedWindows(t)

	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "go test", Cwd: t.TempDir(),
		Command: []string{"/bin/sh", "-c", "exit 3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.tabs(w.shown())[0].Status; got != tabStatusRunning {
		t.Fatalf("a fresh tab reports %q, want running", got)
	}
	if err := w.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the failure to reach the tab", func() bool {
		tabs := w.tabs(w.shown())
		return len(tabs) == 1 && tabs[0].Status == tabStatusFailed
	})
}

func TestAFocusedWindowFloatsEvenWhenDocking(t *testing.T) {
	w, r := testDockedWindows(t)

	if _, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "escalate", Cwd: t.TempDir(),
		Command: []string{"/bin/cat"}, Focus: true,
	}); err != nil {
		t.Fatal(err)
	}
	if spec := <-r.opened; !spec.Focus {
		t.Fatal("the picker opened without focus")
	}
	if drawn := w.surfaces(w.shown()); len(drawn.Tabs) != 0 || len(drawn.Floating) != 1 {
		t.Fatalf("surfaces = %+v, want the picker on screen", drawn)
	}
}

// A page asks for its own session's windows by slug, and drawing another
// session's tabs is drawing work the user cannot reach from that window.
func TestSurfacesAnswersEachSessionWithItsOwnWindows(t *testing.T) {
	r := newFakeRenderer()
	reg := newSessions()
	alpha := reg.add(filepath.Join(t.TempDir(), "alpha"), nil, nil)
	beta := reg.add(filepath.Join(t.TempDir(), "beta"), nil, nil)
	reg.reveal(alpha)
	w := newWindows(r, r.Emit, true, reg)
	t.Cleanup(w.stopAll)

	for _, tc := range []struct {
		owner *sessionState
		label string
	}{{alpha, "▶ alpha dev"}, {beta, "▶ beta dev"}} {
		if _, err := w.openStructural(tc.owner, workbench.WindowOptions{
			Kind: workbench.KindTerminal, Label: tc.label, Cwd: t.TempDir(), Command: []string{"/bin/cat"},
		}); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		slug  string
		label string
	}{{"alpha", "▶ alpha dev"}, {"beta", "▶ beta dev"}} {
		drawn := w.Surfaces(tc.slug)
		if drawn.Session != tc.slug {
			t.Fatalf("surfaces for %q name session %q", tc.slug, drawn.Session)
		}
		if len(drawn.Tabs) != 1 || drawn.Tabs[0].Label != tc.label {
			t.Fatalf("%q sees %+v, want only its own %q", tc.slug, drawn.Tabs, tc.label)
		}
	}
}

// Both payloads reach every session's page, so each has to say which session it
// is about before the page can decide whether it is being spoken to.
func TestTheWindowPayloadsNameTheirSession(t *testing.T) {
	w, r := testDockedWindows(t)
	owner := w.shown()

	id, err := w.openWindow(owner, workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ P006", Source: "thoughts/shared/plans/P006.md", Content: "# P006\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	opened, ok := r.events[windowsEvent].(surfaces)
	if !ok {
		t.Fatalf("no open payload reached the page: %v", r.events)
	}
	if opened.Session != owner.slug() {
		t.Fatalf("the open payload names session %q, want %q", opened.Session, owner.slug())
	}
	if got := r.events[selectEvent]; got != (selection{Session: owner.slug(), ID: id}) {
		t.Fatalf("the select payload is %#v, want the session and %q", got, id)
	}
}
