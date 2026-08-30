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

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// fakeRenderer stands in for the toolkit, so the window lifecycle runs without
// a display.
type fakeRenderer struct {
	mu      sync.Mutex
	opened  chan windowSpec
	titles  map[string]string
	events  map[string]any
	focused map[string]int
	quit    bool
	block   chan struct{}
	once    sync.Once
}

func newFakeRenderer() *fakeRenderer {
	return &fakeRenderer{
		opened:  make(chan windowSpec, 8),
		titles:  map[string]string{},
		events:  map[string]any{},
		focused: map[string]int{},
		block:   make(chan struct{}),
	}
}

func (f *fakeRenderer) Retitle(name, title string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.titles[name] = title
}

func (f *fakeRenderer) Open(spec windowSpec) error {
	f.opened <- spec
	return nil
}

func (f *fakeRenderer) Focus(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.focused[name]++
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

// Quit tolerates being called twice: the workbench quits itself when its
// conversation window closes, and the test stops it again on the way out.
func (f *fakeRenderer) Quit() {
	f.once.Do(func() {
		f.mu.Lock()
		f.quit = true
		f.mu.Unlock()
		close(f.block)
	})
}

// stubBoot stands in for what a session needs to come up, counting the
// supervisor commands and the listeners asked for and keeping each address. The
// window commands are unset until a test sets one, which is the workbench that
// cannot do that thing.
type stubBoot struct {
	argv     []string
	shell    func(sessionRoot string) []string
	reveal   func(sessionRoot string) []string
	document func(sessionRoot, name string) (workbench.WindowOptions, error)

	mu      sync.Mutex
	sockets map[string]string
	resumes map[string]bool
	runners map[string]string
	agents  int
	serves  int
}

func newStubBoot(argv ...string) *stubBoot {
	return &stubBoot{argv: argv, sockets: map[string]string{}, resumes: map[string]bool{},
		runners: map[string]string{}}
}

func (b *stubBoot) Agent(req AgentRequest) (AgentCommand, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.agents++
	b.sockets[req.SessionRoot] = req.Socket
	b.resumes[req.SessionRoot] = req.Resume
	b.runners[req.SessionRoot] = req.RunnerID
	resolved := req.RunnerID
	if resolved == "" {
		resolved = "codex"
	}
	return AgentCommand{Argv: b.argv, Env: os.Environ(), RunnerID: resolved}, nil
}

func (b *stubBoot) Shell(sessionRoot string) []string {
	if b.shell == nil {
		return nil
	}
	return b.shell(sessionRoot)
}

func (b *stubBoot) Reveal(sessionRoot string) []string {
	if b.reveal == nil {
		return nil
	}
	return b.reveal(sessionRoot)
}

func (b *stubBoot) Document(sessionRoot, name string) (workbench.WindowOptions, error) {
	if b.document == nil {
		return workbench.WindowOptions{}, ErrNoEditorCommand
	}
	return b.document(sessionRoot, name)
}

func (b *stubBoot) Runners() ([]assembly.Runner, error) { return nil, nil }

func (b *stubBoot) Signal(string) {}

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
		Env:         os.Environ(),
		Launcher:    boot,
	}, boot
}

// testWorkbench wires a conversation and a window registry over one session
// registry, as Run does; run adds the sessions to it.
func testWorkbench(t *testing.T, r *fakeRenderer, emit emitter) (*Sessions, *Term, *Windows) {
	t.Helper()
	reg := newSessions()
	windows := newWindows(r.Emit, reg)
	t.Cleanup(reg.stopAll)
	t.Cleanup(windows.stopAll)
	return reg, newTerm(reg, emit), windows
}

// startWorkbench runs the workbench and stops it before the temporary root goes:
// a session reaching the screen does not order against the stamping and
// recording that follows it, which would otherwise land in a directory the
// test's cleanup is already removing. The channel carries run's result.
func startWorkbench(t *testing.T, r *fakeRenderer, term *Term, windows *Windows, opts Options) <-chan error {
	t.Helper()
	reg := term.sessions
	// The same teardown Run builds for Settings.Quit and the window's own
	// OnClose, so a test closing the conversation window exercises the real
	// shutdown rather than a bare Quit.
	quit := sync.OnceFunc(func() {
		windows.stopAll()
		reg.stopAll()
		r.Quit()
	})
	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		done <- run(r, term, windows, opts, quit)
	}()
	t.Cleanup(func() {
		r.Quit()
		<-stopped
	})
	return done
}

