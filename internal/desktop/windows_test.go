package desktop

import (
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

func testWindows(t *testing.T) (*Windows, *fakeRenderer) {
	t.Helper()
	r := newFakeRenderer()
	w := newWindows(r, r.Emit, false)
	t.Cleanup(w.stopAll)
	return w, r
}

// A terminal window's page is what starts its command, so the id has to be in
// the URL the renderer is given — the page has no other way to know which
// window it is.
func TestOpenTerminalWindowCarriesItsIDInThePageURL(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(workbench.WindowOptions{
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

// escalate is the one caller that asks for focus: no agent is waiting for the
// keyboard back once the picker is up.
func TestOpenHonoursAFocusRequest(t *testing.T) {
	w, r := testWindows(t)
	if _, err := w.openWindow(workbench.WindowOptions{
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
	if _, err := w.openWindow(workbench.WindowOptions{Kind: workbench.KindTerminal, Label: "empty"}); err != ErrNoWindowCommand {
		t.Fatalf("open error = %v, want ErrNoWindowCommand", err)
	}
}

// A document has no command to run, so Start on one would index an empty argv
// and take the whole workbench with it. Under dock the agent's documents become
// tabs, which is exactly what the terminal page calls Start on.
func TestStartRefusesADocumentWindow(t *testing.T) {
	r := newFakeRenderer()
	w := newWindows(r, r.Emit, true)
	t.Cleanup(w.stopAll)

	id, err := w.openWindow(workbench.WindowOptions{
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
	id, err := w.openWindow(workbench.WindowOptions{
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
	id, err := w.openWindow(workbench.WindowOptions{
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

	plain, err := w.openWindow(workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "🔔", Content: "build finished",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened
	if page, err := w.Content(plain); err != nil || page.Format != "" {
		t.Fatalf("a plain document declared format %q", page.Format)
	}
}

// The toast is the only self-expiring window; nothing else is on a timer.
func TestADocumentWindowWithATTLClosesItself(t *testing.T) {
	w, r := testWindows(t)
	id, err := w.openWindow(workbench.WindowOptions{
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
			id, err := w.openWindow(workbench.WindowOptions{
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
	id, err := w.openWindow(workbench.WindowOptions{
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
	id, err := w.openWindow(workbench.WindowOptions{
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
	if err := w.closeWindow("window-99"); err == nil {
		t.Fatal("closed a window that was never open")
	}
}

// Closing the conversation ends the session, so nothing an agent opened may
// outlive it.
func TestStopAllTearsDownEveryWindow(t *testing.T) {
	w, r := testWindows(t)
	for range 3 {
		if _, err := w.openWindow(workbench.WindowOptions{
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
			r := newFakeRenderer()
			w := newWindows(r, r.Emit, tc.dock)
			t.Cleanup(w.stopAll)

			id, err := w.openWindow(workbench.WindowOptions{
				Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: t.TempDir(),
				Command: []string{"/bin/sh", "-c", "echo docked"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !w.exists(id) || len(w.list()) != 1 {
				t.Fatalf("registry = %v", w.list())
			}
			drawn := w.surfaces()
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
			if err := w.closeWindow(id); err != nil {
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
	r := newFakeRenderer()
	w := newWindows(r, r.Emit, true)
	t.Cleanup(w.stopAll)

	id, err := w.openWindow(workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "go test", Cwd: t.TempDir(),
		Command: []string{"/bin/sh", "-c", "exit 3"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.tabs()[0].Status; got != tabStatusRunning {
		t.Fatalf("a fresh tab reports %q, want running", got)
	}
	if err := w.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the failure to reach the tab", func() bool {
		tabs := w.tabs()
		return len(tabs) == 1 && tabs[0].Status == tabStatusFailed
	})
}

func TestAFocusedWindowFloatsEvenWhenDocking(t *testing.T) {
	r := newFakeRenderer()
	w := newWindows(r, r.Emit, true)
	t.Cleanup(w.stopAll)

	if _, err := w.openWindow(workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "escalate", Cwd: t.TempDir(),
		Command: []string{"/bin/cat"}, Focus: true,
	}); err != nil {
		t.Fatal(err)
	}
	if spec := <-r.opened; !spec.Focus {
		t.Fatal("the picker opened without focus")
	}
	if drawn := w.surfaces(); len(drawn.Tabs) != 0 || len(drawn.Floating) != 1 {
		t.Fatalf("surfaces = %+v, want the picker on screen", drawn)
	}
}
