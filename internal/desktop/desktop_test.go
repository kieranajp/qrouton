package desktop

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
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

// stubBoot stands in for what a session needs to come up, counting the
// supervisor commands and the listeners asked for and keeping each address.
type stubBoot struct {
	argv []string

	mu      sync.Mutex
	sockets map[string]string
	resumes map[string]bool
	agents  int
	serves  int
}

func newStubBoot(argv ...string) *stubBoot {
	return &stubBoot{argv: argv, sockets: map[string]string{}, resumes: map[string]bool{}}
}

func (b *stubBoot) command(sessionRoot, socket string, resume bool) ([]string, []string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.agents++
	b.sockets[sessionRoot] = socket
	b.resumes[sessionRoot] = resume
	return b.argv, os.Environ(), nil
}

func (b *stubBoot) served() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.serves++
}

// resumed reports whether the session rooted at sessionRoot was asked to
// continue its previous conversation.
func (b *stubBoot) resumed(t *testing.T, sessionRoot string) bool {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.resumes[sessionRoot]
}

// socket is the address the session rooted at sessionRoot answers on.
func (b *stubBoot) socket(t *testing.T, sessionRoot string) string {
	t.Helper()
	var socket string
	waitFor(t, "the session's own control socket", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		socket = b.sockets[sessionRoot]
		return socket != "" && workbench.Answered(socket)
	})
	return socket
}

func (b *stubBoot) counts() (agents, serves int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.agents, b.serves
}

// testOptions is a session the workbench can open on, under a sessions root it
// can boot others from, on an address of its own.
func testOptions(t *testing.T) (Options, *stubBoot) {
	t.Helper()
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(socket) })
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	return Options{
		SessionRoot: sessionDir(t, root, "octopus"),
		Root:        root,
		Socket:      socket,
		Onboard:     []string{"/bin/cat"},
		Env:         os.Environ(),
		Agent:       boot.command,
	}, boot
}

// testWorkbench wires a conversation and a window registry over one session
// registry, as Run does; run adds the sessions to it.
func testWorkbench(t *testing.T, r *fakeRenderer, emit emitter) (*Sessions, *Term, *Windows) {
	t.Helper()
	reg := newSessions()
	windows := newWindows(r, r.Emit, false, reg)
	t.Cleanup(reg.stopAll)
	t.Cleanup(windows.stopAll)
	return reg, newTerm(reg, emit), windows
}

// testSessions is a registry that boots sessions as run does: a supervisor
// command per session and a listener of its own, over one window registry.
func testSessions(t *testing.T, root string, boot *stubBoot) (*Sessions, *Windows, *fakeRenderer) {
	t.Helper()
	r := newFakeRenderer()
	reg := newSessions()
	windows := newWindows(r, r.Emit, false, reg)
	t.Cleanup(reg.stopAll)
	t.Cleanup(windows.stopAll)
	reg.boot = booting{
		root: func(slug string) string {
			if root == "" || slug == "" {
				return ""
			}
			dir := filepath.Join(root, slug)
			if _, err := os.Stat(sessionpaths.Manifest(dir)); err != nil {
				return ""
			}
			return dir
		},
		agent: boot.command,
		serve: func(state *sessionState, socket string) (io.Closer, error) {
			boot.served()
			return serveControl(socket, windows, state, controlHooks{attention: state.activity.hook})
		},
		teardown: windows.stop,
	}
	return reg, windows, r
}

// testRegistry is a workbench showing one session.
func testRegistry(t *testing.T, root string) *Sessions {
	t.Helper()
	reg := newSessions()
	reg.reveal(reg.add(root, []string{"/bin/cat"}, os.Environ()))
	return reg
}

