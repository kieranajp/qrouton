package desktop

import (
	"errors"
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
	r := newFakeRenderer()
	w := newWindows(r.Emit, testRegistry(t, t.TempDir()))
	t.Cleanup(w.stopAll)
	return w, r
}

func TestOpenTerminalWindowRegistersATabWithoutOpeningARendererWindow(t *testing.T) {
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
	if !w.exists(id) || len(w.list()) != 1 {
		t.Fatalf("registry = %v after one open", w.list())
	}
	if tabs := w.tabs(w.shown()); len(tabs) != 1 || tabs[0].ID != id || tabs[0].Label != "▶ dev" {
		t.Fatalf("tabs = %+v, want the opened terminal", tabs)
	}
	if got := w.Surfaces(w.shown().slug()).Selected; got != id {
		t.Fatalf("selected = %q, want the agent terminal %q", got, id)
	}
	select {
	case spec := <-r.opened:
		t.Fatalf("an agent tab opened a renderer window: %+v", spec)
	default:
	}
}

func TestEveryAgentWindowIsSelectedAndStructuralWindowsAreNot(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts workbench.WindowOptions
	}{
		{name: "terminal", opts: workbench.WindowOptions{
			Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
		}},
		{name: "document", opts: workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "◆ P006", Source: "thoughts/shared/plans/P006.md",
			Content: "# P006\n", Format: workbench.FormatMarkdown,
		}},
		{name: "attention", opts: workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "🔔", Content: "build finished", Attention: true,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := testWindows(t)
			if _, err := w.openStructural(w.shown(), workbench.WindowOptions{
				Kind: workbench.KindTerminal, Label: "$ shell", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
			}); err != nil {
				t.Fatal(err)
			}
			if got := w.Surfaces(w.shown().slug()).Selected; got != "" {
				t.Fatalf("structural shell selected %q", got)
			}
			id, err := w.openWindow(w.shown(), tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			if got := w.Surfaces(w.shown().slug()).Selected; got != id {
				t.Fatalf("selected = %q, want agent window %q", got, id)
			}
		})
	}
}

func TestOpenRejectsATerminalWindowWithNoCommand(t *testing.T) {
	w, _ := testWindows(t)
	if _, err := w.openWindow(w.shown(), workbench.WindowOptions{Kind: workbench.KindTerminal, Label: "empty"}); err != ErrNoWindowCommand {
		t.Fatalf("open error = %v, want ErrNoWindowCommand", err)
	}
}

// A document has no command to run, so Start on one would index an empty argv
// and take the whole workbench with it. Document tabs never call Start.
func TestStartRefusesADocumentWindow(t *testing.T) {
	w, _ := testWindows(t)

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
	w, _ := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ diff", Content: "diff --git a/x b/x",
	})
	if err != nil {
		t.Fatal(err)
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
	w, _ := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ diff",
		Content: "@@ -1 +1 @@\n-old\n+new", Format: workbench.FormatDiff,
	})
	if err != nil {
		t.Fatal(err)
	}
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
	if page, err := w.Content(plain); err != nil || page.Format != "" || page.Source != "" {
		t.Fatalf("a plain document declared format %q from %q", page.Format, page.Source)
	}
}

// A pane draws the path its document came from, and only the registry knows the
// window it is drawing.
func TestContentReportsTheSessionFileTheDocumentCameFrom(t *testing.T) {
	w, _ := testWindows(t)
	const source = "thoughts/shared/plans/P007-2026-08-11-document-panes.md"
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "P007", Source: source, Content: "# Document panes",
	})
	if err != nil {
		t.Fatal(err)
	}
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
	w, _ := testWindows(t)
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

