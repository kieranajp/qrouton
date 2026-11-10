package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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
	// atEmit runs before each broadcast is recorded, outside the lock.
	atEmit func(event string)
	// openErr is what every Open answers, once set.
	openErr error
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
	if f.openErr != nil {
		return f.openErr
	}
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
	if f.atEmit != nil {
		f.atEmit(event)
	}
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
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
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

// testWorkbench builds the core as Run does, over a local client on r.
func testWorkbench(t *testing.T, r *fakeRenderer, opts *Options) *core.Core {
	t.Helper()
	var quit func()
	kernel := core.New(coreOptions(*opts, func() { quit() }), newLocalClient(r, opts.Launcher, nil))
	quit = quitter(kernel, r)
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
	if err := Run(Options{SessionRoot: t.TempDir(), Socket: "/tmp/x.sock"}); err != core.ErrNoAgentCommand {
		t.Fatalf("Run with no way to build an agent command returned %v, want ErrNoAgentCommand", err)
	}
	if err := Run(Options{SessionRoot: t.TempDir(), Launcher: launcher}); err != ErrNoControlSocket {
		t.Fatalf("Run with no socket returned %v, want ErrNoControlSocket", err)
	}
	if err := Run(Options{SessionRoot: t.TempDir(), Socket: "/tmp/x.sock", Launcher: launcher}); err != core.ErrNoConfig {
		t.Fatalf("Run with nothing to assemble against returned %v, want ErrNoConfig", err)
	}
}

func TestTheProcessSocketQueuesLinearAssemblyWithoutTouchingTheRunningAgent(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	opts.Config = &config.Config{Root: opts.Root}
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions

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
	if pending := kernel.Assembly.Pending(); pending != "https://linear.app/issue/LIF-2841" {
		t.Fatalf("pending = %q", pending)
	}
	if _, prompt := kernel.PendingTicket(); prompt != "Linear prompt" {
		t.Fatalf("pending prompt = %q", prompt)
	}
	waitFor(t, "the conversation window to be focused", func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.focused[mainWindowName] == 1
	})

	seed := kernel.Assembly.Begin()
	if seed.Ticket != "https://linear.app/issue/LIF-2841" {
		t.Fatalf("the draft opened on %q, not the offered ticket", seed.Ticket)
	}
	if outcome, err = workbench.OpenTicket(context.Background(), opts.Socket, "LIF-2841", "duplicate"); err != nil || outcome != assembly.OutcomeDraft {
		t.Fatalf("same issue repeat = %q, %v", outcome, err)
	}
	if _, err = workbench.OpenTicket(context.Background(), opts.Socket, "LIF-2842", ""); err == nil {
		t.Fatal("different issue replaced the open draft")
	}
	kernel.Assembly.End(seed.Generation)
}