// testSessions is a registry that boots sessions as run does: a supervisor
// command per session and a listener of its own, over one window registry.
func testSessions(t *testing.T, root string, boot *stubBoot) (*Sessions, *Windows, *fakeRenderer) {
	t.Helper()
	r := newFakeRenderer()
	reg := newSessions()
	windows := newWindows(r.Emit, reg)
	t.Cleanup(reg.stopAll)
	t.Cleanup(windows.stopAll)
	reg.boot = booting{
		root:  func(slug string) string { return session.Resumable(root, slug) },
		agent: boot.Agent,
		serve: func(state *sessionState, socket string) (io.Closer, error) {
			boot.served()
			return serveControl(socket, windows, state, controlHooks{attention: func(value string, _ uint64) { state.activity.hook(value) }})
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

	startWorkbench(t, r, term, windows, opts)

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

	done := startWorkbench(t, r, term, windows, opts)

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

	startWorkbench(t, r, term, windows, opts)
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

	done := startWorkbench(t, r, term, windows, opts)
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

// A supervisor that failed has something to say, and quitting would take it off
// the screen before anyone read it.
func TestAFailedAgentLeavesItsWindowOpen(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	boot.argv = []string{"/bin/sh", "-c", "exit 3"}
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	state := shownSession(t, reg)
	conversation := state.terminal
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
	waitFor(t, "the pre-generation root failure", func() bool {
		view := state.agents.snapshot()
		return len(view.Records) == 1 && view.Records[0].RunID == agentSetupRunPrefix+"1" &&
			view.Records[0].Provider == "codex" && view.Records[0].State == agentStateFailed
	})
	handle := workbench.Handle{Socket: boot.socket(t, state.root()), SessionRoot: state.root()}
	if err := handle.RunnerGeneration(context.Background(), "codex", 42); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the announced run beside the retained setup failure", func() bool {
		view := state.agents.snapshot()
		return len(view.Records) == 2 && view.Running
	})
	r.Quit()
}

// Decision 10: a shell is always available, opened by the workbench rather than
// asked for. Asking gets you another; adoption running twice does not.
func TestTheWorkbenchOpensOneUserShellAlongsideTheConversation(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	boot.shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	owner := shownSession(t, reg)

	waitFor(t, "the shell tab", func() bool { return len(windows.surfaces(owner).Tabs) == 1 })
	tab := windows.surfaces(owner).Tabs[0]
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
	if !window.opts.CloseOnExit {
		t.Fatal("a shell that exits cleanly leaves its tab behind")
	}
	if len(r.opened) != 0 {
		t.Fatalf("%d OS windows opened; the shell is a tab", len(r.opened))
	}

	if err := windows.Close(tab.ID); err != nil {
		t.Fatal(err)
	}
	if len(windows.surfaces(owner).Tabs) != 0 {
		t.Fatal("the shell tab survived being closed")
	}
	for want := 1; want <= 2; want++ {
		id, err := windows.OpenShell()
		if err != nil {
			t.Fatal(err)
		}
		tabs := windows.surfaces(owner).Tabs
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
	opts, boot := testOptions(t)
	boot.shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	boot.document = func(root, name string) (workbench.WindowOptions, error) {
		return workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "◆ " + filepath.Base(name),
			Source: name, Content: "# " + name, Format: workbench.FormatMarkdown,
		}, nil
	}
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	owner := shownSession(t, reg)
	waitFor(t, "the shell tab", func() bool { return len(windows.surfaces(owner).Tabs) == 1 })

	const doc = "thoughts/shared/research/R7-editor-surfaces.md"
	id, err := windows.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	tabs := windows.surfaces(owner).Tabs
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
	if selected := windows.Surfaces(owner.slug()).Selected; selected != id {
		t.Fatalf("the document chip selected %q, want %q", selected, id)
	}

	again, err := windows.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if again != id {
		t.Fatalf("a second click opened %q rather than selecting %q", again, id)
	}
	if got := len(windows.surfaces(owner).Tabs); got != 2 {
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
	w, _ := testWindows(t)
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
	w, _ := testWindows(t)
	const doc = "thoughts/shared/plans/P006.md"
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ P006", Source: doc, Content: "plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := w.openStructural(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "other", Content: "other",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Select(w.shown().slug(), other); err != nil {
		t.Fatal(err)
	}
	got, err := w.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	selected := w.Surfaces(w.shown().slug()).Selected
	if got != id || selected != id {
		t.Fatalf("OpenDocument returned %q and selected %q, want %q", got, selected, id)
	}
}

// Tabs that all read "$ shell" are tabs nobody can tell apart. The number goes
// in the label the registry stores, so read_window and the manifest's record
// agree with the tab strip.
func TestTheSecondShellOnwardsIsNumbered(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	boot.shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	owner := shownSession(t, reg)
	waitFor(t, "the shell tab", func() bool { return len(windows.surfaces(owner).Tabs) == 1 })

	for range 2 {
		if _, err := windows.OpenShell(); err != nil {
			t.Fatal(err)
		}
	}
	tabs := windows.surfaces(owner).Tabs
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

func TestRunRefusesASessionWithNothingToRun(t *testing.T) {
	launcher := newStubBoot("/bin/cat")
	if err := Run(Options{SessionRoot: t.TempDir(), Socket: "/tmp/x.sock"}); err != ErrNoAgentCommand {
		t.Fatalf("Run with no way to build an agent command returned %v, want ErrNoAgentCommand", err)
	}
	if err := Run(Options{SessionRoot: t.TempDir(), Launcher: launcher}); err != ErrNoControlSocket {
		t.Fatalf("Run with no socket returned %v, want ErrNoControlSocket", err)
	}
	if err := Run(Options{SessionRoot: t.TempDir(), Socket: "/tmp/x.sock", Launcher: launcher}); err != ErrNoConfig {
		t.Fatalf("Run with nothing to assemble against returned %v, want ErrNoConfig", err)
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
	pushChrome(reg, root, nil, nil, nil, r.Emit)

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

// welcomingFor pushes one payload and reports whether the window is asking the
// first-run questions.
func welcomingFor(t *testing.T, reg *Sessions, root string, cfg *config.Config) bool {
	t.Helper()
	r := newFakeRenderer()
	pushChrome(reg, root, cfg, nil, nil, r.Emit)
	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	return fields.Welcoming
}

// Asking over a live conversation would put two answers between the user and the
// agent they were talking to, and answering with a changed root would take that
// session's workbench down with it.
func TestChromeAsksTheFirstRunQuestionsOnlyOnAWindowWithNoSession(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, "octopus")
	if err := session.WriteManifest(dir, session.Manifest{Slug: "octopus", Name: "Octopus"}); err != nil {
		t.Fatal(err)
	}

	if !welcomingFor(t, newSessions(), root, &config.Config{Root: root}) {
		t.Fatal("a window with no session and no marker does not ask")
	}
	for _, marked := range []bool{false, true} {
		cfg := &config.Config{Root: root, Welcomed: marked}
		if welcomingFor(t, testRegistry(t, dir), root, cfg) {
			t.Fatalf("a window holding a session asks anyway (welcomed %v)", marked)
		}
	}
}

// The push runs every couple of seconds off the live config, so a marker set
// while the window is up drops the gate on the next tick rather than at the next
// launch.
func TestChromeReadsTheWelcomedMarkerOffTheLiveConfig(t *testing.T) {
	root := t.TempDir()
	reg := newSessions()
	cfg := &config.Config{Root: root}

	if !welcomingFor(t, reg, root, cfg) {
		t.Fatal("a config with no marker does not raise first run")
	}
	cfg.Welcomed = true
	if welcomingFor(t, reg, root, cfg) {
		t.Fatal("first run stays raised after the live config was marked")
	}
}

func TestWatchChromeStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		watchChrome(ctx, testRegistry(t, t.TempDir()), t.TempDir(), nil, func(string, any) {})
		close(done)
	}()
	<-done
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
	pushChrome(reg, root, nil, nil, nil, r.Emit)

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
	if err := handle.Attention(context.Background(), status.ActivityWaiting, 1); err != nil {
		t.Fatal(err)
	}
	pushChrome(reg, root, nil, nil, nil, r.Emit)

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
	pushChrome(testRegistry(t, shown), root, nil, map[string][]status.RepoStat{shown: {measured}}, nil, r.Emit)

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
		if row.Slug == "kraken" && (len(row.Repos) != 1 || row.Repos[0].Name != "lifesum/api") {
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

func TestChromeSnapshotsTheInitialStateAndSuppressesUnchangedUpdates(t *testing.T) {
	var snapshots []status.Fields
	var chrome *Chrome
	chrome = newChrome(func(event string, _ any) {
		if event == chromeEvent {
			snapshots = append(snapshots, chrome.Snapshot())
		}
	})

	empty := chrome.Snapshot()
	if empty.Sessions == nil || empty.Documents == nil || empty.RepositoryDocuments == nil || empty.Repos == nil || empty.Agents.Agents == nil {
		t.Fatalf("initial snapshot has nil slices: %+v", empty)
	}
	first := status.Fields{
		Activity: "idle", Sessions: []status.SessionRow{},
		Documents: []status.Document{}, RepositoryDocuments: []status.RepositoryDocuments{}, Repos: []status.RepoStat{},
	}
	chrome.publish(chromeEvent, first)
	chrome.publish(chromeEvent, first)
	if len(snapshots) != 1 || !reflect.DeepEqual(snapshots[0], first) {
		t.Fatalf("initial publishes = %+v, want one stored-before-emit snapshot", snapshots)
	}

	changed := first
	changed.Activity = "working"
	chrome.publish(chromeEvent, changed)
	if len(snapshots) != 2 || !reflect.DeepEqual(chrome.Snapshot(), changed) {
		t.Fatalf("changed publishes = %+v, snapshot = %+v", snapshots, chrome.Snapshot())
	}
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
	pushChrome(reg, root, nil, measured, nil, r.Emit)
	if repos := pushedChrome(t, r).Repos; len(repos) != 1 || repos[0].Name != "lifesum/svc" {
		t.Fatalf("chrome repos = %+v, want what was measured for the session on screen", repos)
	}

	reg.reveal(reg.add(second, []string{"/bin/cat"}, os.Environ()))
	pushChrome(reg, root, nil, measured, nil, r.Emit)
	if repos := pushedChrome(t, r).Repos; len(repos) != 0 {
		t.Fatalf("chrome repos = %+v after the switch, want none of the previous session's", repos)
	}
}

// Every session's documents are ×N directory reads, so that count rides the slow
// ticker. Arriving at one session is a single read on a deliberate act, so it
// happens at once: a marker still lit after you have looked is one nobody reads.
func TestUnseenIsRecountedForTheArrivedSessionAndOtherwiseOnlySlowly(t *testing.T) {
	root := t.TempDir()
	reg := testRegistry(t, sessionDir(t, root, "octopus"))
	var pushes, all, arrivals atomic.Int64
	emit := func(event string, _ any) {
		if event == chromeEvent {
			pushes.Add(1)
		}
	}
	counts := unseenCounts{
		all: func(string) map[string]int {
			all.Add(1)
			return map[string]int{}
		},
		in: func(string) (int, bool) {
			arrivals.Add(1)
			return 0, true
		},
	}

	// The registry's own reveal already woke the poller; the switch below is the
	// wake this measures.
	<-reg.touched
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, reg, root, nil, emit, time.Millisecond, time.Hour, counts)

	waitFor(t, "the fast ticker to push repeatedly", func() bool { return pushes.Load() > 20 })
	if got := all.Load(); got != 1 {
		t.Fatalf("every session's unseen count ran %d times over %d pushes, want the one before the loop",
			got, pushes.Load())
	}
	if got := arrivals.Load(); got != 0 {
		t.Fatalf("the fast ticker recounted an arrival %d times", got)
	}

	reg.reveal(reg.add(sessionDir(t, root, "kraken"), []string{"/bin/cat"}, os.Environ()))
	waitFor(t, "the arrival to be recounted", func() bool { return arrivals.Load() == 1 })
	if got := all.Load(); got != 1 {
		t.Fatalf("switching recounted every session's unseen documents, %d counts in all", got)
	}
}

// Opening the app comes back to the session you were last in, which only works
// if putting one on screen stamps it and leaves every other stamp where it was.
func TestShowingASessionStampsOnlyThatOne(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	// The stamp lands inside the shown hook, which the session reaching the
	// screen does not order against.
	var first time.Time
	waitFor(t, "the workbench to stamp the session it opened on", func() bool {
		var ok bool
		first, ok = session.LastOpened(opts.SessionRoot)
		return ok
	})

	kraken := sessionDir(t, opts.Root, "kraken")
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	var second time.Time
	waitFor(t, "the session the rail showed to be stamped", func() bool {
		at, ok := session.LastOpened(kraken)
		second = at
		return ok
	})
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
	reg.boot.agent = func(req AgentRequest) (AgentCommand, error) {
		entered <- struct{}{}
		<-release
		return command(req)
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
	if alpha.provider != "codex" || beta.provider != "codex" {
		t.Fatalf("resolved providers = %q, %q, want codex", alpha.provider, beta.provider)
	}
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
	reg, windows, _ := testSessions(t, root, boot)
	for _, slug := range []string{"alpha", "beta"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
	}

	alpha := filepath.Join(root, "alpha")
	host := (workbench.Handle{Socket: boot.socket(t, alpha), SessionRoot: alpha}).WindowHost()
	id, err := host.Open(context.Background(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ alpha dev", Cwd: alpha, Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if drawn := windows.Surfaces("alpha"); len(drawn.Tabs) != 1 || drawn.Tabs[0].ID != id {
		t.Fatalf("alpha sees %+v, want the window opened on its own socket", drawn)
	}
	if drawn := windows.Surfaces("beta"); len(drawn.Tabs) != 0 {
		t.Fatalf("beta sees %+v, want none of alpha's windows", drawn)
	}
}

// A workbench that opened on a session leaves its process socket owning none, so
// all it answers is the launcher's readiness poll and an onboarding child's
// adopt. A window op is refused in an answer rather than by a closed door.
func TestTheProcessSocketAnswersReadinessAndRefusesAWindowOp(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	adopted := sessionDir(t, opts.Root, "kraken")
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	if !workbench.Answered(opts.Socket) {
		t.Fatal("the process socket does not answer the launcher's readiness poll")
	}
	host := (workbench.Handle{Socket: opts.Socket, SessionRoot: opts.SessionRoot}).WindowHost()
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

	if err := reg.adopt(adopted, ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the adopted session on screen", func() bool {
		state := reg.current()
		return state != nil && state.slug() == "kraken"
	})
}

func TestTheProcessSocketQueuesLinearAssemblyWithoutTouchingTheRunningAgent(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)
	opts.assembly = newAssembly(&config.Config{Root: opts.Root}, nil, reg, r.Emit, nil, nil)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	running := shownSession(t, reg)
	beforeAgents, beforeServes := boot.counts()

	outcome, err := workbench.OpenLinearIssue(context.Background(), opts.Socket, "lif-2841", "Linear prompt")
	if err != nil || outcome != assemblyOutcomeQueued {
		t.Fatalf("OpenLinearIssue = %q, %v", outcome, err)
	}
	if reg.current() != running {
		t.Fatal("opening a draft switched the running session")
	}
	afterAgents, afterServes := boot.counts()
	if afterAgents != beforeAgents || afterServes != beforeServes {
		t.Fatalf("opening a draft changed agent/listener counts from %d/%d to %d/%d",
			beforeAgents, beforeServes, afterAgents, afterServes)
	}
	if pending := opts.assembly.Pending(); pending != "https://linear.app/issue/LIF-2841" {
		t.Fatalf("pending = %q", pending)
	}
	if _, prompt := opts.assembly.pendingLinear(); prompt != "Linear prompt" {
		t.Fatalf("pending prompt = %q", prompt)
	}
	waitFor(t, "the conversation window to be focused", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.focused[mainWindowName] == 1
	})

	seed := opts.assembly.Begin()
	if outcome, err = workbench.OpenLinearIssue(context.Background(), opts.Socket, "LIF-2841", "duplicate"); err != nil || outcome != assemblyOutcomeDraft {
		t.Fatalf("same issue repeat = %q, %v", outcome, err)
	}
	if _, err = workbench.OpenLinearIssue(context.Background(), opts.Socket, "LIF-2842", ""); err == nil {
		t.Fatal("different issue replaced the open draft")
	}
	opts.assembly.End(seed.Generation)
}

func TestAColdLinearIssueIsPendingBeforeTheWindowOpens(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	opts.SessionRoot = ""
	opts.LinearIssue = "https://linear.app/issue/LIF-2841"
	opts.LinearPrompt = "Cold prompt"
	reg, term, windows := testWorkbench(t, r, r.Emit)
	opts.assembly = newAssembly(&config.Config{Root: opts.Root}, nil, reg, r.Emit, nil, nil)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	if pending := opts.assembly.Pending(); pending != opts.LinearIssue {
		t.Fatalf("pending after open = %q, want %q", pending, opts.LinearIssue)
	}
	if _, prompt := opts.assembly.pendingLinear(); prompt != opts.LinearPrompt {
		t.Fatalf("pending prompt after open = %q, want %q", prompt, opts.LinearPrompt)
	}
	if agents, serves := boot.counts(); agents != 0 || serves != 0 {
		t.Fatalf("cold draft started %d agents and %d session listeners", agents, serves)
	}
}

func TestAColdLinearIssueIsInstalledBeforeTheProcessEndpointIsPublished(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	opts.SessionRoot = ""
	opts.LinearIssue = "https://linear.app/issue/LIF-2841"
	reg, term, windows := testWorkbench(t, r, r.Emit)
	offered := make(chan struct{})
	release := make(chan struct{})
	opts.assembly = newAssembly(&config.Config{Root: opts.Root}, nil, reg, func(event string, _ any) {
		if event != assemblyRequestedEvent {
			return
		}
		close(offered)
		<-release
	}, nil, nil)

	startWorkbench(t, r, term, windows, opts)
	<-offered
	if workbench.Published(opts.Socket) {
		t.Fatal("the process endpoint was published before its cold Linear issue was installed")
	}
	if pending := opts.assembly.Pending(); pending != opts.LinearIssue {
		t.Fatalf("pending during publication handoff = %q, want %q", pending, opts.LinearIssue)
	}
	close(release)
	waitFor(t, "the seeded process endpoint to be published", func() bool {
		return workbench.Published(opts.Socket)
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

	startWorkbench(t, r, term, windows, opts)
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
		windows := newWindows(r.Emit, reg)
		release := make(chan struct{})
		serving := make(chan struct{})
		var reserved string
		reg.boot = booting{
			root: func(slug string) string { return filepath.Join(root, slug) },
			agent: func(AgentRequest) (AgentCommand, error) {
				return AgentCommand{Argv: []string{"/bin/cat"}, Env: os.Environ(), RunnerID: "codex"}, nil
			},
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

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	assembled := sessionDir(t, opts.Root, "kraken")
	if err := reg.adopt(assembled, ""); err != nil {
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

// A cleanup or an rm -rf takes a session's directory. Keeping the slug's place
// means it comes back where it was rather than at the front.
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
	// The chrome poller freezes the rail on whichever poll gets there first, so
	// the order under test is the stamps' and not that poll's timing.
	if err := session.MarkOpened(opts.SessionRoot, time.Now()); err != nil {
		t.Fatal(err)
	}
	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	// Which session the frozen order puts first is whatever the first poll saw, and
	// the poller runs on its own goroutine — so the invariant is that switching does
	// not reorder the rail, not that a named session sits at the top.
	before := polledRail(t, r, reg, opts.Root)
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	after := polledRail(t, r, reg, opts.Root)

	fields := lastChrome(t, r)
	if fields.Slug != "kraken" {
		t.Fatalf("chrome names %q as shown, want the session Show revealed", fields.Slug)
	}
	if len(after) < 2 {
		t.Fatalf("rail has %d rows, want both sessions", len(after))
	}
	if strings.Join(before, " ") != strings.Join(after, " ") {
		t.Fatalf("switching reordered the rail from %v to %v", before, after)
	}
}

// Removing a session's files out from under a live supervisor would leave it
// running on an unlinked directory, so everything it holds has to be released
// before the directory goes.
func TestCleanupTearsTheSessionDownBeforeItRemovesIt(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, windows, r := testSessions(t, root, boot)
	dir := sessionDir(t, root, "octopus")

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	state := reg.current()
	term := newTerm(reg, r.Emit)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	pid := state.process.cmd.Process.Pid
	socket := boot.socket(t, dir)
	host := (workbench.Handle{Socket: socket, SessionRoot: dir}).WindowHost()
	if _, err := host.Open(context.Background(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: dir, Command: []string{"/bin/cat"},
	}); err != nil {
		t.Fatal(err)
	}
	var held []string
	reg.boot.cleanup = func(sessionRoot string) error {
		state.mu.Lock()
		if state.process != nil {
			held = append(held, "its conversation")
		}
		if state.control != nil {
			held = append(held, "its listener")
		}
		state.mu.Unlock()
		if drawn := windows.Surfaces("octopus"); len(drawn.Tabs) > 0 {
			held = append(held, "its windows")
		}
		return os.RemoveAll(sessionRoot)
	}

	if err := reg.Cleanup("octopus"); err != nil {
		t.Fatal(err)
	}
	if len(held) > 0 {
		t.Fatalf("the session still had %v when its directory was removed", held)
	}
	waitFor(t, "the conversation to end", func() bool { return syscall.Kill(pid, 0) != nil })
	if workbench.Answered(socket) {
		t.Fatalf("the cleaned-up session still answers on %q", socket)
	}
	if _, err := os.Stat(sessionpaths.Manifest(dir)); !os.IsNotExist(err) {
		t.Fatal("the manifest is back under the removed session, so something wrote it after the removal")
	}
	if reg.bySlug("octopus") != nil {
		t.Fatal("the cleaned-up session is still registered")
	}
}

// Cleaning up the session you are looking at is allowed, and it ends the same way
// a supervisor exiting does: the window falls back rather than closing.
func TestCleanupFallsTheWindowBackAsARetirementDoes(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	reg.boot.cleanup = func(sessionRoot string) error { return os.RemoveAll(sessionRoot) }
	for _, slug := range []string{"octopus", "kraken", "webhook"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
	}

	if err := reg.Cleanup("octopus"); err != nil {
		t.Fatal(err)
	}
	if state := reg.current(); state == nil || state.slug() != "webhook" {
		t.Fatalf("cleaning up a background session moved the window to %v", state)
	}
	if err := reg.Cleanup("webhook"); err != nil {
		t.Fatal(err)
	}
	if state := reg.current(); state == nil || state.slug() != "kraken" {
		t.Fatalf("the window fell back to %v, want the session shown before it", state)
	}
	if err := reg.Cleanup("kraken"); err != nil {
		t.Fatal(err)
	}
	if state := reg.current(); state != nil {
		t.Fatalf("cleaning up the last session left %q on screen", state.slug())
	}
}

// A removal can fail with earlier worktrees already gone, and the dialog has
// nothing to say about it but the error it was handed.
func TestCleanupHandsTheRemovalsFailureBack(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	refused := errors.New("remove lifesum/api worktree: permission denied")
	reg.boot.cleanup = func(string) error { return refused }
	sessionDir(t, root, "octopus")
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}

	if err := reg.Cleanup("octopus"); !errors.Is(err, refused) {
		t.Fatalf("cleanup failure = %v, want the removal's own error intact", err)
	}
}

// A supervisor that outlived its workbench is the one case where removing a
// session's worktrees is worse than leaving it alone. A dead pid is the residue
// of a crash and must not strand the session.
func TestCleanupRefusesAnUnresolvableSlugAndALiveAgent(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	var removals atomic.Int64
	reg.boot.cleanup = func(sessionRoot string) error {
		removals.Add(1)
		return os.RemoveAll(sessionRoot)
	}

	// A directory that lost its manifest is not a session, and the row naming it
	// can outlive it.
	if err := os.MkdirAll(filepath.Join(root, "ghost"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"ghost", "never-existed"} {
		if err := reg.Cleanup(slug); err == nil {
			t.Fatalf("%q was cleaned up with no session under the root", slug)
		} else if !strings.Contains(err.Error(), slug) {
			t.Fatalf("refusal %q does not name the session asked for", err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "ghost")); err != nil {
		t.Fatal("the refused directory was removed anyway:", err)
	}

	writeAgentPID(t, sessionDir(t, root, "live"), os.Getpid())
	err := reg.Cleanup("live")
	if err == nil {
		t.Fatal("a session with a running agent had its worktrees removed underneath it")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(os.Getpid())) {
		t.Fatalf("refusal %q does not name the pid holding the session", err)
	}
	if got := removals.Load(); got != 0 {
		t.Fatalf("the refused sessions were removed %d times", got)
	}

	crashed := sessionDir(t, root, "crashed")
	writeAgentPID(t, crashed, deadPID(t))
	if err := reg.Cleanup("crashed"); err != nil {
		t.Fatalf("a session whose recorded agent is gone refused to be cleaned up: %v", err)
	}
	if _, err := os.Stat(crashed); !os.IsNotExist(err) {
		t.Fatal("the stranded session's directory survived its cleanup")
	}
}

// The rail loses the row on the act rather than up to a tick later.
func TestCleanupWakesTheChromePoller(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	reg.boot.cleanup = func(sessionRoot string) error { return os.RemoveAll(sessionRoot) }
	sessionDir(t, root, "octopus")

	select {
	case <-reg.touched:
	default:
	}
	if err := reg.Cleanup("octopus"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reg.touched:
	default:
		t.Fatal("cleaning up a session left the rail drawing it until the next tick")
	}
}

// The dirty check is N git status subprocesses, so it answers the dialog that
// asked for it and never a poll.
func TestUncommittedAnswersAClickAndNeverATicker(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	var checks atomic.Int64
	dirty := []string{"lifesum/api", "lifesum/web"}
	reg.boot.uncommitted = func(string) ([]string, error) {
		checks.Add(1)
		return dirty, nil
	}
	reg.reveal(reg.add(sessionDir(t, root, "octopus"), []string{"/bin/cat"}, os.Environ()))

	var pushes atomic.Int64
	emit := func(event string, _ any) {
		if event == chromeEvent {
			pushes.Add(1)
		}
	}
	counts := unseenCounts{
		all: func(string) map[string]int { return map[string]int{} },
		in:  func(string) (int, bool) { return 0, true },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, reg, root, nil, emit, time.Millisecond, 2*time.Millisecond, counts)
	waitFor(t, "both tickers to push repeatedly", func() bool { return pushes.Load() > 20 })
	if got := checks.Load(); got != 0 {
		t.Fatalf("the dirty check ran %d times over %d pushes, want none of them", got, pushes.Load())
	}

	got, err := reg.Uncommitted("octopus")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, dirty) {
		t.Fatalf("uncommitted = %v, want the repositories the check named", got)
	}

	dirty = nil
	if got, err = reg.Uncommitted("octopus"); err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("a clean session answered nil, which marshals as JSON null and reaches a .length on the page")
	}
	if len(got) != 0 {
		t.Fatalf("a clean session answered %v", got)
	}

	before := checks.Load()
	if _, err := reg.Uncommitted("ghost"); err == nil {
		t.Fatal("a slug with no session under the root was checked anyway")
	}
	if checks.Load() != before {
		t.Fatal("the unresolvable slug was checked on disk")
	}
}

// Revealing a row is not switching to it: the common case is finding the files
// of a session you are not looking at.
func TestRevealNamesTheRowsOwnDirectoryAndDisturbsNothing(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	refused := errors.New("nothing opened it")
	var revealed []string
	var fail bool
	reg.boot.reveal = func(sessionRoot string) error {
		revealed = append(revealed, sessionRoot)
		if fail {
			return refused
		}
		return nil
	}
	sessionDir(t, root, "octopus")
	kraken := sessionDir(t, root, "kraken")
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	shown := reg.current()
	select {
	case <-reg.touched:
	default:
	}

	if err := reg.Reveal("kraken"); err != nil {
		t.Fatal(err)
	}
	if len(revealed) != 1 || revealed[0] != kraken {
		t.Fatalf("revealed %v, want the row's own directory %q", revealed, kraken)
	}
	if reg.current() != shown {
		t.Fatal("revealing a row put it on screen")
	}
	if reg.bySlug("kraken") != nil {
		t.Fatal("revealing a session booted it")
	}
	select {
	case <-reg.touched:
		t.Fatal("revealing woke the poller, and nothing in the chrome payload moved")
	default:
	}

	if err := reg.Reveal("ghost"); err == nil {
		t.Fatal("a slug with no session under the root was revealed anyway")
	}
	if len(revealed) != 1 {
		t.Fatalf("the unresolvable slug reached the file manager: %v", revealed)
	}

	fail = true
	if err := reg.Reveal("octopus"); !errors.Is(err, refused) {
		t.Fatalf("reveal failure = %v, want the refusal intact", err)
	}
}

// A path nothing can open is an error the page can show rather than a click that
// silently does nothing, which is what waiting on the command buys.
func TestTheWorkbenchRunsTheRevealCommandAndRefusesWithoutOne(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	marker := filepath.Join(t.TempDir(), "revealed")
	boot.reveal = func(sessionRoot string) []string {
		if filepath.Base(sessionRoot) == "kraken" {
			return []string{"/bin/sh", "-c", "exit 3"}
		}
		return []string{"/bin/sh", "-c", `printf %s "$1" > "$2"`, "sh", sessionRoot, marker}
	}
	kraken := sessionDir(t, opts.Root, "kraken")
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	if err := reg.Reveal(filepath.Base(opts.SessionRoot)); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(marker); err != nil || string(got) != opts.SessionRoot {
		t.Fatalf("the reveal command was run with %q (%v), want the session's root %q", got, err, opts.SessionRoot)
	}
	if err := reg.Reveal(filepath.Base(kraken)); err == nil {
		t.Fatal("a reveal command that failed came back as a success")
	}
}

// A launcher that builds no command for a session is a workbench that cannot do
// that thing: the page gets the sentinel rather than an empty argv reaching exec.
func TestAWorkbenchWithNoShellCommandRefuses(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	boot.shell = func(string) []string { return nil }
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	owner := shownSession(t, reg)

	if _, err := windows.OpenShell(); !errors.Is(err, ErrNoShellCommand) {
		t.Fatalf("a shell with nothing to run = %v, want ErrNoShellCommand", err)
	}
	if tabs := windows.surfaces(owner).Tabs; len(tabs) != 0 {
		t.Fatalf("the refused shell opened %d tabs", len(tabs))
	}
	if _, err := windows.OpenDocument("thoughts/shared/plans/P1.md"); !errors.Is(err, ErrNoEditorCommand) {
		t.Fatalf("a document with no editor = %v, want ErrNoEditorCommand", err)
	}
}

func TestAWorkbenchWithNoRevealCommandRefuses(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	if err := reg.Reveal(filepath.Base(opts.SessionRoot)); !errors.Is(err, ErrNoRevealCommand) {
		t.Fatalf("reveal with nothing to run = %v, want ErrNoRevealCommand", err)
	}
}

// A removal resolves its target from the manifest rather than from the directory
// the manifest was read in, so a directory holding another session's manifest
// would take that session's worktrees — one click away from --force.
func TestCleanupRefusesADirectoryHoldingAnotherSessionsManifest(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	reg, term, windows := testWorkbench(t, r, r.Emit)

	startWorkbench(t, r, term, windows, opts)
	<-r.opened
	shownSession(t, reg)

	kraken := sessionDir(t, opts.Root, "kraken")
	stray := filepath.Join(opts.Root, "webhook")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(stray, session.Manifest{Slug: "kraken", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}

	err := reg.Cleanup("webhook")
	if err == nil {
		t.Fatal("a directory holding another session's manifest was cleaned up anyway")
	}
	for _, name := range []string{"webhook", "kraken"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("refusal %q names neither the directory asked for nor the one its manifest claims", err)
		}
	}
	if _, err := os.Stat(sessionpaths.Manifest(kraken)); err != nil {
		t.Fatal("the session the stray manifest named was removed:", err)
	}
	if _, err := os.Stat(stray); err != nil {
		t.Fatal("the refused directory was removed:", err)
	}
}

// polledRail is the rail as one poll draws it, pushed explicitly so the assertion
// does not race the background poller's own tick.
func polledRail(t *testing.T, r *fakeRenderer, reg *Sessions, root string) []string {
	t.Helper()
	pushChrome(reg, root, nil, nil, nil, r.Emit)
	var slugs []string
	for _, row := range lastChrome(t, r).Sessions {
		slugs = append(slugs, row.Slug)
	}
	return slugs
}

func lastChrome(t *testing.T, r *fakeRenderer) status.Fields {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed: %v", r.events)
	}
	return fields
}
