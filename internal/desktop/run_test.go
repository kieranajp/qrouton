package desktop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/vault"
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
	closed  []string
	sent    []delivery
	quits   atomic.Int32
	inFront atomic.Bool
	quit    bool
	block   chan struct{}
	once    sync.Once
	// atQuit runs as the application is told to quit, before the event loop ends.
	atQuit func()
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

func (f *fakeRenderer) Focused(string) bool { return f.inFront.Load() }

func (f *fakeRenderer) Close(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = append(f.closed, name)
}

func (f *fakeRenderer) Emit(event string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events[event] = payload
}

func (f *fakeRenderer) Send(name, event string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, delivery{window: name, event: event, payload: payload})
}

func (f *fakeRenderer) Run() error {
	<-f.block
	return nil
}

// Quit tolerates being called twice: the workbench quits itself when its
// conversation window closes, and the test stops it again on the way out.
func (f *fakeRenderer) Quit() {
	f.quits.Add(1)
	f.once.Do(func() {
		if f.atQuit != nil {
			f.atQuit()
		}
		f.mu.Lock()
		f.quit = true
		f.mu.Unlock()
		close(f.block)
	})
}

// delivery is one payload the toolkit was asked to put in front of one window's
// page, rather than every page.
type delivery struct {
	window  string
	event   string
	payload any
}

// stubBoot stands in for the launcher, counting the supervisor commands asked
// for and keeping each one's address. The local commands are unset until a test
// sets one, which is the workbench that cannot do that thing.
type stubBoot struct {
	argv     []string
	shell    func(sessionRoot string) []string
	reveal   func(sessionRoot string) []string
	copyPic  func(path string) []string
	copyDoc  func(path string) []string
	openText func(path string) []string
	// booting runs inside each supervisor request, while the session boots.
	booting func(core.AgentRequest)

	mu      sync.Mutex
	sockets map[string]string
	resumes map[string]bool
	runners map[string]string
	agents  int
}

func newStubBoot(argv ...string) *stubBoot {
	return &stubBoot{argv: argv, sockets: map[string]string{}, resumes: map[string]bool{}, runners: map[string]string{}}
}

func (b *stubBoot) Agent(req core.AgentRequest) (core.AgentCommand, error) {
	if b.booting != nil {
		b.booting(req)
	}
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
	return core.AgentCommand{Argv: b.argv, Env: os.Environ(), RunnerID: resolved}, nil
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

func (b *stubBoot) OpenText(path string) []string {
	if b.openText == nil {
		return nil
	}
	return b.openText(path)
}

func (b *stubBoot) CopyImage(path string) []string {
	if b.copyPic == nil {
		return nil
	}
	return b.copyPic(path)
}

func (b *stubBoot) CopyFile(path string) []string {
	if b.copyDoc == nil {
		return nil
	}
	return b.copyDoc(path)
}

func (b *stubBoot) Document(string, string) (workbench.WindowOptions, error) {
	return workbench.WindowOptions{}, core.ErrNoEditorCommand
}

func (b *stubBoot) Runners() ([]assembly.Runner, error) { return nil, nil }

func (b *stubBoot) Signal(string) {}

// counts is the supervisors asked for and the session listeners answering.
func (b *stubBoot) counts() (agents, serves int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, socket := range b.sockets {
		if workbench.Answered(socket) {
			serves++
		}
	}
	return b.agents, serves
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

// testWorkbench builds the core as Run does, over a local client on r. It reads
// the assembly through opts when a ticket arrives, so a test may set one after.
func testWorkbench(t *testing.T, r *fakeRenderer, opts *Options) *core.Core {
	t.Helper()
	kernel := core.New(core.Options{
		Root:     opts.Root,
		Config:   opts.Config,
		Launcher: opts.Launcher,
		OpenTicket: func(url, prompt string) (string, error) {
			return opts.assembly.offer(url, prompt)
		},
	}, newLocalClient(r, opts.Launcher, nil))
	t.Cleanup(kernel.Shutdown)
	return kernel
}

// startWorkbench runs the workbench and stops it before the temporary root goes:
// a session reaching the screen does not order against the stamping and
// recording that follows it, which would otherwise land in a directory the
// test's cleanup is already removing. The channel carries run's result.
func startWorkbench(t *testing.T, r *fakeRenderer, kernel *core.Core, opts Options) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		done <- run(r, kernel, opts, quitter(kernel, r))
	}()
	t.Cleanup(func() {
		r.Quit()
		<-stopped
	})
	return done
}