func TestAColdTicketIsPendingBeforeTheWindowOpens(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	opts.SessionRoot = ""
	opts.Ticket = "https://linear.app/issue/LIF-2841"
	opts.TicketPrompt = "Cold prompt"
	opts.Config = &config.Config{Root: opts.Root}
	kernel := testWorkbench(t, r, &opts)

	startWorkbench(t, r, kernel, opts)
	<-r.opened
	if pending := kernel.Assembly.Pending(); pending != opts.Ticket {
		t.Fatalf("pending after open = %q, want %q", pending, opts.Ticket)
	}
	if _, prompt := kernel.PendingTicket(); prompt != opts.TicketPrompt {
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
	opts.Config = &config.Config{Root: opts.Root}
	offered := make(chan struct{})
	release := make(chan struct{})
	r.atEmit = func(event string) {
		if event != "assembly:requested" {
			return
		}
		close(offered)
		<-release
	}
	kernel := testWorkbench(t, r, &opts)

	startWorkbench(t, r, kernel, opts)
	<-offered
	if workbench.Published(opts.Socket) {
		t.Fatal("the process endpoint was published before its cold Linear issue was installed")
	}
	if pending := kernel.Assembly.Pending(); pending != opts.Ticket {
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

	if err := reg.Reveal(filepath.Base(opts.SessionRoot)); !errors.Is(err, core.ErrNoRevealCommand) {
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

// recordingRelauncher keeps what each relaunch was handed to carry.
type recordingRelauncher struct {
	calls   int
	pending func() (string, string)
}

func (r *recordingRelauncher) Relaunch(pending func() (string, string)) error {
	r.calls++
	r.pending = pending
	return nil
}

// Catches the Relauncher not reaching Settings, or a settings restart carrying
// the pending ticket into the successor, which would open it a second time there.
func TestTheRelauncherReachesASettingsRestartWithoutATicket(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := &config.Config{Root: t.TempDir()}
	kernel := testWorkbench(t, newFakeRenderer(), &Options{Config: cfg})
	if err := (&Settings{settings: kernel.Settings}).Relaunch(); !errors.Is(err, core.ErrNoRelaunch) {
		t.Fatalf("Relaunch with no Relauncher = %v, want ErrNoRelaunch", err)
	}

	relauncher := &recordingRelauncher{}
	kernel = testWorkbench(t, newFakeRenderer(), &Options{Config: cfg, Relauncher: relauncher})
	if _, err := kernel.Offer("LIF-2841", ""); err != nil {
		t.Fatal(err)
	}
	if err := (&Settings{settings: kernel.Settings}).Relaunch(); err != nil {
		t.Fatal(err)
	}
	if relauncher.calls != 1 || relauncher.pending != nil {
		t.Fatalf("relaunched %d times, carrying a ticket %v; want once with none", relauncher.calls, relauncher.pending != nil)
	}
}

// refusingValidator refuses only the field it names, so a refusal shows which
// of the two answers reached the settings panel.
type refusingValidator struct{ field string }

var errRefused = errors.New("refused")

func (v refusingValidator) ValidateEditor([]string) error {
	if v.field == "editor" {
		return errRefused
	}
	return nil
}

func (v refusingValidator) ValidateLaunch(map[string][]string) error {
	if v.field == "launch" {
		return errRefused
	}
	return nil
}

// Catches a nil Validator refusing every editor and launch table, and either
// half of a non-nil one never reaching the settings panel.
func TestANilValidatorAcceptsTheEditorAndTheLaunchTable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	in := core.SettingsInput{Orgs: []string{"acme"}, Root: t.TempDir(), Editor: "vim",
		Launch: `{"claude": ["claude", "--verbose"]}`}
	for _, tc := range []struct {
		validator Validator
		refusal   string
	}{
		{nil, ""},
		{refusingValidator{field: "editor"}, "editor: refused"},
		{refusingValidator{field: "launch"}, "launch: refused"},
	} {
		cfg := &config.Config{Root: in.Root}
		kernel := testWorkbench(t, newFakeRenderer(), &Options{Config: cfg, Validator: tc.validator})
		_, err := (&Settings{settings: kernel.Settings}).Save(in)
		if got := fmt.Sprint(err); (tc.refusal == "" && err != nil) || (tc.refusal != "" && got != tc.refusal) {
			t.Fatalf("Save with Validator %+v = %v, want refusal %q", tc.validator, err, tc.refusal)
		}
		if tc.refusal == "" && (!slices.Equal(cfg.Snapshot().Editor, []string{"vim"}) || len(cfg.Snapshot().Launch) != 1) {
			t.Fatalf("an accepted Save left editor %q, launch %v", cfg.Snapshot().Editor, cfg.Snapshot().Launch)
		}
	}
}

// Settings.Relaunch must run the exact teardown closing the conversation window
// runs, not a bare renderer Quit: every open session ends and the renderer's
// own Quit is reached only after that teardown completes.
func TestSettingsRelaunchRunsTheSameTeardownClosingTheWindowRuns(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	relauncher := &recordingRelauncher{}
	opts.Relauncher = relauncher
	opts.Config = &config.Config{}
	kernel := testWorkbench(t, r, &opts)
	reg := kernel.Sessions

	quit := quitter(kernel, r)
	settings := &Settings{settings: kernel.Settings}

	done := make(chan error, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		done <- run(r, kernel, opts, quit)
	}()
	t.Cleanup(func() {
		r.Quit()
		<-stopped
	})

	<-r.opened
	shownSession(t, reg)
	waitFor(t, "the session's listener", func() bool { _, serves := boot.counts(); return serves == 1 })

	if err := settings.Relaunch(); err != nil {
		t.Fatal(err)
	}
	if relauncher.calls != 1 {
		t.Fatal("Settings.Relaunch quit without starting a successor")
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	quitReached := r.quit
	r.mu.Unlock()
	if !quitReached {
		t.Fatal("Settings.Relaunch did not reach the renderer's own Quit")
	}
	if _, serves := boot.counts(); serves != 0 {
		t.Fatal("Settings.Relaunch did not stop the session registry")
	}
}

// Catches a run that fails after Start leaving the process socket serving, a
// workbench nothing can quit since its window never opened.
func TestARunWhoseWindowFailsToOpenStopsTheCore(t *testing.T) {
	r := newFakeRenderer()
	r.openErr = errors.New("no display")
	opts, _ := testOptions(t)
	kernel := testWorkbench(t, r, &opts)
	if err := run(r, kernel, opts, quitter(kernel, r)); !errors.Is(err, r.openErr) {
		t.Fatalf("run = %v, want the window's own error", err)
	}
	if workbench.Answered(opts.Socket) {
		t.Fatal("the process socket still answers after run failed")
	}
}

// Catches a run whose Start fails returning while its placement check still
// runs, free to write config.json after the workbench, or its test, has gone.
func TestARunWhoseStartFailsWaitsForThePlacementCheck(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	opts.Socket = filepath.Join(notADir, "qrouton.sock")
	opts.Config = &config.Config{Root: t.TempDir(), Welcomed: true}
	kernel := testWorkbench(t, r, &opts)
	opts.placement = kernel.Placement
	// Shutdown stops sessions before it waits for the check, so a booted one shows it has begun.
	if err := kernel.OpenSession(opts.SessionRoot, false); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the session's listener", func() bool { _, serves := boot.counts(); return serves == 1 })
	held, release := make(chan struct{}), make(chan struct{})
	go func() {
		_ = opts.Config.Transact(func(*config.Config) error { close(held); <-release; return nil })
	}()
	<-held
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)

	done := make(chan error, 1)
	go func() { done <- run(r, kernel, opts, quitter(kernel, r)) }()
	stopping := make(chan struct{})
	go func() {
		defer close(stopping)
		for {
			if _, serves := boot.counts(); serves == 0 {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()
	select {
	case err := <-done:
		t.Fatalf("run returned %v while its placement check was still waiting to write", err)
	case <-stopping:
	}
	unblock()
	if err := <-done; err == nil {
		t.Fatal("run with a socket under a regular file succeeded")
	}
	if file, err := config.ReadFile(); err != nil || file.ThoughtsPlacement == "" {
		t.Fatalf("placement check unfinished when run returned: %+v, %v", file, err)
	}
}
