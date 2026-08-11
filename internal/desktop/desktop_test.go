package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// fakeRenderer stands in for the toolkit, so the window lifecycle runs without
// a display.
type fakeRenderer struct {
	mu     sync.Mutex
	opened chan windowSpec
	closed []string
	specs  map[string]windowSpec
	titles map[string]string
	events map[string]any
	quit   bool
	block  chan struct{}
}

func newFakeRenderer() *fakeRenderer {
	return &fakeRenderer{
		opened: make(chan windowSpec, 8),
		specs:  map[string]windowSpec{},
		titles: map[string]string{},
		events: map[string]any{},
		block:  make(chan struct{}),
	}
}

func (f *fakeRenderer) Retitle(name, title string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.titles[name] = title
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

	go func() { _ = run(r, term, newWindows(r, r.Emit, false), opts) }()

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
	go func() { done <- run(r, term, newWindows(r, r.Emit, false), opts) }()

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

// Exiting the agent is the other way out of a session: the conversation window
// closes itself and takes the workbench with it, as closing it by hand does.
func TestACleanAgentExitEndsTheSession(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	opts.Argv = []string{"/bin/sh", "-c", "exit 0"}
	term := newTerm(opts, r.Emit)

	done := make(chan error, 1)
	go func() { done <- run(r, term, newWindows(r, r.Emit, false), opts) }()
	<-r.opened

	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.quit {
		t.Fatal("the agent exited and the workbench stayed open")
	}
}

// Onboarding execs the supervisor rather than spawning it, so a session has one
// terminal: the PTY, the page it draws on and the exit that ends the session all
// survive the child replacing itself. A shell's own exec stands in for
// syscall.Exec, which a test cannot perform on itself.
func TestTheConversationSurvivesItsChildReplacingItself(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	opts.Argv = []string{"/bin/sh", "-c", `printf 'landing list\n'; exec /bin/sh -c "printf 'the agent\n'; exit 0"`}
	rec := &recorder{}
	term := newTerm(opts, rec.emit)

	done := make(chan error, 1)
	go func() { done <- run(r, term, newWindows(r, r.Emit, false), opts) }()
	<-r.opened
	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	output := rec.output()
	if !strings.Contains(output, "landing list") || !strings.Contains(output, "the agent") {
		t.Fatalf("the terminal lost half the session across the handover: %q", output)
	}
}

// A supervisor that failed has something to say, and quitting would take it off
// the screen before anyone read it.
func TestAFailedAgentLeavesItsWindowOpen(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	opts.Argv = []string{"/bin/sh", "-c", "exit 3"}
	term := newTerm(opts, r.Emit)

	go func() { _ = run(r, term, newWindows(r, r.Emit, false), opts) }()
	<-r.opened
	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the exit to reach the page", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		code, reported := r.events[ptyExitEvent]
		return reported && code == 3
	})
	r.mu.Lock()
	quit := r.quit
	r.mu.Unlock()
	if quit {
		t.Fatal("a failed agent took its window and its error with it")
	}
	r.Quit()
}

// Decision 10: a shell is always available, opened by the workbench rather than
// asked for. Asking gets you another; adoption running twice does not.
func TestTheWorkbenchOpensOneUserShellAlongsideTheConversation(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	windows := newWindows(r, r.Emit, false)

	go func() { _ = run(r, newTerm(opts, r.Emit), windows, opts) }()
	<-r.opened

	waitFor(t, "the shell tab", func() bool { return len(windows.tabs()) == 1 })
	tab := windows.tabs()[0]
	if tab.Label != shellWindowLabel {
		t.Fatalf("first tab is %q, want the user shell", tab.Label)
	}
	window, ok := windows.window(tab.ID)
	if !ok {
		t.Fatalf("the shell %q is not registered", tab.ID)
	}
	if window.opts.Cwd != opts.SessionRoot {
		t.Fatalf("the shell is rooted at %q, not the session", window.opts.Cwd)
	}
	if got := strings.Join(window.opts.Command, " "); got != "/bin/cat "+opts.SessionRoot {
		t.Fatalf("shell command = %q", got)
	}
	if window.opts.CloseOnExit {
		t.Fatal("the shell closes on exit; qrouton shell restarts the shell instead")
	}
	if len(r.opened) != 0 {
		t.Fatalf("%d OS windows opened; the shell is a tab", len(r.opened))
	}

	if err := windows.Close(tab.ID); err != nil {
		t.Fatal(err)
	}
	if len(windows.tabs()) != 0 {
		t.Fatal("the shell tab survived being closed")
	}
	for want := 1; want <= 2; want++ {
		id, err := windows.OpenShell()
		if err != nil {
			t.Fatal(err)
		}
		tabs := windows.tabs()
		if len(tabs) != want {
			t.Fatalf("%d tabs after asking %d times: %+v", len(tabs), want, tabs)
		}
		// The page selects the tab by the id it gets back, so a wrong one
		// focuses a terminal the user did not ask for.
		if tabs[len(tabs)-1].ID != id {
			t.Fatalf("OpenShell returned %q, newest tab is %q", id, tabs[len(tabs)-1].ID)
		}
	}
}