func TestMarkdownViewportStartsUnavailableAndAcceptsOnlyNewValidReports(t *testing.T) {
	w, _ := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown,
		Source: "thoughts/shared/plans/P1.md", Content: "# Plan\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := w.Content(id)
	if err != nil || page.ViewportEpoch == 0 {
		t.Fatalf("Content viewport epoch = %d, %v", page.ViewportEpoch, err)
	}
	epoch := page.ViewportEpoch
	view, err := w.viewport(w.shown(), id)
	if err != nil || view == nil || view.Available || view.Selected || view.Intervals == nil {
		t.Fatalf("initial viewport = %+v, %v", view, err)
	}
	if err := w.ReportViewport(id, ViewportReport{
		Epoch: epoch, Seq: 2, Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 9, To: 10}, {Line: 3, To: 5}, {Line: 6, To: 8}},
	}); err != nil {
		t.Fatal(err)
	}
	view, err = w.viewport(w.shown(), id)
	if err != nil || view.Source != "thoughts/shared/plans/P1.md" || len(view.Intervals) != 1 ||
		view.Intervals[0] != (workbench.LineInterval{Line: 3, To: 10}) {
		t.Fatalf("measured viewport = %+v, %v", view, err)
	}
	if err := w.ReportViewport(id, ViewportReport{Epoch: epoch, Seq: 1}); err != nil {
		t.Fatal(err)
	}
	if stale, _ := w.viewport(w.shown(), id); !stale.Available || len(stale.Intervals) != 1 {
		t.Fatalf("stale report replaced viewport: %+v", stale)
	}
	if err := w.ReportViewport(id, ViewportReport{
		Epoch: epoch, Seq: 3, Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 8, To: 7}},
	}); !errors.Is(err, ErrInvalidViewport) {
		t.Fatalf("invalid report error = %v", err)
	}
	if after, _ := w.viewport(w.shown(), id); !after.Available || len(after.Intervals) != 1 {
		t.Fatalf("invalid report replaced viewport: %+v", after)
	}
	if err := w.ReportViewport(id, ViewportReport{Epoch: epoch, Seq: 4, Selected: false, Available: true,
		Intervals: []workbench.LineInterval{{Line: 3, To: 5}}}); err != nil {
		t.Fatal(err)
	}
	if inactive, _ := w.viewport(w.shown(), id); inactive.Available || inactive.Selected || len(inactive.Intervals) != 0 || inactive.Intervals == nil {
		t.Fatalf("inactive viewport = %+v", inactive)
	}

	reloaded, err := w.Content(id)
	if err != nil || reloaded.ViewportEpoch <= epoch {
		t.Fatalf("reloaded viewport epoch = %d after %d, %v", reloaded.ViewportEpoch, epoch, err)
	}
	if reset, _ := w.viewport(w.shown(), id); reset.Available || reset.Selected || reset.Intervals == nil {
		t.Fatalf("reload did not reset viewport: %+v", reset)
	}
	if err := w.ReportViewport(id, ViewportReport{
		Epoch: epoch, Seq: 99, Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 30, To: 30}},
	}); err != nil {
		t.Fatal(err)
	}
	if stale, _ := w.viewport(w.shown(), id); stale.Available || stale.Selected || len(stale.Intervals) != 0 {
		t.Fatalf("pre-reload report replaced viewport: %+v", stale)
	}
	if err := w.ReportViewport(id, ViewportReport{
		Epoch: reloaded.ViewportEpoch, Seq: 1, Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 12, To: 14}},
	}); err != nil {
		t.Fatal(err)
	}
	if current, _ := w.viewport(w.shown(), id); !current.Available || !current.Selected ||
		len(current.Intervals) != 1 || current.Intervals[0] != (workbench.LineInterval{Line: 12, To: 14}) {
		t.Fatalf("post-reload report = %+v", current)
	}
}

