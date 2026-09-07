package desktop

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func testPresenter(t *testing.T) (*Presenter, *fakeRenderer) {
	t.Helper()
	r := newFakeRenderer()
	return newPresenter(r, r.Emit), r
}

func openedSpec(t *testing.T, r *fakeRenderer) windowSpec {
	t.Helper()
	select {
	case spec := <-r.opened:
		return spec
	default:
		t.Fatal("nothing was opened")
		return windowSpec{}
	}
}

func TestPresenterOpensItsOwnWindowWithoutTakingTheKeyboard(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	spec := openedSpec(t, r)

	if spec.Name == mainWindowName {
		t.Fatalf("the notes window opened as the conversation %q", spec.Name)
	}
	if spec.URL != notesRoot {
		t.Fatalf("URL = %q, want %q", spec.URL, notesRoot)
	}
	if spec.Focus {
		t.Fatal("the notes window took the keyboard from the presenting one")
	}
	if spec.OnClose == nil {
		t.Fatal("the notes window closes with nothing to hear it")
	}
}

func TestASecondOpenLeavesTheOneNotesWindowAlone(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	openedSpec(t, r)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}

	select {
	case spec := <-r.opened:
		t.Fatalf("a second notes window opened: %+v", spec)
	default:
	}
}

func TestClosingTheNotesWindowLeavesTheApplicationRunning(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	spec := openedSpec(t, r)
	spec.OnClose()

	r.mu.Lock()
	_, announced := r.events[presenterClosedEvent]
	r.mu.Unlock()
	if !announced {
		t.Fatalf("the page was not told the notes window went: %v", r.events)
	}
}

func TestClosingTheNotesWindowFreesTheNameForTheNextOne(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	openedSpec(t, r).OnClose()
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}

	openedSpec(t, r)
}

func TestPresenterCloseEndsOnlyTheNotesWindow(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	openedSpec(t, r)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	r.mu.Lock()
	closed := append([]string(nil), r.closed...)
	r.mu.Unlock()
	if len(closed) != 1 || closed[0] != notesWindowName {
		t.Fatalf("closed = %v, want the notes window alone", closed)
	}
}

func TestCloseWithNoNotesWindowAsksTheToolkitForNothing(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.closed) != 0 {
		t.Fatalf("closed = %v with no notes window open", r.closed)
	}
}

func TestShowReachesTheNotesWindowAloneAndNobodyElse(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	openedSpec(t, r)
	note := noteView{Index: 3, Total: 7, Title: "Fourth", HTML: "<p>Say this bit.</p>"}
	if err := p.Show(note); err != nil {
		t.Fatal(err)
	}

	r.mu.Lock()
	sent := append([]delivery(nil), r.sent...)
	_, broadcast := r.events[presenterNotesEvent]
	r.mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("sent = %+v, want one delivery", sent)
	}
	if sent[0].window != notesWindowName || sent[0].event != presenterNotesEvent {
		t.Fatalf("delivery = %+v, want the notes window's own topic", sent[0])
	}
	if sent[0].payload != note {
		t.Fatalf("payload = %+v, want %+v", sent[0].payload, note)
	}
	if broadcast {
		t.Fatal("the note went out to every window as well")
	}
}

func TestShowWithNoNotesWindowRetainsWithoutSending(t *testing.T) {
	p, r := testPresenter(t)
	note := noteView{Index: 1, Total: 4, Title: "Second", HTML: "<p>Retained.</p>"}
	if err := p.Show(note); err != nil {
		t.Fatal(err)
	}

	r.mu.Lock()
	sent := len(r.sent)
	r.mu.Unlock()
	if sent != 0 {
		t.Fatalf("sent %d payloads with no notes window open", sent)
	}
	if got := p.Note(); got != note {
		t.Fatalf("Note() = %+v, want the retained %+v", got, note)
	}
}

func TestNoteAnswersTheLastShow(t *testing.T) {
	p, _ := testPresenter(t)
	if err := p.Show(noteView{Index: 0, Total: 2, Title: "First"}); err != nil {
		t.Fatal(err)
	}
	last := noteView{Index: 1, Total: 2, Title: "Second", HTML: "<p>Latest.</p>"}
	if err := p.Show(last); err != nil {
		t.Fatal(err)
	}

	if got := p.Note(); got != last {
		t.Fatalf("Note() = %+v, want %+v", got, last)
	}
}

// The toolkit destroys a window after Close returns, so the one this presenter
// asked to go still reports itself closed — after the next one is already up.
func TestALateCloseFromARetiredWindowSaysNothing(t *testing.T) {
	p, r := testPresenter(t)
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	retired := openedSpec(t, r)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}
	openedSpec(t, r)

	retired.OnClose()

	r.mu.Lock()
	_, announced := r.events[presenterClosedEvent]
	r.mu.Unlock()
	if announced {
		t.Fatal("a retired window's close turned the page's Notes control off")
	}
	if err := p.Show(noteView{Index: 1, Total: 2}); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.sent) != 1 {
		t.Fatalf("sent %d payloads; the live notes window stopped being fed", len(r.sent))
	}
}

// A window that failed to open must not leave the presenter believing one is up,
// or the control never opens a second.
func TestAFailedOpenIsRetried(t *testing.T) {
	r := newFakeRenderer()
	refuse := newRefusingRenderer(r)
	p := newPresenter(refuse, r.Emit)
	if err := p.Open(); err == nil {
		t.Fatal("a refused open answered nil")
	}
	refuse.refuse = false
	if err := p.Open(); err != nil {
		t.Fatal(err)
	}

	openedSpec(t, r)
}

type refusingRenderer struct {
	*fakeRenderer
	refuse bool
}

func newRefusingRenderer(r *fakeRenderer) *refusingRenderer {
	return &refusingRenderer{fakeRenderer: r, refuse: true}
}

func (r *refusingRenderer) Open(spec windowSpec) error {
	if r.refuse {
		return errNoDisplay
	}
	return r.fakeRenderer.Open(spec)
}

var errNoDisplay = errors.New("no display to open a window on")

// The toolkit's per-window EmitEvent is not the narrower call it reads as: it
// stamps a sender and re-enters the same broadcast every window hears. A
// regression to it looks correct until a third window exists, which nothing
// running against one fake renderer can see.
func TestTheRendererReachesOneWindowByDispatchingToIt(t *testing.T) {
	source, err := os.ReadFile("wails.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "DispatchWailsEvent") {
		t.Fatal("the renderer no longer dispatches to a window directly")
	}
	if strings.Contains(string(source), ".EmitEvent(") {
		t.Fatal("the renderer emits through a window, which broadcasts to every one of them")
	}
}