// The header's document chip is one control for two states: nothing open yet,
// and the window already showing it. A second tab on the same document is the
// failure the user sees.
func TestTheDocumentChipOpensADocumentOnceAndSelectsItAfter(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	opts.Document = func(root, name string) ([]string, error) {
		return []string{"/bin/cat", filepath.Join(root, name)}, nil
	}
	windows := newWindows(r, r.Emit, false)
	t.Cleanup(windows.stopAll)

	go func() { _ = run(r, newTerm(opts, r.Emit), windows, opts) }()
	<-r.opened
	waitFor(t, "the shell tab", func() bool { return len(windows.tabs()) == 1 })

	const doc = "thoughts/shared/research/R7-editor-surfaces.md"
	id, err := windows.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	tabs := windows.tabs()
	if len(tabs) != 2 || tabs[1].ID != id {
		t.Fatalf("the document is not the newest tab: %+v", tabs)
	}
	if tabs[1].Label != "R7-editor-surfaces.md" {
		t.Fatalf("the document tab reads %q", tabs[1].Label)
	}
	if len(r.opened) != 0 {
		t.Fatalf("%d OS windows opened; a document the user asked for is a tab", len(r.opened))
	}

	again, err := windows.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if again != id {
		t.Fatalf("a second click opened %q rather than selecting %q", again, id)
	}
	if got := len(windows.tabs()); got != 2 {
		t.Fatalf("%d tabs after clicking twice", got)
	}

	// Dismissing it is what makes the chip open one again.
	if err := windows.Close(id); err != nil {
		t.Fatal(err)
	}
	reopened, err := windows.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if reopened == id {
		t.Fatal("the dismissed document came back with its old id")
	}
}

// The agent opens documents too, through its file tool. The chip has to find
// that window rather than stack a second copy beside it.
func TestTheDocumentChipSelectsTheWindowTheAgentOpened(t *testing.T) {
	w, _ := testWindows(t)
	w.newDocument = func(string) (string, error) {
		t.Fatal("the chip opened a second window on a document already up")
		return "", nil
	}

	const doc = "thoughts/shared/plans/P006.md"
	id, err := w.openWindow(workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "Editor", Source: doc, Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := w.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("the chip returned %q, not the editor window %q", got, id)
	}
}

// Tabs that all read "$ shell" are tabs nobody can tell apart. The number goes
// in the label the registry stores, so read_window and the manifest's record
// agree with the tab strip.
func TestTheSecondShellOnwardsIsNumbered(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	windows := newWindows(r, r.Emit, false)
	t.Cleanup(windows.stopAll)

	go func() { _ = run(r, newTerm(opts, r.Emit), windows, opts) }()
	<-r.opened
	waitFor(t, "the shell tab", func() bool { return len(windows.tabs()) == 1 })

	for range 2 {
		if _, err := windows.OpenShell(); err != nil {
			t.Fatal(err)
		}
	}
	tabs := windows.tabs()
	want := []string{shellWindowLabel, "$ shell 2", "$ shell 3"}
	if len(tabs) != len(want) {
		t.Fatalf("tabs = %+v", tabs)
	}
	for i, label := range want {
		if tabs[i].Label != label {
			t.Fatalf("tab %d reads %q, want %q", i, tabs[i].Label, label)
		}
		window, ok := windows.window(tabs[i].ID)
		if !ok || window.opts.Label != label {
			t.Fatalf("the registry stores %q for the tab reading %q", window.opts.Label, label)
		}
	}
}

// On the landing-list path the workbench opens before a session exists, so the
// shell and the title bar wait for onboarding to adopt one.
func TestTheShellWindowWaitsForOnboardingToChooseASession(t *testing.T) {
	r := newFakeRenderer()
	opts := testOptions(t)
	opts.SessionRoot = ""
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	adopted := t.TempDir()
	windows := newWindows(r, r.Emit, false)

	go func() { _ = run(r, newTerm(opts, r.Emit), windows, opts) }()
	if spec := <-r.opened; spec.Title != mainWindowTitle {
		t.Fatalf("window title %q names a session nobody has chosen", spec.Title)
	}
	if len(windows.tabs()) != 0 {
		t.Fatal("a shell opened before onboarding chose a session")
	}

	host, err := (workbench.Handle{Socket: opts.Socket, SessionRoot: adopted}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Adopt(context.Background(), adopted); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the shell tab", func() bool { return len(windows.tabs()) == 1 })
	window, ok := windows.window(windows.tabs()[0].ID)
	if !ok || window.opts.Cwd != adopted {
		t.Fatalf("the shell is not rooted in the adopted session: %+v", window)
	}
	waitFor(t, "the retitled conversation", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return strings.Contains(r.titles[mainWindowName], filepath.Base(adopted))
	})
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

	pushChrome(dir, status.ActivityIdle, nil, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	if fields.Mode == "" || fields.Phase == "" || fields.Identity != "Octopus" {
		t.Fatalf("chrome = %+v, want the session's mode, phase and name", fields)
	}
}

func TestChromeStaysSilentWithoutAManifest(t *testing.T) {
	r := newFakeRenderer()
	pushChrome(t.TempDir(), status.ActivityIdle, nil, r.Emit)
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
		watchChrome(ctx, func() string { return dir }, func() string { return "" }, func(string, any) {})
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
	windows := newWindows(r, r.Emit, false)
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
		fields, ok := r.events[chromeEvent].(status.Fields)
		return ok && fields.Identity == "Adopted"
	})
}