// newSessions is a registry with nothing booted, over a core with no launcher.
func newSessions(t *testing.T) *core.Sessions {
	t.Helper()
	return testWorkbench(t, newFakeRenderer(), &Options{}).Sessions
}

// testSessions is a registry that boots sessions under root with boot's
// supervisor, as Run's does.
func testSessions(t *testing.T, root string, boot *stubBoot) *core.Sessions {
	t.Helper()
	return testWorkbench(t, newFakeRenderer(), &Options{Root: root, Launcher: boot}).Sessions
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
func shownSession(t *testing.T, reg *core.Sessions) *core.SessionState {
	t.Helper()
	var state *core.SessionState
	waitFor(t, "the session on screen", func() bool {
		state = reg.Current()
		return state != nil
	})
	return state
}

// waitCeiling is not a latency any test here asserts: a child answers in
// milliseconds or never, and a shared runner can stall for whole seconds.
const waitCeiling = 30 * time.Second

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(waitCeiling)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type offlineEmbedder struct{}

var errOffline = errors.New("ollama is not running")

func (offlineEmbedder) Model(context.Context) (vault.Model, error) { return vault.Model{}, errOffline }
func (offlineEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errOffline
}

func TestRunOpensOneConversationWindowAtTheFrontendRoot(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions

	startWorkbench(t, r, kernel, opts)

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

// Breaks if QROUTON_INSPECT no longer reaches the page the window loads.
func TestRunAsksThePageToLeaveRightClicksToTheWebviewWhenInspecting(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	opts.Inspect = true
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions

	startWorkbench(t, r, kernel, opts)

	spec := <-r.opened
	shownSession(t, reg)
	if want := frontendRoot + "?" + inspectQuery; spec.URL != want {
		t.Fatalf("window URL is %q, want %q", spec.URL, want)
	}
}

// Closing the conversation window ends the session: no detached server, and
// nothing surviving in the background.
func TestClosingTheConversationWindowQuits(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	pidFile := filepath.Join(t.TempDir(), "pid")
	opts.Launcher.(*stubBoot).argv = []string{"/bin/sh", "-c", `echo $$ > "$1"; exec cat`, "sh", pidFile}
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions

	done := startWorkbench(t, r, kernel, opts)

	conversation := <-r.opened
	shownSession(t, reg)
	if err := kernel.Term.Start(chromeOf(t, kernel, func(f status.Fields) bool { return f.Terminal != "" }).Terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	var pid int
	waitFor(t, "the supervisor's pid", func() bool {
		data, err := os.ReadFile(pidFile)
		pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil
	})
	// The core goes down before the application does, so nothing outlives the
	// event loop that Run returns from.
	var socketAtQuit, supervisorAtQuit bool
	r.atQuit = func() {
		socketAtQuit = workbench.Answered(opts.Socket)
		supervisorAtQuit = syscall.Kill(pid, 0) == nil
	}
	conversation.OnClose()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.quit {
		t.Fatal("closing the conversation window left the application running")
	}
	if socketAtQuit || supervisorAtQuit {
		t.Fatalf("at Quit the process socket answered %v and the supervisor was alive %v; the core must be down first", socketAtQuit, supervisorAtQuit)
	}
}

// A second window is not a second thing to shut down: the conversation's own
// teardown still ends the app, and ends it once.
func TestQuittingWithTheNotesWindowOpenEndsTheApplicationOnce(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions
	presenter := newPresenter(r, r.Emit)

	done := startWorkbench(t, r, kernel, opts)
	conversation := <-r.opened
	shownSession(t, reg)
	if err := presenter.Open(); err != nil {
		t.Fatal(err)
	}
	<-r.opened

	conversation.OnClose()
	conversation.OnClose()

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := r.quits.Load(); got != 1 {
		t.Fatalf("the application was told to quit %d times, want once", got)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.quit {
		t.Fatal("quitting with a notes window open left the application running")
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

func TestTheProcessSocketQueuesLinearAssemblyWithoutTouchingTheRunningAgent(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions
	opts.assembly = newAssembly(&config.Config{Root: opts.Root}, nil, reg, r.Emit, nil, nil)

	startWorkbench(t, r, kernel, opts)
	<-r.opened
	running := shownSession(t, reg)
	beforeAgents, beforeServes := boot.counts()

	outcome, err := workbench.OpenTicket(context.Background(), opts.Socket, "lif-2841", "Linear prompt")
	if err != nil || outcome != assembly.OutcomeQueued {
		t.Fatalf("OpenTicket = %q, %v", outcome, err)
	}
	if reg.Current() != running {
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
	if _, prompt := opts.assembly.pendingTicket(); prompt != "Linear prompt" {
		t.Fatalf("pending prompt = %q", prompt)
	}
	waitFor(t, "the conversation window to be focused", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.focused[mainWindowName] == 1
	})

	seed := opts.assembly.Begin()
	if outcome, err = workbench.OpenTicket(context.Background(), opts.Socket, "LIF-2841", "duplicate"); err != nil || outcome != assembly.OutcomeDraft {
		t.Fatalf("same issue repeat = %q, %v", outcome, err)
	}
	if _, err = workbench.OpenTicket(context.Background(), opts.Socket, "LIF-2842", ""); err == nil {
		t.Fatal("different issue replaced the open draft")
	}
	opts.assembly.End(seed.Generation)
}

func TestAColdTicketIsPendingBeforeTheWindowOpens(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	opts.SessionRoot = ""
	opts.Ticket = "https://linear.app/issue/LIF-2841"
	opts.TicketPrompt = "Cold prompt"
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions
	opts.assembly = newAssembly(&config.Config{Root: opts.Root}, nil, reg, r.Emit, nil, nil)

	startWorkbench(t, r, kernel, opts)
	<-r.opened
	if pending := opts.assembly.Pending(); pending != opts.Ticket {
		t.Fatalf("pending after open = %q, want %q", pending, opts.Ticket)
	}
	if _, prompt := opts.assembly.pendingTicket(); prompt != opts.TicketPrompt {
		t.Fatalf("pending prompt after open = %q, want %q", prompt, opts.TicketPrompt)
	}
	if agents, serves := boot.counts(); agents != 0 || serves != 0 {
		t.Fatalf("cold draft started %d agents and %d session listeners", agents, serves)
	}
}

func TestAColdTicketIsInstalledBeforeTheProcessEndpointIsPublished(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	opts.SessionRoot = ""
	opts.Ticket = "https://linear.app/issue/LIF-2841"
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions
	offered := make(chan struct{})
	release := make(chan struct{})
	opts.assembly = newAssembly(&config.Config{Root: opts.Root}, nil, reg, func(event string, _ any) {
		if event != assemblyRequestedEvent {
			return
		}
		close(offered)
		<-release
	}, nil, nil)

	startWorkbench(t, r, kernel, opts)
	<-offered
	if workbench.Published(opts.Socket) {
		t.Fatal("the process endpoint was published before its cold Linear issue was installed")
	}
	if pending := opts.assembly.Pending(); pending != opts.Ticket {
		t.Fatalf("pending during publication handoff = %q, want %q", pending, opts.Ticket)
	}
	close(release)
	waitFor(t, "the seeded process endpoint to be published", func() bool {
		return workbench.Published(opts.Socket)
	})
}

func TestAWorkbenchWithNoRevealCommandRefuses(t *testing.T) {
	r := newFakeRenderer()
	opts, _ := testOptions(t)
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions

	startWorkbench(t, r, kernel, opts)
	<-r.opened
	shownSession(t, reg)

	if err := reg.Reveal(filepath.Base(opts.SessionRoot)); !errors.Is(err, ErrNoRevealCommand) {
		t.Fatalf("reveal with nothing to run = %v, want ErrNoRevealCommand", err)
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
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions

	startWorkbench(t, r, kernel, opts)
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

// startedCore is a core built as Run builds it and serving its own process
// socket, so its chrome poller is running.
func startedCore(t *testing.T, opts *Options) *core.Core {
	t.Helper()
	kernel := testWorkbench(t, newFakeRenderer(), opts)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(socket) })
	if err := kernel.Start(socket); err != nil {
		t.Fatal(err)
	}
	return kernel
}

// resumed reports whether the session rooted at sessionRoot was asked to
// continue its previous conversation.
func (b *stubBoot) resumed(sessionRoot string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.resumes[sessionRoot]
}

// chromeOf is the chrome a started core publishes once want holds of it.
func chromeOf(t *testing.T, kernel *core.Core, want func(status.Fields) bool) status.Fields {
	t.Helper()
	var fields status.Fields
	waitFor(t, "the chrome to be published", func() bool {
		fields = kernel.Chrome.Snapshot()
		return want(fields)
	})
	return fields
}