// sessionDir is a session's directory under the sessions root, which is where
// the rail looks for one it has not booted. The manifest is what makes it a
// session rather than a directory.
func sessionDir(t *testing.T, root, slug string) string {
	t.Helper()
	dir := filepath.Join(root, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(dir, session.Manifest{Slug: slug, Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// shownSession is the session run put on screen, which it does once the
// conversation window is open.
func shownSession(t *testing.T, reg *Sessions) *sessionState {
	t.Helper()
	var state *sessionState
	waitFor(t, "the session on screen", func() bool {
		state = reg.current()
		return state != nil
	})
	return state
}

// endConversation ends a session the way its supervisor does: cat exits zero on
// end-of-transmission.
func endConversation(t *testing.T, term *Term, state *sessionState) {
	t.Helper()
	if err := term.Write(state.terminal, base64.StdEncoding.EncodeToString([]byte{0x04})); err != nil {
		t.Fatal(err)
	}
}

// writeAgentPID records a session's agent pid, as its supervisor does.
func writeAgentPID(t *testing.T, root string, pid int) {
	t.Helper()
	if err := os.MkdirAll(sessionpaths.Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.AgentPID(root), []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// deadPID is a pid nothing answers to: a process this test started and reaped.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestRunOpensOneConversationWindowAtTheFrontendRoot(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()

	spec := <-r.opened
	shownSession(t, reg)
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
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	done := make(chan error, 1)
	go func() { done <- run(r, term, windows, opts) }()

	conversation := <-r.opened
	shownSession(t, reg)
	conversation.OnClose()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.quit {
		t.Fatal("closing the conversation window left the application running")
	}
}

// A supervisor exiting ends its own session and not the app: the window falls
// back to the session shown before it, and to none once the last one has gone.
func TestACleanSupervisorExitRetiresOnlyItsSession(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	first := shownSession(t, reg)

	sessionDir(t, opts.Root, "kraken")
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	second := reg.current()
	for _, state := range []*sessionState{first, second} {
		if err := term.Start(state.terminal, 80, 24); err != nil {
			t.Fatal(err)
		}
	}
	socket := boot.socket(t, second.root())

	endConversation(t, term, second)

	waitFor(t, "the window to fall back to the session shown before it", func() bool {
		return reg.current() == first
	})
	if _, live := reg.byTerminal(second.terminal); live {
		t.Fatalf("the retired session %q is still registered", second.slug())
	}
	if workbench.Answered(socket) {
		t.Fatalf("the retired session still answers on %q", socket)
	}

	endConversation(t, term, first)
	waitFor(t, "the window with no session", func() bool { return reg.current() == nil })
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.quit {
		t.Fatal("the last supervisor exiting took the workbench with it")
	}
}

// Closing the conversation window is the one way out of the app, and every
// session goes with it: no supervisor left running, no listener left answering.
func TestClosingTheConversationWindowTearsDownEverySession(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	done := make(chan error, 1)
	go func() { done <- run(r, term, windows, opts) }()
	conversation := <-r.opened
	first := shownSession(t, reg)

	sessionDir(t, opts.Root, "kraken")
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}

	var pids []int
	var sockets []string
	for _, state := range []*sessionState{first, reg.current()} {
		if err := term.Start(state.terminal, 80, 24); err != nil {
			t.Fatal(err)
		}
		pids = append(pids, state.process.cmd.Process.Pid)
		sockets = append(sockets, boot.socket(t, state.root()))
	}

	conversation.OnClose()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for _, pid := range pids {
		waitFor(t, "the supervisor to die", func() bool { return syscall.Kill(pid, 0) != nil })
	}
	for _, socket := range sockets {
		if workbench.Answered(socket) {
			t.Fatalf("a session still answers on %q after its window closed", socket)
		}
	}
}

// Onboarding execs the supervisor rather than spawning it, so a session has one
// terminal: the PTY, the page it draws on and the exit that ends the session all
// survive the child replacing itself. A shell's own exec stands in for
// syscall.Exec, which a test cannot perform on itself.
func TestTheConversationSurvivesItsChildReplacingItself(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	boot.argv = []string{"/bin/sh", "-c", `printf 'landing list\n'; exec /bin/sh -c "printf 'the agent\n'; exit 0"`}
	rec := &recorder{}
	reg, term, windows := testWorkbench(t, r, rec.emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	if err := term.Start(shownSession(t, reg).terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "both halves of the session", func() bool {
		output := rec.output()
		return strings.Contains(output, "landing list") && strings.Contains(output, "the agent")
	})
}

// A supervisor that failed has something to say, and quitting would take it off
// the screen before anyone read it.
func TestAFailedAgentLeavesItsWindowOpen(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	boot.argv = []string{"/bin/sh", "-c", "exit 3"}
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	conversation := shownSession(t, reg).terminal
	if err := term.Start(conversation, 80, 24); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the exit to reach the page", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		code, reported := r.events[ptyExitEvent+conversation]
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
	opts, _ := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	owner := shownSession(t, reg)

	waitFor(t, "the shell tab", func() bool { return len(windows.tabs(owner)) == 1 })
	tab := windows.tabs(owner)[0]
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
	if len(windows.tabs(owner)) != 0 {
		t.Fatal("the shell tab survived being closed")
	}
	for want := 1; want <= 2; want++ {
		id, err := windows.OpenShell()
		if err != nil {
			t.Fatal(err)
		}
		tabs := windows.tabs(owner)
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
	opts, _ := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	opts.Document = func(root, name string) (workbench.WindowOptions, error) {
		return workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "◆ " + filepath.Base(name),
			Source: name, Content: "# " + name, Format: workbench.FormatMarkdown,
		}, nil
	}
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	owner := shownSession(t, reg)
	waitFor(t, "the shell tab", func() bool { return len(windows.tabs(owner)) == 1 })

	const doc = "thoughts/shared/research/R7-editor-surfaces.md"
	id, err := windows.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	tabs := windows.tabs(owner)
	if len(tabs) != 2 || tabs[1].ID != id {
		t.Fatalf("the document is not the newest tab: %+v", tabs)
	}
	if tabs[1].Label != "◆ R7-editor-surfaces.md" {
		t.Fatalf("the document tab reads %q", tabs[1].Label)
	}
	if tabs[1].Kind != string(workbench.KindDocument) {
		t.Fatalf("the document tab is a %q; a rendered pane runs no process", tabs[1].Kind)
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
	if got := len(windows.tabs(owner)); got != 2 {
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

// The rail's add-repos button. The old terminal picker took the whole screen,
// which is what made adding a repository feel like leaving the conversation;
// here it is a tab, and a second click selects the one already open rather than
// racing a second picker at the same manifest.
func TestTheAddReposButtonOpensOnePickerTab(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	opts.Picker = func(dir string) []string { return []string{"/bin/cat", "pick", dir} }
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	owner := shownSession(t, reg)
	waitFor(t, "the shell tab", func() bool { return len(windows.tabs(owner)) == 1 })

	id, err := windows.OpenPicker()
	if err != nil {
		t.Fatal(err)
	}
	tabs := windows.tabs(owner)
	if len(tabs) != 2 || tabs[1].ID != id {
		t.Fatalf("the picker is not the newest tab: %+v", tabs)
	}
	if len(r.opened) != 0 {
		t.Fatalf("%d OS windows opened; the picker is a tab", len(r.opened))
	}
	window, ok := windows.window(id)
	if !ok {
		t.Fatalf("the picker %q is not registered", id)
	}
	if got := strings.Join(window.opts.Command, " "); got != "/bin/cat pick "+opts.SessionRoot {
		t.Fatalf("picker command = %q", got)
	}
	if !window.opts.CloseOnExit {
		t.Fatal("the picker tab outlives the picker")
	}

	again, err := windows.OpenPicker()
	if err != nil {
		t.Fatal(err)
	}
	if again != id || len(windows.tabs(owner)) != 2 {
		t.Fatalf("a second click opened %q rather than selecting %q", again, id)
	}
}

func TestConcurrentAddReposClicksOpenOnePicker(t *testing.T) {
	w, _ := testWindows(t)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	w.newPicker = func() (string, error) {
		entered <- struct{}{}
		<-release
		return w.openStructural(w.shown(), workbench.WindowOptions{
			Kind: workbench.KindTerminal, Label: pickerWindowLabel, Source: pickerSource,
			Command: []string{"/bin/cat"},
		})
	}

	type result struct {
		id  string
		err error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			id, err := w.OpenPicker()
			results <- result{id: id, err: err}
		}()
	}
	<-entered
	select {
	case <-entered:
		close(release)
		t.Fatal("both clicks started a picker")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("picker errors = %v, %v", first.err, second.err)
	}
	if first.id != second.id || len(w.tabs(w.shown())) != 1 {
		t.Fatalf("concurrent clicks returned %q and %q with tabs %+v", first.id, second.id, w.tabs(w.shown()))
	}
}

// The rail's + session button. Assembling a session is a tab like the picker,
// and a second click selects the assembly already up rather than racing a second
// one at the same manifest.
func TestTheNewSessionButtonOpensOneOnboardTab(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	opts.Onboarding = func(socket string) []string { return []string{"/bin/cat", "onboard", socket} }
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	owner := shownSession(t, reg)
	waitFor(t, "the shell tab", func() bool { return len(windows.tabs(owner)) == 1 })

	id, err := windows.OpenOnboard()
	if err != nil {
		t.Fatal(err)
	}
	tabs := windows.tabs(owner)
	if len(tabs) != 2 || tabs[1].ID != id {
		t.Fatalf("the assembly is not the newest tab: %+v", tabs)
	}
	if len(r.opened) != 0 {
		t.Fatalf("%d OS windows opened; the assembly is a tab", len(r.opened))
	}
	window, ok := windows.window(id)
	if !ok {
		t.Fatalf("the assembly %q is not registered", id)
	}
	if got := strings.Join(window.opts.Command, " "); got != "/bin/cat onboard "+opts.Socket {
		t.Fatalf("onboard command = %q, want it carrying the control socket", got)
	}
	if !window.opts.CloseOnExit {
		t.Fatal("the assembly tab outlives the assembly")
	}

	again, err := windows.OpenOnboard()
	if err != nil {
		t.Fatal(err)
	}
	if again != id || len(windows.tabs(owner)) != 2 {
		t.Fatalf("a second click opened %q rather than selecting %q", again, id)
	}
}

func TestConcurrentNewSessionClicksOpenOneOnboard(t *testing.T) {
	w, _ := testWindows(t)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	w.newOnboard = func() (string, error) {
		entered <- struct{}{}
		<-release
		return w.openStructural(w.shown(), workbench.WindowOptions{
			Kind: workbench.KindTerminal, Label: onboardWindowLabel, Source: onboardSource,
			Command: []string{"/bin/cat"},
		})
	}

	type result struct {
		id  string
		err error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			id, err := w.OpenOnboard()
			results <- result{id: id, err: err}
		}()
	}
	<-entered
	select {
	case <-entered:
		close(release)
		t.Fatal("both clicks started an assembly")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil {
		t.Fatalf("onboard errors = %v, %v", first.err, second.err)
	}
	if first.id != second.id || len(w.tabs(w.shown())) != 1 {
		t.Fatalf("concurrent clicks returned %q and %q with tabs %+v", first.id, second.id, w.tabs(w.shown()))
	}
}

// The window left when the last session's agent exits has no session to own the
// assembly, and that is exactly when the user reaches for it.
func TestOnboardOpensWithNoSessionOnScreen(t *testing.T) {
	r := newFakeRenderer()
	reg := newSessions()
	w := newWindows(r, r.Emit, false, reg)
	t.Cleanup(w.stopAll)
	root := t.TempDir()

	socket := "/tmp/qrouton-sock/501/deadbeef.sock"
	id, err := openOnboard(w, nil, func(s string) []string { return []string{"/bin/cat", s} }, socket, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.opened) != 0 {
		t.Fatalf("%d OS windows opened; the assembly is a tab", len(r.opened))
	}
	drawn := w.Surfaces("")
	if len(drawn.Tabs) != 1 || drawn.Tabs[0].ID != id {
		t.Fatalf("the session-less window sees %+v, want the assembly %q", drawn, id)
	}
	window, _ := w.window(id)
	if got := strings.Join(window.opts.Command, " "); got != "/bin/cat "+socket {
		t.Fatalf("onboard command = %q, want it carrying the control socket", got)
	}
	if window.opts.Cwd != root {
		t.Fatalf("the assembly runs in %q, want the sessions root %q", window.opts.Cwd, root)
	}
}

func TestOnboardRefusesWithoutACommandOrASocket(t *testing.T) {
	w, _ := testWindows(t)
	argv := func(string) []string { return []string{"/bin/cat"} }
	if _, err := openOnboard(w, nil, nil, "/tmp/sock", t.TempDir()); !errors.Is(err, ErrNoOnboardCommand) {
		t.Fatalf("openOnboard without a command = %v, want ErrNoOnboardCommand", err)
	}
	if _, err := openOnboard(w, nil, argv, "", t.TempDir()); !errors.Is(err, ErrNoOnboardCommand) {
		t.Fatalf("openOnboard without a socket = %v, want ErrNoOnboardCommand", err)
	}
	if _, err := w.OpenOnboard(); !errors.Is(err, ErrNoOnboardCommand) {
		t.Fatalf("OpenOnboard with nothing wired = %v, want ErrNoOnboardCommand", err)
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
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
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

func TestAnAgentReplacesTheDocumentTheUserOpened(t *testing.T) {
	w, _ := testDockedWindows(t)
	const doc = "thoughts/shared/plans/P006.md"
	old, err := w.openStructural(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ old", Source: doc, Content: "old",
	})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ fresh", Source: doc, Content: "fresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if old == fresh || w.exists(old) {
		t.Fatalf("old document %q survived replacement by %q", old, fresh)
	}
	if ids := w.list(); len(ids) != 1 || ids[0] != fresh {
		t.Fatalf("open windows = %v, want only %q", ids, fresh)
	}
	content, err := w.Content(fresh)
	if err != nil || content.Text != "fresh" {
		t.Fatalf("replacement content = %+v, %v", content, err)
	}
}

func TestOpeningAnExistingDocumentSelectsItsTab(t *testing.T) {
	w, r := testDockedWindows(t)
	const doc = "thoughts/shared/plans/P006.md"
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ P006", Source: doc, Content: "plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	delete(r.events, selectEvent)
	r.mu.Unlock()
	got, err := w.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	selected := r.events[selectEvent]
	r.mu.Unlock()
	if got != id || selected != (selection{Session: w.shown().slug(), ID: id}) {
		t.Fatalf("OpenDocument returned %q and selected %v, want %q", got, selected, id)
	}
}

// Tabs that all read "$ shell" are tabs nobody can tell apart. The number goes
// in the label the registry stores, so read_window and the manifest's record
// agree with the tab strip.
func TestTheSecondShellOnwardsIsNumbered(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	owner := shownSession(t, reg)
	waitFor(t, "the shell tab", func() bool { return len(windows.tabs(owner)) == 1 })

	for range 2 {
		if _, err := windows.OpenShell(); err != nil {
			t.Fatal(err)
		}
	}
	tabs := windows.tabs(owner)
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
	opts, _ := testOptions(t)
	opts.SessionRoot = ""
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	adopted := t.TempDir()
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	if spec := <-r.opened; spec.Title != mainWindowTitle {
		t.Fatalf("window title %q names a session nobody has chosen", spec.Title)
	}
	owner := shownSession(t, reg)
	if len(windows.tabs(owner)) != 0 {
		t.Fatal("a shell opened before onboarding chose a session")
	}

	host, err := (workbench.Handle{Socket: opts.Socket, SessionRoot: adopted}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Adopt(context.Background(), adopted, false); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the shell tab", func() bool { return len(windows.tabs(owner)) == 1 })
	window, ok := windows.window(windows.tabs(owner)[0].ID)
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
	agent := newStubBoot("/bin/cat").command
	if err := Run(Options{SessionRoot: t.TempDir(), Socket: "/tmp/x.sock"}); err != ErrNoAgentCommand {
		t.Fatalf("Run with no way to build an agent command returned %v, want ErrNoAgentCommand", err)
	}
	if err := Run(Options{Socket: "/tmp/x.sock", Agent: agent}); err != ErrNoAgentCommand {
		t.Fatalf("Run with neither a session nor an onboarding command returned %v, want ErrNoAgentCommand", err)
	}
	if err := Run(Options{SessionRoot: t.TempDir(), Agent: agent}); err != ErrNoControlSocket {
		t.Fatalf("Run with no socket returned %v, want ErrNoControlSocket", err)
	}
}

// The manifest is the chrome's only source, so an escalation shows up on the
// next poll rather than needing the window to be told.
func TestChromePushesTheManifestsModePhaseAndName(t *testing.T) {
	r := newFakeRenderer()
	root := t.TempDir()
	dir := filepath.Join(root, "octopus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(dir, session.Manifest{Slug: "octopus", Name: "Octopus", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}

	reg := testRegistry(t, dir)
	state := reg.current()
	pushChrome(reg, root, nil, nil, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	if fields.Mode == "" || fields.Phase == "" || fields.Identity != "Octopus" {
		t.Fatalf("chrome = %+v, want the session's mode, phase and name", fields)
	}
	if fields.Slug != "octopus" || fields.Terminal != state.terminal {
		t.Fatalf("chrome names session %q on terminal %q", fields.Slug, fields.Terminal)
	}
}

// The landing path: the page cannot attach to a conversation whose terminal id
// it has not been told, and it is told by the chrome.
func TestChromePushesTheTerminalWithoutAManifest(t *testing.T) {
	r := newFakeRenderer()
	root := t.TempDir()
	reg := testRegistry(t, filepath.Join(root, "unnamed"))
	state := reg.current()

	pushChrome(reg, root, nil, nil, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed for a session with no manifest: %v", r.events)
	}
	if fields.Terminal != state.terminal {
		t.Fatalf("chrome names terminal %q, want %q", fields.Terminal, state.terminal)
	}
	if fields.Mode != "" || fields.Phase != "" || fields.Identity != "" || fields.Slug != "" {
		t.Fatalf("chrome = %+v, want the manifest's fields empty", fields)
	}
	if fields.Sessions == nil || fields.Documents == nil || fields.Repos == nil {
		t.Fatalf("chrome = %+v; a nil slice marshals as null and takes the page down", fields)
	}
}

// A rail row clicked while the conversation is choosing a session would switch
// away from an onboarding nothing can return to, so it is sent no rows.
func TestTheLandingPathPublishesNoRailRows(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	opts.SessionRoot = ""
	sessionDir(t, opts.Root, "octopus")
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	landing := shownSession(t, reg)

	pushChrome(reg, opts.Root, nil, nil, r.Emit)
	fields := pushedChrome(t, r)
	if len(fields.Sessions) != 0 {
		t.Fatalf("the landing list was sent rail rows %+v", fields.Sessions)
	}
	if fields.Sessions == nil {
		t.Fatal("chrome sessions is nil; a nil slice marshals as null and takes the page down")
	}
	if fields.Terminal != landing.terminal {
		t.Fatalf("chrome names terminal %q, want the landing conversation %q", fields.Terminal, landing.terminal)
	}
}

func TestWatchChromeStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		watchChrome(ctx, testRegistry(t, t.TempDir()), t.TempDir(), func(string, any) {})
		close(done)
	}()
	<-done
}

// Adopt is how onboarding names the session it chose, so the chrome has to read
// the root on every tick rather than capturing it once.
func TestAdoptRepointsTheChromeAtTheAdoptedSession(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	adopted := sessionDir(t, opts.Root, "adopted")
	if err := session.WriteManifest(adopted, session.Manifest{Slug: "adopted", Name: "Adopted", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	reg, term, windows := testWorkbench(t, r, r.Emit)
	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	shownSession(t, reg)

	host, err := (workbench.Handle{Socket: opts.Socket, SessionRoot: opts.SessionRoot}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Adopt(context.Background(), adopted, true); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the adopted session's chrome", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		fields, ok := r.events[chromeEvent].(status.Fields)
		return ok && fields.Identity == "Adopted"
	})
}

// The page draws a row only if it has a conversation id for it, and marks the row
// on screen by matching its slug; the file scan behind the rows knows neither.
func TestChromeNamesTheTerminalOfEveryBootedSessionAndTheSlugOnScreen(t *testing.T) {
	r := newFakeRenderer()
	root := t.TempDir()
	for _, slug := range []string{"octopus", "kraken"} {
		dir := sessionDir(t, root, slug)
		if err := session.WriteManifest(dir, session.Manifest{Slug: slug, Name: slug}); err != nil {
			t.Fatal(err)
		}
	}

	reg := testRegistry(t, filepath.Join(root, "octopus"))
	pushChrome(reg, root, nil, nil, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	if len(fields.Sessions) != 2 {
		t.Fatalf("the rail lists %+v, want both sessions under the root", fields.Sessions)
	}
	matched := 0
	for _, row := range fields.Sessions {
		if booted := row.Slug == "octopus"; booted != (row.Terminal != "") {
			t.Fatalf("row %+v carries a terminal id the workbench cannot attach to", row)
		}
		if row.Slug == fields.Slug {
			matched++
		}
	}
	if fields.Slug != "octopus" || matched != 1 {
		t.Fatalf("%d of %+v match the slug on screen %q; the page marks the one that does",
			matched, fields.Sessions, fields.Slug)
	}
}

// Attention is a session's claim about itself and arrives on its own listener, so
// it marks that session's row whether or not it is the one on screen.
func TestAttentionOnABackgroundSessionsListenerMarksOnlyItsRow(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, r := testSessions(t, root, boot)
	for _, slug := range []string{"background", "onscreen"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
	}

	dir := filepath.Join(root, "background")
	handle := workbench.Handle{Socket: boot.socket(t, dir), SessionRoot: dir}
	if err := handle.Attention(context.Background(), status.ActivityWaiting); err != nil {
		t.Fatal(err)
	}
	pushChrome(reg, root, nil, nil, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	if fields.Slug != "onscreen" {
		t.Fatalf("chrome shows %q, want the session revealed last", fields.Slug)
	}
	rows := map[string]status.SessionRow{}
	for _, row := range fields.Sessions {
		rows[row.Slug] = row
	}
	if got := rows["background"].Activity; got != status.ActivityWaiting {
		t.Fatalf("the background row reads %q, want waiting", got)
	}
	if got := rows["onscreen"].Activity; got == status.ActivityWaiting {
		t.Fatalf("the row on screen reads %q; attention reached a session it was not sent to", got)
	}
}

// Repo stats cost two subprocesses per repository, so they are the session on
// screen's alone. A background session's row carries what its manifest names.
func TestOnlyTheSessionOnScreenCarriesMeasuredRepos(t *testing.T) {
	r := newFakeRenderer()
	root := t.TempDir()
	shown := sessionDir(t, root, "octopus")
	background := sessionDir(t, root, "kraken")
	if err := session.WriteManifest(background, session.Manifest{Slug: "kraken", Mode: session.ModeAssistant,
		Repos: []session.ManifestRepo{{Name: "api", Org: "lifesum"}}}); err != nil {
		t.Fatal(err)
	}

	measured := status.RepoStat{Name: "lifesum/svc", Role: "editing", Commits: 3, Measured: true}
	pushChrome(testRegistry(t, shown), root, map[string][]status.RepoStat{shown: {measured}}, nil, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	if len(fields.Repos) != 1 || fields.Repos[0] != measured {
		t.Fatalf("chrome repos = %+v, want the session on screen's alone", fields.Repos)
	}
	for _, row := range fields.Sessions {
		if row.Slug == "kraken" && (len(row.Repos) != 1 || row.Repos[0].Name != "api") {
			t.Fatalf("the background row = %+v, want the repository its manifest names", row)
		}
	}
}

// pushedChrome is the chrome the window was last sent.
func pushedChrome(t *testing.T, r *fakeRenderer) status.Fields {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	return fields
}

// A measurement belongs to the session it was taken for, so the session switched
// to is drawn with no repository numbers rather than the last one's.
func TestSwitchingSessionsDropsThePreviousSessionsRepos(t *testing.T) {
	r := newFakeRenderer()
	root := t.TempDir()
	first := sessionDir(t, root, "octopus")
	second := sessionDir(t, root, "kraken")
	measured := map[string][]status.RepoStat{
		first: {{Name: "lifesum/svc", Role: "editing", Commits: 3, Measured: true}},
	}

	reg := testRegistry(t, first)
	pushChrome(reg, root, measured, nil, r.Emit)
	if repos := pushedChrome(t, r).Repos; len(repos) != 1 || repos[0].Name != "lifesum/svc" {
		t.Fatalf("chrome repos = %+v, want what was measured for the session on screen", repos)
	}

	reg.reveal(reg.add(second, []string{"/bin/cat"}, os.Environ()))
	pushChrome(reg, root, measured, nil, r.Emit)
	if repos := pushedChrome(t, r).Repos; len(repos) != 0 {
		t.Fatalf("chrome repos = %+v after the switch, want none of the previous session's", repos)
	}
}

func TestTheUnseenCountIsRecountedOnlyOnTheSlowTicker(t *testing.T) {
	root := t.TempDir()
	reg := testRegistry(t, sessionDir(t, root, "octopus"))
	var pushes, counts atomic.Int64
	emit := func(event string, _ any) {
		if event == chromeEvent {
			pushes.Add(1)
		}
	}
	count := func(string) map[string]int {
		counts.Add(1)
		return map[string]int{}
	}

	// The registry's own reveal already woke the poller; the switch below is the
	// wake this measures.
	<-reg.touched
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, reg, root, emit, time.Millisecond, time.Hour, count)

	waitFor(t, "the fast ticker to push repeatedly", func() bool { return pushes.Load() > 20 })
	if got := counts.Load(); got != 1 {
		t.Fatalf("the unseen count ran %d times over %d pushes, want the one before the loop",
			got, pushes.Load())
	}

	reg.reveal(reg.add(sessionDir(t, root, "kraken"), []string{"/bin/cat"}, os.Environ()))
	waitFor(t, "the switch to reach the poller", func() bool { return len(reg.touched) == 0 })
	settled := pushes.Load()
	waitFor(t, "the pushes after the switch", func() bool { return pushes.Load() > settled+5 })
	if got := counts.Load(); got != 1 {
		t.Fatalf("switching sessions recounted unseen documents, %d counts in all", got)
	}
}

// Opening the app comes back to the session you were last in, which only works
// if putting one on screen stamps it and leaves every other stamp where it was.
func TestShowingASessionStampsOnlyThatOne(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	shownSession(t, reg)

	first, ok := session.LastOpened(opts.SessionRoot)
	if !ok {
		t.Fatal("the session the workbench opened on was never stamped")
	}

	kraken := sessionDir(t, opts.Root, "kraken")
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	second, ok := session.LastOpened(kraken)
	if !ok {
		t.Fatal("a session the rail showed was never stamped")
	}
	if !second.After(first) {
		t.Fatalf("kraken was stamped %v, want later than the session before it at %v", second, first)
	}
	if again, _ := session.LastOpened(opts.SessionRoot); !again.Equal(first) {
		t.Fatalf("showing kraken moved the first session's stamp to %v, want %v", again, first)
	}
}

// The rail's click. A session this workbench has not run yet boots; one it has is
// revealed, and booting it twice is a second agent nobody asked for.
func TestShowBootsASessionOnceAndRevealsItAfterwards(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, r := testSessions(t, root, boot)
	sessionDir(t, root, "octopus")

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	booted := reg.current()
	if booted == nil {
		t.Fatal("Show left the window with no session on it")
	}
	if booted.control == nil {
		t.Fatal("the booted session has no listener of its own")
	}
	term := newTerm(reg, r.Emit)
	if err := term.Start(booted.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	if booted.process == nil {
		t.Fatal("the booted session has no conversation process")
	}
	if agents, serves := boot.counts(); agents != 1 || serves != 1 {
		t.Fatalf("booting one session asked for %d supervisors and %d listeners", agents, serves)
	}

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	if again := reg.current(); again != booted {
		t.Fatalf("Show came back with terminal %q, want the booted session's %q", again.terminal, booted.terminal)
	}
	if agents, serves := boot.counts(); agents != 1 || serves != 1 {
		t.Fatalf("showing a booted session started %d supervisors and %d listeners", agents, serves)
	}
}

// Show asks whether a session is up and then boots it, and a doubled rail click
// reaching that gap would put two supervisors on one session directory.
func TestConcurrentShowsOfOneSessionBootItOnce(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	sessionDir(t, root, "octopus")

	command := reg.boot.agent
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	reg.boot.agent = func(sessionRoot, socket string, resume bool) ([]string, []string, error) {
		entered <- struct{}{}
		<-release
		return command(sessionRoot, socket, resume)
	}

	shows := make(chan error, 2)
	for range 2 {
		go func() { shows <- reg.Show("octopus") }()
	}
	<-entered
	select {
	case <-entered:
		close(release)
		t.Fatal("both clicks booted the session")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	for range 2 {
		if err := <-shows; err != nil {
			t.Fatal(err)
		}
	}
	if agents, serves := boot.counts(); agents != 1 || serves != 1 {
		t.Fatalf("two clicks asked for %d supervisors and %d listeners", agents, serves)
	}
	if state := reg.current(); state == nil || state.slug() != "octopus" {
		t.Fatalf("the session on screen is %v, want the one that booted", state)
	}
	if all := reg.all(); len(all) != 1 {
		t.Fatalf("%d sessions registered under one slug", len(all))
	}
}

func TestShowRefusesASlugItCannotResolve(t *testing.T) {
	reg, _, _ := testSessions(t, "", newStubBoot("/bin/cat"))
	err := reg.Show("octopus")
	if err == nil {
		t.Fatal("Show booted a session with no directory behind it")
	}
	if !strings.Contains(err.Error(), "octopus") {
		t.Fatalf("refusal %q does not name the session asked for", err)
	}
}

// Two sessions are two agents on two addresses. Sharing either is two rail rows
// drawing one conversation.
func TestTwoSessionsGetTheirOwnSocketAndSupervisor(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, r := testSessions(t, root, boot)
	term := newTerm(reg, r.Emit)

	booted := map[string]*sessionState{}
	for _, slug := range []string{"alpha", "beta"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
		state := reg.current()
		if err := term.Start(state.terminal, 80, 24); err != nil {
			t.Fatal(err)
		}
		booted[slug] = state
	}

	alpha, beta := booted["alpha"], booted["beta"]
	if alpha.terminal == beta.terminal {
		t.Fatalf("both sessions were minted %q", alpha.terminal)
	}
	if alpha.process.cmd.Process.Pid == beta.process.cmd.Process.Pid {
		t.Fatal("two sessions share one supervisor")
	}
	if first, second := boot.socket(t, alpha.root()), boot.socket(t, beta.root()); first == second {
		t.Fatalf("both sessions were served on %q", first)
	}
}

// A handler bound to a session's listener cannot address another session, so no
// agent can reach into the window of a session that is not its own.
func TestAnOpOnOneSessionsListenerTouchesOnlyThatSessionsWindows(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, windows, r := testSessions(t, root, boot)
	for _, slug := range []string{"alpha", "beta"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
	}

	alpha := filepath.Join(root, "alpha")
	host, err := (workbench.Handle{Socket: boot.socket(t, alpha), SessionRoot: alpha}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	id, err := host.Open(context.Background(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ alpha dev", Cwd: alpha, Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened

	if drawn := windows.Surfaces("alpha"); len(drawn.Floating) != 1 || drawn.Floating[0].ID != id {
		t.Fatalf("alpha sees %+v, want the window opened on its own socket", drawn)
	}
	if drawn := windows.Surfaces("beta"); len(drawn.Floating) != 0 || len(drawn.Tabs) != 0 {
		t.Fatalf("beta sees %+v, want none of alpha's windows", drawn)
	}
}

// A workbench that opened on a session leaves its process socket owning none, so
// all it answers is the launcher's readiness poll and an onboarding child's
// adopt. A window op is refused in an answer rather than by a closed door.
func TestTheProcessSocketAnswersReadinessAndAdoptAndRefusesAWindowOp(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	adopted := sessionDir(t, opts.Root, "kraken")
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	shownSession(t, reg)

	if !workbench.Answered(opts.Socket) {
		t.Fatal("the process socket does not answer the launcher's readiness poll")
	}
	host, err := (workbench.Handle{Socket: opts.Socket, SessionRoot: opts.SessionRoot}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: opts.SessionRoot, Command: []string{"/bin/cat"},
	}); err == nil {
		t.Fatal("the process socket opened a window with no session to own it")
	} else if errors.Is(err, workbench.ErrWorkbenchUnreachable) {
		t.Fatalf("the refusal arrived as a transport failure: %v", err)
	} else if err.Error() != ErrNoSession.Error() {
		t.Fatalf("refusal = %q, want %q", err, ErrNoSession)
	}

	if err := host.Adopt(ctx, adopted, true); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the adopted session on screen", func() bool {
		state := reg.current()
		return state != nil && state.slug() == "kraken"
	})
}

// Refusing a dead agent.pid would strand the session until somebody deleted the
// file by hand. A live one is a supervisor that outlived its workbench, which is
// an error rather than a state.
func TestADeadAgentPIDBootsAndALiveOneRefuses(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)

	writeAgentPID(t, sessionDir(t, root, "crashed"), deadPID(t))
	if err := reg.Show("crashed"); err != nil {
		t.Fatalf("a session whose recorded agent is gone refused to boot: %v", err)
	}
	if state := reg.current(); state == nil || state.slug() != "crashed" {
		t.Fatalf("the session on screen is %v, want the one that booted", state)
	}

	writeAgentPID(t, sessionDir(t, root, "live"), os.Getpid())
	err := reg.Show("live")
	if err == nil {
		t.Fatal("a session whose agent is still running booted a second one over it")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(os.Getpid())) {
		t.Fatalf("refusal %q does not name the pid holding the session", err)
	}
	if agents, serves := boot.counts(); agents != 1 || serves != 1 {
		t.Fatalf("the refused session got %d supervisors and %d listeners of the two built", agents, serves)
	}
}

// A rail row can outlive the directory it names, and booting one anyway mints a
// socket and registers a state for a session that is not there.
func TestShowRefusesARowWhoseSessionIsGone(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	shownSession(t, reg)

	ghost := filepath.Join(opts.Root, "ghost")
	if err := os.MkdirAll(ghost, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("ghost"); err == nil {
		t.Fatal("a directory with no manifest booted; a session without one must not appear resumable")
	}
	if reg.bySlug("ghost") != nil {
		t.Fatal("the refused session was registered anyway")
	}

	if err := session.WriteManifest(ghost, session.Manifest{Slug: "ghost", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("ghost"); err != nil {
		t.Fatalf("a session with a manifest refused to boot: %v", err)
	}
	if boot.socket(t, ghost) == "" {
		t.Fatal("the booted session got no control socket of its own")
	}
}

// The page drives Show and the toolkit drives OnClose, so a session coming up and
// the app going down reach its listener on two goroutines.
func TestBootingASessionRacesTeardownSafely(t *testing.T) {
	root := t.TempDir()
	sessionDir(t, root, "octopus")
	r := newFakeRenderer()

	for range 40 {
		reg := newSessions()
		windows := newWindows(r, r.Emit, false, reg)
		release := make(chan struct{})
		serving := make(chan struct{})
		var reserved string
		reg.boot = booting{
			root:  func(slug string) string { return filepath.Join(root, slug) },
			agent: func(string, string, bool) ([]string, []string, error) { return []string{"/bin/cat"}, os.Environ(), nil },
			serve: func(state *sessionState, socket string) (io.Closer, error) {
				reserved = socket
				close(serving)
				<-release
				return serveControl(socket, windows, state, controlHooks{})
			},
			teardown: windows.stop,
		}

		booted := make(chan error, 1)
		go func() { booted <- reg.Show("octopus") }()
		<-serving
		torn := make(chan struct{})
		go func() {
			reg.stopAll()
			close(torn)
		}()
		close(release)
		<-torn
		if err := <-booted; err != nil && !errors.Is(err, ErrNoSession) {
			t.Fatalf("boot raced by teardown = %v, want it booted or refused as ErrNoSession", err)
		}
		reg.stopAll()
		windows.stopAll()
		if workbench.Answered(reserved) {
			t.Fatalf("the torn-down session still answers on %q", reserved)
		}
	}
}

// A session assembled in a pane has no conversation to continue, and a runner
// asked to resume one skips the opening message that orients the orchestrator.
func TestAdoptingAFreshlyAssembledSessionDoesNotResumeIt(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	shownSession(t, reg)

	assembled := sessionDir(t, opts.Root, "kraken")
	host, err := (workbench.Handle{Socket: opts.Socket, SessionRoot: assembled}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Adopt(context.Background(), assembled, true); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the adopted session on screen", func() bool {
		state := reg.current()
		return state != nil && state.slug() == "kraken"
	})
	if socket := boot.socket(t, assembled); socket == "" {
		t.Fatal("the assembled session was never given a supervisor, so nothing was asked to resume or not")
	}
	if boot.resumed(t, assembled) {
		t.Fatal("the assembled session was booted with resume; its runner would continue a conversation that never happened, and start without its opening message")
	}
}

// Onboarding in a pane has no terminal to hand over, so the session it named gets
// an agent of its own, and the landing list it answered goes with its terminal.
func TestAPaneAdoptBootsTheNewSessionAndRetiresTheLanding(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	opts.SessionRoot = ""
	reg, term, windows := testWorkbench(t, r, r.Emit)

	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	landing := shownSession(t, reg)
	if err := term.Start(landing.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	landingPID := landing.process.cmd.Process.Pid

	assembled := sessionDir(t, opts.Root, "kraken")
	host, err := (workbench.Handle{Socket: opts.Socket, SessionRoot: assembled}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	if err := host.Adopt(context.Background(), assembled, true); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the assembled session on screen", func() bool {
		state := reg.current()
		return state != nil && state.slug() == "kraken"
	})
	booted := reg.current()
	if booted == landing {
		t.Fatal("the landing session was renamed rather than the assembled one booted, so it never got an agent")
	}
	if socket := boot.socket(t, assembled); socket == "" {
		t.Fatal("the assembled session got no control socket of its own")
	}
	if err := term.Start(booted.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	if booted.process.cmd.Process.Pid == landingPID {
		t.Fatal("the assembled session runs in the landing session's process")
	}
	waitFor(t, "the landing session to be unregistered", func() bool {
		_, live := reg.byTerminal(landing.terminal)
		return !live && reg.landing() == nil
	})
	waitFor(t, "the landing conversation to end", func() bool { return syscall.Kill(landingPID, 0) != nil })
}

// A handover adopt is onboarding about to exec the supervisor in the terminal it
// drew in, so booting one here would be a second agent beside it.
func TestAHandoverAdoptWithoutALandingSessionRefuses(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	sessionDir(t, root, "octopus")
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}

	if err := reg.adopt(sessionDir(t, root, "kraken"), false); !errors.Is(err, ErrNoLandingSession) {
		t.Fatalf("a handover adopt with nothing to hand over = %v, want ErrNoLandingSession", err)
	}
	if agents, serves := boot.counts(); agents != 1 || serves != 1 {
		t.Fatalf("the refused adopt left %d supervisors and %d listeners", agents, serves)
	}
	if reg.bySlug("kraken") != nil {
		t.Fatal("the refused session was registered anyway")
	}
	if state := reg.current(); state == nil || state.slug() != "octopus" {
		t.Fatalf("the session on screen is %v, want the one already up", state)
	}
}

// railSlugs is the order the rail would draw a poll's rows in.
func railSlugs(reg *Sessions, rows []status.SessionRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range reg.railOrder(rows) {
		out = append(out, row.Slug)
	}
	return out
}

// polled is what status.Sessions returns: most recently opened first.
func polled(slugs ...string) []status.SessionRow {
	rows := make([]status.SessionRow, 0, len(slugs))
	for _, slug := range slugs {
		rows = append(rows, status.SessionRow{Slug: slug, Name: slug})
	}
	return rows
}

// A row's position is how the keyboard addresses it, so re-sorting a poll would
// rename every shortcut under the user's fingers. Showing a session stamps it,
// which is exactly what would move it.
func TestTheRailKeepsTheOrderItWasFirstDrawnIn(t *testing.T) {
	reg := newSessions()
	launch := polled("kraken", "octopus", "webhook")
	if got := railSlugs(reg, launch); !reflect.DeepEqual(got, []string{"kraken", "octopus", "webhook"}) {
		t.Fatalf("the rail's first draw = %v, want the poll's own order", got)
	}
	// A later poll with octopus now the most recently opened.
	for _, rows := range [][]status.SessionRow{
		polled("octopus", "kraken", "webhook"),
		polled("webhook", "octopus", "kraken"),
	} {
		if got := railSlugs(reg, rows); !reflect.DeepEqual(got, []string{"kraken", "octopus", "webhook"}) {
			t.Fatalf("a poll reordered the rail to %v", got)
		}
	}
}

// Creating a session is a deliberate act and it is the most recent thing there
// is, so it takes the first row and the numbers below it shift.
func TestASessionAssembledMidLifetimeTakesTheFirstRow(t *testing.T) {
	reg := newSessions()
	railSlugs(reg, polled("kraken", "octopus"))

	if got := railSlugs(reg, polled("fresh", "kraken", "octopus")); !reflect.DeepEqual(got, []string{"fresh", "kraken", "octopus"}) {
		t.Fatalf("a new session landed at %v, want the front", got)
	}
	// Two at once keep the order they were polled in rather than reversing.
	if got := railSlugs(reg, polled("newest", "second", "fresh", "kraken", "octopus")); !reflect.DeepEqual(got, []string{"newest", "second", "fresh", "kraken", "octopus"}) {
		t.Fatalf("a batch of new sessions landed as %v", got)
	}
}

// Nothing in the app deletes a session, so this only follows an rm -rf. Keeping
// the slug's place means it comes back where it was rather than at the front.
func TestASessionGoneFromDiskKeepsItsPlaceForWhenItReturns(t *testing.T) {
	reg := newSessions()
	railSlugs(reg, polled("kraken", "octopus", "webhook"))

	if got := railSlugs(reg, polled("kraken", "webhook")); !reflect.DeepEqual(got, []string{"kraken", "webhook"}) {
		t.Fatalf("rail without octopus = %v", got)
	}
	if got := railSlugs(reg, polled("octopus", "kraken", "webhook")); !reflect.DeepEqual(got, []string{"kraken", "octopus", "webhook"}) {
		t.Fatalf("octopus came back at %v, not the place it held", got)
	}
}

// The rail draws selection from the session on screen, so the top row is only
// selected while it is the one being shown.
func TestTheShownSessionIsNamedIndependentlyOfRailPosition(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	sessionDir(t, opts.Root, "kraken")
	go func() { _ = run(r, term, windows, opts) }()
	<-r.opened
	shownSession(t, reg)

	// The rail freezes on the first poll, with the session the workbench opened on
	// at the top because showing it stamped it.
	pushChrome(reg, opts.Root, nil, nil, r.Emit)
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	pushChrome(reg, opts.Root, nil, nil, r.Emit)
	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed: %v", r.events)
	}
	if fields.Slug != "kraken" {
		t.Fatalf("chrome names %q as shown, want the session Show revealed", fields.Slug)
	}
	if len(fields.Sessions) < 2 {
		t.Fatalf("rail has %d rows, want both sessions", len(fields.Sessions))
	}
	if fields.Sessions[0].Slug == fields.Slug {
		t.Fatal("switching moved the session to the top row; the rail's order is not frozen")
	}
}
