package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// fakeRenderer stands in for the toolkit, so the window lifecycle runs without
// a display.
type fakeRenderer struct {
	mu     sync.Mutex
	opened chan windowSpec
	closed []string
	specs  map[string]windowSpec
	events map[string]any
	quit   bool
	block  chan struct{}
}

func newFakeRenderer() *fakeRenderer {
	return &fakeRenderer{
		opened: make(chan windowSpec, 8),
		specs:  map[string]windowSpec{},
		events: map[string]any{},
		block:  make(chan struct{}),
	}
}

func (f *fakeRenderer) Open(spec windowSpec) error {
	f.mu.Lock()
	f.specs[spec.Name] = spec
	f.mu.Unlock()
	f.opened <- spec
	return nil
}

// Close mirrors the toolkit: taking a window off the screen still fires its
// close handler, which is what makes the registry's teardown idempotent.
func (f *fakeRenderer) Close(name string) {
	f.mu.Lock()
	f.closed = append(f.closed, name)
	spec, ok := f.specs[name]
	delete(f.specs, name)
	f.mu.Unlock()
	if ok && spec.OnClose != nil {
		spec.OnClose()
	}
}

func (f *fakeRenderer) Emit(event string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events[event] = payload
}

func (f *fakeRenderer) Run() error {
	<-f.block
	return nil
}

func (f *fakeRenderer) Quit() {
	f.mu.Lock()
	f.quit = true
	f.mu.Unlock()
	close(f.block)
}

func (f *fakeRenderer) wasClosed(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, closed := range f.closed {
		if closed == name {
			return true
		}
	}
	return false
}

// testOptions is a session the workbench can open on, with an address of its
// own so parallel tests do not fight over one socket.
func testOptions(t *testing.T) Options {
	t.Helper()
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(socket) })
	return Options{SessionRoot: t.TempDir(), Socket: socket, Argv: []string{"/bin/cat"}, Env: os.Environ()}
}

func TestRunOpensOneConversationWindowAtTheFrontendRoot(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	term := newTerm(opts, r.Emit)

	go func() { _ = run(r, term, newWindows(r, r.Emit), opts) }()

	spec := <-r.opened
	if spec.URL != frontendRoot {
		t.Fatalf("window URL is %q; a page path 301-redirects into a blank window", spec.URL)
	}
	if !strings.Contains(spec.Title, filepath.Base(opts.SessionRoot)) {
		t.Fatalf("window title %q does not name the session", spec.Title)
	}
	if spec.OnClose == nil {
		t.Fatal("conversation window has no close handler; the session would outlive its window")
	}
	if len(r.opened) != 0 {
		t.Fatalf("%d further windows opened; the workbench opens one", len(r.opened))
	}
}

// Closing the conversation window ends the session: no detached server, and
// nothing surviving in the background.
func TestClosingTheConversationWindowQuits(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	term := newTerm(opts, r.Emit)

	done := make(chan error, 1)
	go func() { done <- run(r, term, newWindows(r, r.Emit), opts) }()

	(<-r.opened).OnClose()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.quit {
		t.Fatal("closing the conversation window left the application running")
	}
}

func TestRunRefusesASessionWithNothingToRun(t *testing.T) {
	if err := Run(Options{SessionRoot: t.TempDir(), Socket: "/tmp/x.sock"}); err != ErrNoAgentCommand {
		t.Fatalf("Run with no argv returned %v, want ErrNoAgentCommand", err)
	}
	if err := Run(Options{SessionRoot: t.TempDir(), Argv: []string{"/bin/cat"}}); err != ErrNoControlSocket {
		t.Fatalf("Run with no socket returned %v, want ErrNoControlSocket", err)
	}
}

// The manifest is the chrome's only source, so an escalation shows up on the
// next poll rather than needing the window to be told.
func TestChromePushesTheManifestsModePhaseAndName(t *testing.T) {
	r := newFakeRenderer()
	dir := t.TempDir()
	if err := session.WriteManifest(dir, session.Manifest{Slug: "octopus", Name: "Octopus", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}

	pushChrome(dir, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(chromeFields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	if fields.Mode == "" || fields.Phase == "" || fields.Identity != "octopus" {
		t.Fatalf("chrome = %+v, want the session's mode, phase and slug", fields)
	}
}

func TestChromeStaysSilentWithoutAManifest(t *testing.T) {
	r := newFakeRenderer()
	pushChrome(t.TempDir(), r.Emit)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, pushed := r.events[chromeEvent]; pushed {
		t.Fatal("chrome pushed for a directory with no manifest")
	}
}

func TestWatchChromeStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	done := make(chan struct{})
	go func() {
		watchChrome(ctx, func() string { return dir }, func(string, any) {})
		close(done)
	}()
	<-done
}

// Adopt is how onboarding names the session it chose, so the chrome has to read
// the root on every tick rather than capturing it once.
func TestAdoptRepointsTheChromeAtTheAdoptedSession(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	adopted := t.TempDir()
	if err := session.WriteManifest(adopted, session.Manifest{Slug: "adopted", Name: "Adopted", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	windows := newWindows(r, r.Emit)
	go func() { _ = run(r, newTerm(opts, r.Emit), windows, opts) }()
	<-r.opened

	host, err := (workbench.Handle{Socket: opts.Socket, SessionRoot: opts.SessionRoot}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Adopt(context.Background(), adopted); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the adopted session's chrome", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		fields, ok := r.events[chromeEvent].(chromeFields)
		return ok && fields.Identity == "adopted"
	})
}