func TestViewportIsMarkdownOnlyAndOwnerScoped(t *testing.T) {
	r := newFakeRenderer()
	reg := newSessions()
	alpha := reg.add(filepath.Join(t.TempDir(), "alpha"), nil, nil)
	beta := reg.add(filepath.Join(t.TempDir(), "beta"), nil, nil)
	w := newWindows(r.Emit, reg)
	t.Cleanup(w.stopAll)
	terminal, err := w.openStructural(alpha, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Cwd: t.TempDir(), Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if view, err := w.viewport(alpha, terminal); err != nil || view != nil {
		t.Fatalf("terminal viewport = %+v, %v", view, err)
	}
	markdown, err := w.openWindow(alpha, workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Source: "plan.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.viewport(beta, markdown); err == nil {
		t.Fatal("another session inspected the Markdown viewport")
	}
	if err := w.ReportViewport(terminal, ViewportReport{Seq: 1}); !errors.Is(err, ErrNoViewport) {
		t.Fatalf("terminal report error = %v", err)
	}
	w.discard(markdown)
	if _, err := w.viewport(alpha, markdown); err == nil {
		t.Fatal("discarded viewport remained readable")
	}
}

func TestAnAttentionWindowReportsWaitingAndRemainsUntilClosed(t *testing.T) {
	w, _ := testWindows(t)

	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "🔔", Content: "build finished",
		Attention: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := w.tabs(w.shown())[0].Status; got != tabStatusWaiting {
		t.Fatalf("an attention tab reports %q, want waiting", got)
	}
	if !w.exists(id) {
		t.Fatal("an attention tab closed without being dismissed")
	}
	if err := w.Close(id); err != nil {
		t.Fatal(err)
	}
	if w.exists(id) {
		t.Fatal("the attention tab survived being closed")
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
	w, _ := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "output", Cwd: t.TempDir(),
		Command: []string{"/bin/sh", "-c", `printf 'a\033[31mred\033[0m\nprogress\rdone\n'`},
	})
	if err != nil {
		t.Fatal(err)
	}
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

func TestCloseWindowRejectsAnUnknownID(t *testing.T) {
	w, _ := testWindows(t)
	if err := w.Close("window-99"); err == nil {
		t.Fatal("closed a window that was never open")
	}
}

func TestClosingTheSelectedWindowFallsBackToTheOldestRemainingTab(t *testing.T) {
	w, _ := testWindows(t)
	owner := w.shown()
	var ids []string
	for range 3 {
		id, err := w.openStructural(owner, workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "document",
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := w.Select(owner.slug(), ids[2]); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- w.Close(ids[2]) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("closing the selected tab deadlocked")
	}
	if got := w.Surfaces(owner.slug()).Selected; got != ids[0] {
		t.Fatalf("selected = %q, want oldest remaining %q", got, ids[0])
	}
	if err := w.Close(ids[0]); err != nil {
		t.Fatal(err)
	}
	if got := w.Surfaces(owner.slug()).Selected; got != ids[1] {
		t.Fatalf("selected = %q, want last remaining %q", got, ids[1])
	}
	if err := w.Close(ids[1]); err != nil {
		t.Fatal(err)
	}
	if got := w.Surfaces(owner.slug()).Selected; got != "" {
		t.Fatalf("empty tab strip selected %q", got)
	}
}

// Closing the conversation ends the session, so nothing an agent opened may
// outlive it.
func TestStopAllTearsDownEveryWindow(t *testing.T) {
	w, _ := testWindows(t)
	for range 3 {
		if _, err := w.openWindow(w.shown(), workbench.WindowOptions{
			Kind: workbench.KindTerminal, Label: "dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	w.stopAll()
	if got := w.list(); len(got) != 0 {
		t.Fatalf("windows survived the session: %v", got)
	}
}

// A tab reports the state of its process, which is what AGENTS.md requires
// before a tab may stand in for a window.
func TestATabReportsItsProcessWithoutBeingFocused(t *testing.T) {
	w, _ := testWindows(t)

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

// A page asks for its own session's windows by slug, and drawing another
// session's tabs is drawing work the user cannot reach from that window.
func TestSurfacesAnswersEachSessionWithItsOwnWindows(t *testing.T) {
	r := newFakeRenderer()
	reg := newSessions()
	alpha := reg.add(filepath.Join(t.TempDir(), "alpha"), nil, nil)
	beta := reg.add(filepath.Join(t.TempDir(), "beta"), nil, nil)
	reg.reveal(alpha)
	w := newWindows(r.Emit, reg)
	t.Cleanup(w.stopAll)

	for _, tc := range []struct {
		owner *sessionState
		label string
	}{{alpha, "▶ alpha dev"}, {beta, "▶ beta dev"}} {
		if _, err := w.openWindow(tc.owner, workbench.WindowOptions{
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
		if drawn.Selected != drawn.Tabs[0].ID {
			t.Fatalf("%q restored selected %q, want %q", tc.slug, drawn.Selected, drawn.Tabs[0].ID)
		}
	}
	alphaID := w.Surfaces("alpha").Tabs[0].ID
	if err := w.Select("beta", alphaID); err == nil {
		t.Fatal("beta selected alpha's tab")
	}
	if err := w.Select("missing", alphaID); err == nil {
		t.Fatal("an unknown session selected a tab")
	}
}

// Both payloads reach every session's page, so each has to say which session it
// is about before the page can decide whether it is being spoken to.
func TestTheWindowPayloadsNameTheirSession(t *testing.T) {
	w, r := testWindows(t)
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
	if opened.Selected != id {
		t.Fatalf("the complete payload selects %q, want %q", opened.Selected, id)
	}
}
