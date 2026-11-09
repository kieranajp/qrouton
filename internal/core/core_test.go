package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
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
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// fakeClient stands in for the machine the human sits at, recording what the
// core asked of it. Its local actions are unset until a test sets one, which is
// the client that cannot do that thing.
type fakeClient struct {
	mu      sync.Mutex
	events  map[string]any
	shown   []string
	raised  int
	inFront atomic.Bool
	reveal  func(path string) error
	rung    []string
}

func newFakeClient() *fakeClient {
	return &fakeClient{events: map[string]any{}}
}

func (f *fakeClient) Emit(event string, payload any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events[event] = payload
}

func (f *fakeClient) Focused() bool { return f.inFront.Load() }

func (f *fakeClient) Raise() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.raised++
}

func (f *fakeClient) SessionShown(root string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = append(f.shown, root)
}

func (f *fakeClient) Supports(Capability) bool { return false }

func (f *fakeClient) Reveal(path string) error {
	if f.reveal == nil {
		return ErrUnavailable
	}
	return f.reveal(path)
}

func (f *fakeClient) OpenText(string) error { return ErrUnavailable }

func (f *fakeClient) CopyImage(string) error { return ErrUnavailable }

func (f *fakeClient) CopyFile(string) error { return ErrUnavailable }

func (f *fakeClient) ChooseDirectory() (string, error) { return "", ErrUnavailable }

func (f *fakeClient) Chime(sessionRoot string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rung = append(f.rung, sessionRoot)
}

// stubBoot stands in for what a session needs to come up, counting the
// supervisor commands and the listeners asked for and keeping each address. The
// window commands are unset until a test sets one, which is the workbench that
// cannot do that thing.
type stubBoot struct {
	argv     []string
	shell    func(sessionRoot string) []string
	document func(sessionRoot, name string) (workbench.WindowOptions, error)
	// booting runs inside each supervisor request, while the session boots.
	booting func(AgentRequest)

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
	return AgentCommand{Argv: b.argv, Env: os.Environ(), RunnerID: resolved}, nil
}

func (b *stubBoot) Shell(sessionRoot string) []string {
	if b.shell == nil {
		return nil
	}
	return b.shell(sessionRoot)
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

// workbenchOptions is how a client runs a core: the core's own options, the
// process socket it serves, and the session it opens on.
type workbenchOptions struct {
	Options
	SessionRoot string
	Socket      string
	Resume      bool
}

// testOptions is a session the core can open on, under a sessions root it can
// boot others from, on an address of its own.
func testOptions(t *testing.T) (workbenchOptions, *stubBoot) {
	t.Helper()
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(socket) })
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	return workbenchOptions{
		Options:     Options{Root: root, Launcher: boot},
		SessionRoot: sessionDir(t, root, "octopus"),
		Socket:      socket,
	}, boot
}

// testWorkbench builds the core a client runs over r and shuts it down before
// the temporary root goes: a session reaching the screen does not order against
// the stamping and recording that follows it, which would otherwise land in a
// directory the test's cleanup is already removing.
func testWorkbench(t *testing.T, r *fakeClient, opts workbenchOptions) *Core {
	t.Helper()
	c := New(opts.Options, r)
	t.Cleanup(c.Shutdown)
	return c
}

// startWorkbench starts the core on its process socket and opens it on the
// session the options name, as a client does once its window is up.
func startWorkbench(t *testing.T, c *Core, opts workbenchOptions) {
	t.Helper()
	if err := c.Start(opts.Socket); err != nil {
		t.Fatal(err)
	}
	if opts.SessionRoot != "" {
		if err := c.OpenSession(opts.SessionRoot, opts.Resume); err != nil {
			t.Fatal(err)
		}
	}
}

// shutDown is whether the core has stopped serving its process socket, which
// Shutdown is the only way to do.
func shutDown(opts workbenchOptions) bool { return !workbench.Answered(opts.Socket) }

// testSessions is a registry that boots sessions as the core does: a supervisor
// command per session and a listener of its own, over one window registry.
func testSessions(t *testing.T, root string, boot *stubBoot) (*Sessions, *Windows, *fakeClient) {
	t.Helper()
	r := newFakeClient()
	reg := newSessions()
	windows := newWindows(r.Emit, reg)
	t.Cleanup(reg.stopAll)
	t.Cleanup(windows.stopAll)
	reg.boot = booting{
		root:  func(slug string) string { return session.Resumable(root, slug) },
		agent: boot.Agent,
		serve: func(state *SessionState, socket string) (io.Closer, error) {
			boot.served()
			return serveControl(socket, windows, state, controlHooks{
				attention: func(value string, generation uint64) { state.agents.attention(generation, value) },
				generation: func(req workbench.RunnerGenerationRequest) {
					state.agents.begin(req.Provider, req.Generation)
				},
			})
		},
		teardown: windows.stop,
		reveal:   r.Reveal,
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

// shownSession is the session the core put on screen.
func shownSession(t *testing.T, reg *Sessions) *SessionState {
	t.Helper()
	var state *SessionState
	waitFor(t, "the session on screen", func() bool {
		state = reg.Current()
		return state != nil
	})
	return state
}

// endConversation ends a session the way its supervisor does: cat exits zero on
// end-of-transmission.
func endConversation(t *testing.T, term *Term, state *SessionState) {
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

// A supervisor exiting ends its own session and not the app: the window falls
// back to another active session, then starts the next inactive one.
func TestACleanSupervisorExitRetiresOnlyItsSession(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg, term := c.Sessions, c.Term

	startWorkbench(t, c, opts)
	first := shownSession(t, reg)

	sessionDir(t, opts.Root, "kraken")
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	second := reg.Current()
	for _, state := range []*SessionState{first, second} {
		if err := term.Start(state.terminal, 80, 24); err != nil {
			t.Fatal(err)
		}
	}
	socket := boot.socket(t, second.Root())

	endConversation(t, term, second)

	waitFor(t, "the window to fall back to the session shown before it", func() bool {
		return reg.Current() == first
	})
	if _, live := reg.ByTerminal(second.terminal); live {
		t.Fatalf("the retired session %q is still registered", second.Slug())
	}
	if workbench.Answered(socket) {
		t.Fatalf("the retired session still answers on %q", socket)
	}

	endConversation(t, term, first)
	waitFor(t, "the next inactive session to start", func() bool {
		current := reg.Current()
		return current != nil && current != second && current.Slug() == second.Slug()
	})
	if !boot.resumed(t, second.Root()) {
		t.Fatal("the inactive session was restarted without resuming its conversation")
	}
	if shutDown(opts) {
		t.Fatal("the last supervisor exiting took the workbench with it")
	}
}

func TestACleanSupervisorExitLeavesNoSessionWhenItWasTheOnlyOne(t *testing.T) {
	r := newFakeClient()
	opts, _ := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg, term := c.Sessions, c.Term

	startWorkbench(t, c, opts)
	state := shownSession(t, reg)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}

	endConversation(t, term, state)
	waitFor(t, "the window with no session", func() bool { return reg.Current() == nil })
	if shutDown(opts) {
		t.Fatal("the only supervisor exiting took the workbench with it")
	}
}

// Shutdown is what closing the conversation window runs, and every
// session goes with it: no supervisor left running, no listener left answering.
func TestClosingTheConversationWindowTearsDownEverySession(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg, term := c.Sessions, c.Term

	startWorkbench(t, c, opts)
	first := shownSession(t, reg)

	sessionDir(t, opts.Root, "kraken")
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}

	var pids []int
	var sockets []string
	for _, state := range []*SessionState{first, reg.Current()} {
		if err := term.Start(state.terminal, 80, 24); err != nil {
			t.Fatal(err)
		}
		pids = append(pids, ptyPid(state.process))
		sockets = append(sockets, boot.socket(t, state.Root()))
	}

	c.Shutdown()
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
	r := newFakeClient()
	opts, boot := testOptions(t)
	boot.argv = []string{"/bin/sh", "-c", "exit 3"}
	c := testWorkbench(t, r, opts)
	reg, term := c.Sessions, c.Term

	startWorkbench(t, c, opts)
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
	if shutDown(opts) {
		t.Fatal("a failed agent took its window and its error with it")
	}
	waitFor(t, "the pre-generation root failure", func() bool {
		view := state.agents.snapshot()
		return len(view.Records) == 1 && view.Records[0].RunID == agentSetupRunPrefix+"1" &&
			view.Records[0].Provider == "codex" && view.Records[0].State == agentStateFailed
	})
	handle := workbench.Handle{Socket: boot.socket(t, state.Root()), SessionRoot: state.Root()}
	if err := handle.RunnerGeneration(context.Background(), "codex", 42); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the announced run beside the retained setup failure", func() bool {
		view := state.agents.snapshot()
		return len(view.Records) == 2 && view.Running
	})
	c.Shutdown()
}

// The header's document chip is one control for two states: nothing open yet,
// and the window already showing it. A second tab on the same document is the
// failure the user sees.
func TestTheDocumentChipOpensADocumentOnceAndSelectsItAfter(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	boot.shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	boot.document = func(root, name string) (workbench.WindowOptions, error) {
		return workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: filepath.Base(name),
			Source: name, Content: "# " + name, Format: workbench.FormatMarkdown,
		}, nil
	}
	c := testWorkbench(t, r, opts)
	reg, windows := c.Sessions, c.Windows

	startWorkbench(t, c, opts)
	owner := shownSession(t, reg)
	waitFor(t, "the Overview tab", func() bool { return len(windows.surfaces(owner).Tabs) == 1 })

	const doc = "thoughts/shared/research/R7-editor-surfaces.md"
	id, err := windows.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	tabs := windows.surfaces(owner).Tabs
	if len(tabs) != 2 || tabs[1].ID != id {
		t.Fatalf("the document is not the newest tab: %+v", tabs)
	}
	if tabs[1].Label != "R7-editor-surfaces.md" {
		t.Fatalf("the document tab reads %q", tabs[1].Label)
	}
	if tabs[1].Kind != string(workbench.KindDocument) {
		t.Fatalf("the document tab is a %q; a rendered pane runs no process", tabs[1].Kind)
	}
	if selected := windows.Surfaces(owner.Slug()).Selected; selected != id {
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

// The session's thoughts is a symlink out of the session root.
func TestALeadBadgePathOpensItsDocument(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, "shared", "research"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "shared", "research", "R1-x.md"), []byte("# R1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(vault, filepath.Join(opts.SessionRoot, sessionpaths.ThoughtsDirName)); err != nil {
		t.Fatal(err)
	}
	boot.document = func(root, name string) (workbench.WindowOptions, error) {
		return launch.DocumentWindow(root, name, launch.EditorCommand{}, workbench.LineSpan{})
	}
	c := testWorkbench(t, r, opts)
	reg, windows := c.Sessions, c.Windows

	startWorkbench(t, c, opts)
	owner := shownSession(t, reg)
	waitFor(t, "the Overview tab", func() bool { return len(windows.surfaces(owner).Tabs) == 1 })

	written := filepath.Join(owner.Root(), "thoughts", "shared", "research", "R1-x.md")
	documents := writtenDocuments(owner.Root(), []string{written})
	if len(documents) != 1 || documents[0].Path != "thoughts/shared/research/R1-x.md" {
		t.Fatalf("the badge's documents = %+v", documents)
	}
	id, err := windows.OpenDocument(documents[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	tabs := windows.surfaces(owner).Tabs
	if len(tabs) != 2 || tabs[1].ID != id || tabs[1].Kind != string(workbench.KindDocument) {
		t.Fatalf("the badge did not open a document tab: %+v", tabs)
	}
	if selected := windows.Surfaces(owner.Slug()).Selected; selected != id {
		t.Fatalf("the badge selected %q, want %q", selected, id)
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
		Kind: workbench.KindDocument, Label: "old", Source: doc, Content: "old",
	})
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "fresh", Source: doc, Content: "fresh",
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
		Kind: workbench.KindDocument, Label: "P006", Source: doc, Content: "plan",
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
	if err := w.Select(w.shown().Slug(), other); err != nil {
		t.Fatal(err)
	}
	got, err := w.OpenDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	selected := w.Surfaces(w.shown().Slug()).Selected
	if got != id || selected != id {
		t.Fatalf("OpenDocument returned %q and selected %q, want %q", got, selected, id)
	}
}

// Tabs that all read "shell" are tabs nobody can tell apart. The number goes
// in the label the registry stores, so read_window and the manifest's record
// agree with the tab strip.
func TestTheSecondShellOnwardsIsNumbered(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	boot.shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	c := testWorkbench(t, r, opts)
	reg, windows := c.Sessions, c.Windows

	startWorkbench(t, c, opts)
	owner := shownSession(t, reg)
	waitFor(t, "the Overview tab", func() bool { return len(windows.surfaces(owner).Tabs) == 1 })

	for range 3 {
		if _, err := windows.OpenShell(); err != nil {
			t.Fatal(err)
		}
	}
	tabs := windows.surfaces(owner).Tabs
	want := []string{overviewWindowLabel, shellWindowLabel, "shell 2", "shell 3"}
	if len(tabs) != len(want) {
		t.Fatalf("tabs = %+v", tabs)
	}
	for i, label := range want {
		if tabs[i].Label != label {
			t.Fatalf("tab %d reads %q, want %q", i, tabs[i].Label, label)
		}
		icon := workbench.IconShell
		if i == 0 {
			icon = workbench.IconTree
		}
		if tabs[i].Icon != icon {
			t.Fatalf("tab %d draws icon %q, want %q", i, tabs[i].Icon, icon)
		}
		window, ok := windows.window(tabs[i].ID)
		if !ok || window.opts.Label != label {
			t.Fatalf("the registry stores %q for the tab reading %q", window.opts.Label, label)
		}
	}
}

// The manifest is the chrome's only source, so an escalation shows up on the
// next poll rather than needing the window to be told.
func TestChromePushesTheManifestsModePhaseAndName(t *testing.T) {
	r := newFakeClient()
	root := t.TempDir()
	dir := filepath.Join(root, "octopus")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(dir, session.Manifest{Slug: "octopus", Name: "Octopus", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}

	reg := testRegistry(t, dir)
	state := reg.Current()
	pushChrome(reg, root, nil, nil, nil, nil, r.Emit)

	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed at the window: %v", r.events)
	}
	if fields.Mode == "" || fields.Identity != "Octopus" {
		t.Fatalf("chrome = %+v, want the session's mode and name", fields)
	}
	if fields.Slug != "octopus" || fields.Terminal != state.terminal {
		t.Fatalf("chrome names session %q on terminal %q", fields.Slug, fields.Terminal)
	}
}

// welcomingFor pushes one payload and reports whether the window is asking the
// first-run questions.
func welcomingFor(t *testing.T, reg *Sessions, root string, cfg *config.Config) bool {
	t.Helper()
	r := newFakeClient()
	pushChrome(reg, root, cfg, nil, nil, nil, r.Emit)
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
	r := newFakeClient()
	root := t.TempDir()
	for _, slug := range []string{"octopus", "kraken"} {
		dir := sessionDir(t, root, slug)
		if err := session.WriteManifest(dir, session.Manifest{Slug: slug, Name: slug}); err != nil {
			t.Fatal(err)
		}
	}

	reg := testRegistry(t, filepath.Join(root, "octopus"))
	pushChrome(reg, root, nil, nil, nil, nil, r.Emit)

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
	if err := handle.RunnerGeneration(context.Background(), agentProviderCodex, 1); err != nil {
		t.Fatal(err)
	}
	if err := handle.Attention(context.Background(), status.ActivityWaiting, 1); err != nil {
		t.Fatal(err)
	}
	pushChrome(reg, root, nil, nil, nil, nil, r.Emit)

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
	r := newFakeClient()
	root := t.TempDir()
	shown := sessionDir(t, root, "octopus")
	background := sessionDir(t, root, "kraken")
	if err := session.WriteManifest(background, session.Manifest{Slug: "kraken", Mode: session.ModeAssistant,
		Repos: []session.ManifestRepo{{Name: "api", Org: "lifesum"}}}); err != nil {
		t.Fatal(err)
	}

	measured := status.RepoStat{Name: "lifesum/svc", Role: "editing", Commits: 3, Measured: true}
	pushChrome(testRegistry(t, shown), root, nil, map[string][]status.RepoStat{shown: {measured}}, nil, nil, r.Emit)

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
func pushedChrome(t *testing.T, r *fakeClient) status.Fields {
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
	if empty.Sessions == nil || empty.Documents == nil || empty.RepositoryDocuments == nil || empty.Repos == nil {
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
	r := newFakeClient()
	root := t.TempDir()
	first := sessionDir(t, root, "octopus")
	second := sessionDir(t, root, "kraken")
	measured := map[string][]status.RepoStat{
		first: {{Name: "lifesum/svc", Role: "editing", Commits: 3, Measured: true}},
	}

	reg := testRegistry(t, first)
	pushChrome(reg, root, nil, measured, nil, nil, r.Emit)
	if repos := pushedChrome(t, r).Repos; len(repos) != 1 || repos[0].Name != "lifesum/svc" {
		t.Fatalf("chrome repos = %+v, want what was measured for the session on screen", repos)
	}

	reg.reveal(reg.add(second, []string{"/bin/cat"}, os.Environ()))
	pushChrome(reg, root, nil, measured, nil, nil, r.Emit)
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
	go watch(ctx, reg, root, nil, emit, time.Millisecond, time.Hour, time.Hour, counts, noPulls)

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
	r := newFakeClient()
	opts, _ := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg := c.Sessions

	startWorkbench(t, c, opts)
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
	booted := reg.Current()
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
	if again := reg.Current(); again != booted {
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
	if state := reg.Current(); state == nil || state.Slug() != "octopus" {
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

// A supervisor that fails leaves its state registered, and a pane drawn against
// a dead process swallows every keystroke. The row goes back to unloaded with
// the session's tabs intact, and showing it again boots a replacement.
func TestADeadSupervisorUnloadsItsRowAndShowBootsAReplacement(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/sh", "-c", "exit 3")
	reg, windows, r := testSessions(t, root, boot)
	sessionDir(t, root, "octopus")

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	dead := reg.Current()
	tab, err := windows.openStructural(dead, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Command: []string{"/bin/cat"}, Cwd: dead.Root(),
	})
	if err != nil {
		t.Fatal(err)
	}
	term := newTerm(reg, r.Emit)
	if err := term.Start(dead.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the supervisor to fail", func() bool { return !dead.alive() })

	pushChrome(reg, root, nil, nil, nil, nil, r.Emit)
	crashed := pushedChrome(t, r)
	if crashed.Terminal != "" {
		t.Fatalf("the session on screen still names terminal %q after its supervisor died", crashed.Terminal)
	}
	if got := rowTerminal(t, crashed, "octopus"); got != "" {
		t.Fatalf("the row still names terminal %q after its supervisor died", got)
	}
	if tabs := windows.surfaces(dead).Tabs; len(tabs) != 1 || tabs[0].ID != tab {
		t.Fatalf("the crash took the session's tabs with it: %+v", tabs)
	}

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	booted := reg.Current()
	if booted == dead || booted.terminal == dead.terminal {
		t.Fatalf("showing the dead session came back with terminal %q", booted.terminal)
	}
	if !boot.resumed(t, booted.Root()) {
		t.Fatal("the replacement supervisor started a fresh conversation")
	}
	if tabs := windows.surfaces(dead).Tabs; len(tabs) != 0 {
		t.Fatalf("the reboot left the dead session's tabs open: %+v", tabs)
	}
	pushChrome(reg, root, nil, nil, nil, nil, r.Emit)
	if got := rowTerminal(t, pushedChrome(t, r), "octopus"); got != booted.terminal {
		t.Fatalf("the row names terminal %q, want the session just booted %q", got, booted.terminal)
	}
}

// Reload is the way out of a session that is wedged rather than dead, so it
// tears down a live supervisor and comes back on the same conversation.
func TestReloadReplacesALiveSupervisorAndResumesTheConversation(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, r := testSessions(t, root, boot)
	sessionDir(t, root, "octopus")

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	before := reg.Current()
	term := newTerm(reg, r.Emit)
	if err := term.Start(before.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	pid := ptyPid(before.process)

	if err := reg.Reload("octopus"); err != nil {
		t.Fatal(err)
	}
	after := reg.Current()
	if after == before || after.terminal == before.terminal {
		t.Fatalf("Reload came back with terminal %q, want a session of its own", after.terminal)
	}
	if state, ok := reg.ByTerminal(before.terminal); ok {
		t.Fatalf("the reloaded session %q is still registered", state.terminal)
	}
	waitFor(t, "the previous supervisor to die", func() bool { return syscall.Kill(pid, 0) != nil })
	if !boot.resumed(t, after.Root()) {
		t.Fatal("the reloaded session started a fresh conversation")
	}
	if agents, _ := boot.counts(); agents != 2 {
		t.Fatalf("Reload asked for %d supervisors in total, want the first and its replacement", agents)
	}
}

// A reboot that cannot start leaves the window on the session before it. An
// empty window is the assembly overlay, and at its first step there is no way out.
func TestARebootThatCannotStartFallsBackToThePreviousSession(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/sh", "-c", "exit 3")
	reg, _, r := testSessions(t, root, boot)
	for _, slug := range []string{"kraken", "octopus"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
	}
	previous := reg.BySlug("kraken")
	dead := reg.Current()
	term := newTerm(reg, r.Emit)
	if err := term.Start(dead.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the supervisor to fail", func() bool { return !dead.alive() })

	reg.boot.agent = func(AgentRequest) (AgentCommand, error) { return AgentCommand{}, ErrNoSession }
	if err := reg.Show("octopus"); err == nil {
		t.Fatal("Show reported a reboot that never happened")
	}
	if state := reg.Current(); state != previous {
		t.Fatalf("the window fell back to %q, want the session shown before the one that failed", state.Slug())
	}
}

func TestUnloadStopsOwnedResourcesAndRetainsAResumableRow(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, windows, r := testSessions(t, root, boot)
	dir := sessionDir(t, root, "octopus")
	marker := filepath.Join(dir, "conversation.txt")
	if err := os.WriteFile(marker, []byte("retained"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	state := reg.Current()
	term := newTerm(reg, r.Emit)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	pid := ptyPid(state.process)
	socket := boot.socket(t, dir)
	tab, err := windows.openStructural(state, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Cwd: dir, Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.Start(tab, 80, 24); err != nil {
		t.Fatal(err)
	}
	shell, err := windows.terminals.process(tab)
	if err != nil {
		t.Fatal(err)
	}
	shellPID := ptyPid(shell)
	state.agents.output()
	reg.railOrder(polled("octopus"))
	for len(reg.touched) > 0 {
		<-reg.touched
	}
	if err := reg.Unload("octopus"); err != nil {
		t.Fatal(err)
	}
	if reg.Current() != nil || reg.BySlug("octopus") != nil {
		t.Fatal("the unloaded conversation is still selected or registered")
	}
	if _, ok := reg.ByTerminal(state.terminal); ok {
		t.Fatal("the unloaded terminal is still registered")
	}
	if _, ok := reg.agentActivitySnapshots()["octopus"]; ok {
		t.Fatal("the unloaded row retained its live activity")
	}
	if len(reg.touched) == 0 {
		t.Fatal("Unload did not wake chrome")
	}
	waitFor(t, "the conversation and shell to end", func() bool {
		return syscall.Kill(pid, 0) != nil && syscall.Kill(shellPID, 0) != nil
	})
	if workbench.Answered(socket) || len(windows.Surfaces("octopus").Tabs) != 0 {
		t.Fatal("the unloaded session retained its listener or tabs")
	}
	if _, err := os.Stat(sessionpaths.Manifest(dir)); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(marker); err != nil || string(content) != "retained" {
		t.Fatalf("Unload changed session files: %q, %v", content, err)
	}
	pushChrome(reg, root, nil, nil, nil, nil, r.Emit)
	fields := pushedChrome(t, r)
	if len(fields.Sessions) != 1 || fields.Slug != "" || fields.Terminal != "" ||
		rowTerminal(t, fields, "octopus") != "" || fields.Sessions[0].Activity != "" {
		t.Fatalf("Unload did not leave a dimmed row and empty state: %+v", fields)
	}
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	if reg.Current() == state || !boot.resumed(t, dir) {
		t.Fatal("Show did not resume a replacement conversation")
	}
	if got := railSlugs(reg, polled("octopus")); !reflect.DeepEqual(got, []string{"octopus"}) {
		t.Fatalf("Unload changed the rail: %v", got)
	}
}

func TestAcceptedWindowOpenCannotOutliveSessionUnload(t *testing.T) {
	for _, kind := range []workbench.WindowKind{workbench.KindTerminal, workbench.KindDocument} {
		t.Run(string(kind), func(t *testing.T) {
			root := t.TempDir()
			reg, windows, _ := testSessions(t, root, newStubBoot("/bin/cat"))
			dir := sessionDir(t, root, "octopus")
			if err := reg.Show("octopus"); err != nil {
				t.Fatal(err)
			}
			owner := reg.Current()
			accepted, client := net.Pipe()
			defer client.Close()
			if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			control := &control{windows: windows, owner: owner}
			go control.handle(accepted)
			if err := reg.Unload("octopus"); err != nil {
				t.Fatal(err)
			}
			options := workbench.WindowOptions{Kind: kind, Cwd: dir, Content: "retained"}
			if kind == workbench.KindTerminal {
				options.Command = []string{"/bin/cat"}
			}
			if err := json.NewEncoder(client).Encode(workbench.Request{Op: workbench.OpOpen, Options: &options}); err != nil {
				t.Fatal(err)
			}
			var response workbench.Response
			if err := json.NewDecoder(client).Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.Error != ErrSessionUnloaded.Error() || response.ID != "" || len(windows.list()) != 0 {
				t.Fatalf("open after unload = %+v, tabs %v", response, windows.list())
			}
			if id, err := windows.openStructural(owner, options); !errors.Is(err, ErrSessionUnloaded) || id != "" {
				t.Fatalf("structural open for unloaded owner = %q, %v", id, err)
			}
			if err := reg.Show("octopus"); err != nil {
				t.Fatal(err)
			}
			replacement := reg.Current()
			if replacement == owner {
				t.Fatal("Show reused the unloaded owner")
			}
			if id, err := windows.openStructural(replacement, options); err != nil || id == "" {
				t.Fatalf("structural open for replacement = %q, %v", id, err)
			}
			if len(windows.list()) != 1 || len(windows.surfaces(owner).Tabs) != 0 {
				t.Fatal("a delayed open left tabs owned by the unloaded session")
			}
		})
	}
}

func TestRegistryShutdownRefusesOrdinaryAndStructuralAdmissions(t *testing.T) {
	root := t.TempDir()
	reg, windows, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	sessionDir(t, root, "octopus")
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	owner := reg.Current()
	windows.stopAll()
	for _, kind := range []workbench.WindowKind{workbench.KindTerminal, workbench.KindDocument, workbench.KindOverview} {
		options := workbench.WindowOptions{Kind: kind, Cwd: owner.Root()}
		if kind == workbench.KindTerminal {
			options.Command = []string{"/bin/cat"}
		}
		if kind != workbench.KindOverview {
			if id, err := windows.openWindow(owner, options); !errors.Is(err, ErrSessionUnloaded) || id != "" {
				t.Fatalf("%s open after shutdown = %q, %v", kind, id, err)
			}
		}
		if id, err := windows.openStructural(owner, options); !errors.Is(err, ErrSessionUnloaded) || id != "" {
			t.Fatalf("%s structural open after shutdown = %q, %v", kind, id, err)
		}
	}
	if len(windows.list()) != 0 {
		t.Fatalf("shutdown admitted tabs: %v", windows.list())
	}
}

func TestRepeatedSessionTeardownRefusesLateAdmissionsWithoutRetainingOwners(t *testing.T) {
	root := t.TempDir()
	reg, windows, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	dir := sessionDir(t, root, "octopus")
	options := workbench.WindowOptions{Kind: workbench.KindTerminal, Cwd: dir, Command: []string{"/bin/cat"}, Select: true}
	for i := 0; i < 20; i++ {
		if err := reg.Show("octopus"); err != nil {
			t.Fatal(err)
		}
		owner := reg.Current()
		if id, err := windows.openWindow(owner, options); err != nil || id == "" {
			t.Fatalf("fresh owner admission = %q, %v", id, err)
		}
		retainedHistory := reg.history
		if err := reg.Unload("octopus"); err != nil {
			t.Fatal(err)
		}
		for _, entry := range retainedHistory {
			if entry != nil {
				t.Fatal("history retained the unloaded owner beyond its length")
			}
		}
		windows.stop(owner)
		if id, err := windows.openStructural(owner, options); !errors.Is(err, ErrSessionUnloaded) || id != "" {
			t.Fatalf("late admission = %q, %v", id, err)
		}
		if !owner.admissionClosed() || len(windows.list()) != 0 || len(windows.selected) != 0 || len(reg.all()) != 0 {
			t.Fatal("teardown left registrations behind")
		}
	}
}

func TestSessionRemovalSkipsDeadAndObsoleteHistoryFallbacks(t *testing.T) {
	for _, action := range []string{"unload", "reload-failure", "retire"} {
		for _, loadedFallback := range []bool{false, true} {
			t.Run(action+"/"+strconv.FormatBool(loadedFallback), func(t *testing.T) {
				root := t.TempDir()
				reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
				for _, slug := range []string{"live", "dead", "obsolete", "shown"} {
					sessionDir(t, root, slug)
				}
				if loadedFallback {
					if err := reg.Show("live"); err != nil {
						t.Fatal(err)
					}
				}
				if err := reg.Show("dead"); err != nil {
					t.Fatal(err)
				}
				dead := reg.Current()
				finished := exitedPTY(t)
				dead.mu.Lock()
				dead.process = finished
				dead.mu.Unlock()
				if err := reg.Show("obsolete"); err != nil {
					t.Fatal(err)
				}
				obsolete := reg.Current()
				t.Cleanup(obsolete.stop)
				reg.mu.Lock()
				delete(reg.slugs, obsolete.Slug())
				delete(reg.terms, obsolete.terminal)
				reg.mu.Unlock()
				if err := reg.Show("shown"); err != nil {
					t.Fatal(err)
				}
				switch action {
				case "unload":
					if err := reg.Unload("shown"); err != nil {
						t.Fatal(err)
					}
				case "reload-failure":
					reg.boot.agent = func(AgentRequest) (AgentCommand, error) { return AgentCommand{}, errNoAgent }
					if err := reg.Reload("shown"); !errors.Is(err, errNoAgent) {
						t.Fatalf("reload failure = %v", err)
					}
				case "retire":
					reg.retire(reg.Current())
				}
				if reg.Current() != reg.BySlug("live") {
					t.Fatalf("%s selected %q, want the alive registered fallback or none", action, reg.Current().Slug())
				}
			})
		}
	}
}

func TestUnloadBackgroundPreservesSelectionAndHistory(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	for _, slug := range []string{"octopus", "kraken", "webhook"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
	}
	shown := reg.Current()
	previous := reg.BySlug("octopus")
	if err := reg.Unload("kraken"); err != nil {
		t.Fatal(err)
	}
	if reg.Current() != shown || !reflect.DeepEqual(reg.history, []*SessionState{previous, shown}) {
		t.Fatal("unloading a background session changed selection or remaining history")
	}
	if err := reg.Unload("webhook"); err != nil {
		t.Fatal(err)
	}
	if reg.Current() != previous {
		t.Fatal("Unload did not return to the most recently shown loaded session")
	}
}

func TestUnloadSkipsDeadFallbacksAndNeverBootsInactiveRows(t *testing.T) {
	for _, loadedFallback := range []bool{false, true} {
		t.Run(strconv.FormatBool(loadedFallback), func(t *testing.T) {
			root := t.TempDir()
			boot := newStubBoot("/bin/sh", "-c", "exit 3")
			reg, _, r := testSessions(t, root, boot)
			for _, slug := range []string{"live", "dead", "shown", "inactive"} {
				sessionDir(t, root, slug)
			}
			if loadedFallback {
				if err := reg.Show("live"); err != nil {
					t.Fatal(err)
				}
			}
			if err := reg.Show("dead"); err != nil {
				t.Fatal(err)
			}
			dead := reg.Current()
			if err := newTerm(reg, r.Emit).Start(dead.terminal, 80, 24); err != nil {
				t.Fatal(err)
			}
			waitFor(t, "the fallback supervisor to fail", func() bool { return !dead.alive() })
			if err := reg.Show("shown"); err != nil {
				t.Fatal(err)
			}
			reg.railOrder(polled("shown", "inactive", "dead", "live"))
			agents, serves := boot.counts()
			if err := reg.Unload("shown"); err != nil {
				t.Fatal(err)
			}
			if reg.Current() != reg.BySlug("live") {
				t.Fatalf("Unload selected %q, want the alive loaded fallback or none", reg.Current().Slug())
			}
			if afterAgents, afterServes := boot.counts(); afterAgents != agents || afterServes != serves {
				t.Fatal("Unload booted a fallback")
			}
		})
	}
}

func TestUnloadInactiveIsANoopAndUnknownSlugsAreRefused(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	sessionDir(t, root, "shown")
	sessionDir(t, root, "inactive")
	if err := reg.Show("shown"); err != nil {
		t.Fatal(err)
	}
	shown := reg.Current()
	for len(reg.touched) > 0 {
		<-reg.touched
	}
	if err := reg.Unload("inactive"); err != nil {
		t.Fatal(err)
	}
	if reg.Current() != shown || len(reg.touched) != 0 {
		t.Fatal("unloading an inactive session changed live state")
	}
	for _, slug := range []string{"unknown", "", ".", "..", "../shown", "shown/../inactive"} {
		if err := reg.Unload(slug); err == nil {
			t.Fatalf("Unload accepted unknown or unsafe slug %q", slug)
		}
	}
	if agents, serves := boot.counts(); agents != 1 || serves != 1 {
		t.Fatal("an inactive or unknown Unload booted a session")
	}
}

func TestDelayedRetirementCannotRemoveAReopenedSession(t *testing.T) {
	root := t.TempDir()
	reg, windows, r := testSessions(t, root, newStubBoot("/bin/sh", "-c", "exit 0"))
	sessionDir(t, root, "octopus")
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	old := reg.Current()
	arrived, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	term := newTerm(reg, r.Emit)
	term.whenChildExits(func(state *SessionState, code int) {
		close(arrived)
		<-release
		reg.retire(state)
		close(finished)
	})
	if err := term.Start(old.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("the supervisor exit callback did not arrive")
	}
	if err := reg.Unload("octopus"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	replacement := reg.Current()
	tab, err := windows.openStructural(replacement, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Cwd: replacement.Root(), Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the delayed retirement did not finish")
	}
	if reg.Current() != replacement || reg.BySlug("octopus") != replacement || !replacement.alive() {
		t.Fatal("a delayed retirement removed or stopped the replacement")
	}
	if _, ok := reg.ByTerminal(replacement.terminal); !ok {
		t.Fatal("a delayed retirement lost the replacement's terminal or tabs")
	}
	if tabs := windows.Surfaces("octopus").Tabs; len(tabs) != 1 || tabs[0].ID != tab {
		t.Fatalf("replacement tabs = %+v, want %q", tabs, tab)
	}
}

func TestRetirementOfAnObsoleteStateLeavesItsReplacementRegistered(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	dir := sessionDir(t, root, "octopus")
	old := reg.add(dir, []string{"/bin/cat"}, os.Environ())
	reg.forget(old)
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	replacement := reg.Current()
	reg.retire(old)
	if reg.Current() != replacement || reg.BySlug("octopus") != replacement || !replacement.alive() {
		t.Fatal("retirement of an obsolete state changed its replacement")
	}
	if !workbench.Answered(boot.socket(t, dir)) {
		t.Fatal("retirement closed the replacement's listener")
	}
}

func TestAnIntentionallyStoppedChildDoesNotReportItsExit(t *testing.T) {
	root := t.TempDir()
	reg, _, r := testSessions(t, root, newStubBoot("/bin/cat"))
	sessionDir(t, root, "octopus")
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	old := reg.Current()
	if err := reg.Unload("octopus"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	replacement := reg.Current()
	old.childExited(r.Emit, func(*SessionState, int) {
		t.Fatal("an intentional stop invoked the child-exit handler")
	}, 0)
	r.mu.Lock()
	_, emitted := r.events[ptyExitEvent+old.terminal]
	r.mu.Unlock()
	if emitted || reg.Current() != replacement {
		t.Fatal("an intentional stop emitted an exit or changed selection")
	}
}

func TestReloadRefusesASlugItCannotResolve(t *testing.T) {
	reg, _, _ := testSessions(t, "", newStubBoot("/bin/cat"))
	err := reg.Reload("octopus")
	if err == nil {
		t.Fatal("Reload booted a session with no directory behind it")
	}
	if !strings.Contains(err.Error(), "octopus") {
		t.Fatalf("refusal %q does not name the session asked for", err)
	}
}

func rowTerminal(t *testing.T, fields status.Fields, slug string) string {
	t.Helper()
	for _, row := range fields.Sessions {
		if row.Slug == slug {
			return row.Terminal
		}
	}
	t.Fatalf("no row for %q in %+v", slug, fields.Sessions)
	return ""
}

func TestCycleStickerPersistsAndTouchesChromeOnlyAfterSuccess(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	dir := sessionDir(t, root, "octopus-0192")
	reg.boot.stickers = func() []session.Sticker { return []session.Sticker{"a1b2c3d4", "star"} }

	if got, err := reg.CycleSticker("octopus-0192"); err != nil || got != "a1b2c3d4" {
		t.Fatalf("CycleSticker() = %q, %v", got, err)
	}
	select {
	case <-reg.touched:
	case <-time.After(time.Second):
		t.Fatal("successful sticker cycle did not touch chrome")
	}
	loaded, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Sticker != "a1b2c3d4" {
		t.Fatalf("persisted sticker = %q", loaded.Sticker)
	}

	if _, err := reg.CycleSticker("missing"); err == nil {
		t.Fatal("CycleSticker accepted an unknown session")
	}
	select {
	case <-reg.touched:
		t.Fatal("failed sticker cycle touched chrome")
	default:
	}
}

func TestCycleStickerRejectsUnsafeSessionIdentifiers(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "sessions")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	valid := sessionDir(t, root, "octopus")
	escaped := filepath.Join(workspace, "escaped")
	if err := os.Mkdir(escaped, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(escaped, session.Manifest{Slug: "../escaped"}); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "nested", "child")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(nested, session.Manifest{Slug: "nested/child"}); err != nil {
		t.Fatal(err)
	}

	for _, slug := range []string{
		"", ".", "..", "../escaped", `..\\escaped`, "nested/child", `nested\\child`,
	} {
		t.Run(slug, func(t *testing.T) {
			if _, err := reg.CycleSticker(slug); err == nil {
				t.Fatalf("CycleSticker(%q) accepted an unsafe identifier", slug)
			}
		})
	}
	select {
	case <-reg.touched:
		t.Fatal("refused sticker cycle touched chrome")
	default:
	}
	for _, dir := range []string{valid, escaped, nested} {
		m, err := session.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if m.Sticker != "" {
			t.Fatalf("refused sticker cycle changed %q to %q", dir, m.Sticker)
		}
	}
}

func TestCycleStickerAcceptsExistingDirectChildSessionIdentifiers(t *testing.T) {
	for _, slug := range []string{"Octopus", "octopus_0192", "octopus--0192", "-octopus", "octopus-"} {
		t.Run(slug, func(t *testing.T) {
			root := t.TempDir()
			reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
			dir := sessionDir(t, root, slug)
			reg.boot.stickers = func() []session.Sticker { return []session.Sticker{"star"} }

			got, err := reg.CycleSticker(slug)
			if err != nil {
				t.Fatal(err)
			}
			if got != session.Sticker("star") {
				t.Fatalf("CycleSticker(%q) = %q, want %q", slug, got, session.Sticker("star"))
			}
			manifest, err := session.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if manifest.Slug != slug || manifest.Sticker != session.Sticker("star") {
				t.Fatalf("manifest = slug %q, sticker %q", manifest.Slug, manifest.Sticker)
			}
		})
	}
}

func TestCycleStickerRejectsASymlinkOutsideTheSessionsRoot(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "sessions")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	escaped := filepath.Join(workspace, "octopus")
	if err := os.Mkdir(escaped, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteManifest(escaped, session.Manifest{Slug: "octopus"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(escaped, filepath.Join(root, "octopus")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))

	if _, err := reg.CycleSticker("octopus"); err == nil {
		t.Fatal("CycleSticker accepted a session symlink outside the sessions root")
	}
	m, err := session.Load(escaped)
	if err != nil {
		t.Fatal(err)
	}
	if m.Sticker != "" {
		t.Fatalf("refused sticker cycle changed the escaped manifest to %q", m.Sticker)
	}
	select {
	case <-reg.touched:
		t.Fatal("refused sticker cycle touched chrome")
	default:
	}
}

func TestCycleStickerRejectsAMismatchedManifestSlug(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	dir := sessionDir(t, root, "octopus")
	if err := session.UpdateManifest(dir, func(m session.Manifest) (session.Manifest, error) {
		m.Slug = "kraken"
		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := reg.CycleSticker("octopus"); err == nil {
		t.Fatal("CycleSticker accepted a manifest for another session")
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Slug != "kraken" || m.Sticker != "" {
		t.Fatalf("refused sticker cycle changed the manifest to slug %q, sticker %q", m.Slug, m.Sticker)
	}
	select {
	case <-reg.touched:
		t.Fatal("refused sticker cycle touched chrome")
	default:
	}
}

func TestConcurrentStickerCyclesCommitEachCall(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	dir := sessionDir(t, root, "octopus")

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := reg.CycleSticker("octopus")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Sticker != "" {
		t.Fatalf("10 cycles left sticker %q, want none", loaded.Sticker)
	}
}

// Two sessions are two agents on two addresses. Sharing either is two rail rows
// drawing one conversation.
func TestTwoSessionsGetTheirOwnSocketAndSupervisor(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, r := testSessions(t, root, boot)
	term := newTerm(reg, r.Emit)

	booted := map[string]*SessionState{}
	for _, slug := range []string{"alpha", "beta"} {
		sessionDir(t, root, slug)
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
		state := reg.Current()
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
	if ptyPid(alpha.process) == ptyPid(beta.process) {
		t.Fatal("two sessions share one supervisor")
	}
	if first, second := boot.socket(t, alpha.Root()), boot.socket(t, beta.Root()); first == second {
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
		Kind: workbench.KindTerminal, Label: "alpha dev", Cwd: alpha, Command: []string{"/bin/cat"},
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
	r := newFakeClient()
	opts, _ := testOptions(t)
	adopted := sessionDir(t, opts.Root, "kraken")
	c := testWorkbench(t, r, opts)
	reg := c.Sessions

	startWorkbench(t, c, opts)
	shownSession(t, reg)

	if !workbench.Answered(opts.Socket) {
		t.Fatal("the process socket does not answer the launcher's readiness poll")
	}
	host := (workbench.Handle{Socket: opts.Socket, SessionRoot: opts.SessionRoot}).WindowHost()
	ctx := context.Background()
	if _, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "dev", Cwd: opts.SessionRoot, Command: []string{"/bin/cat"},
	}); err == nil {
		t.Fatal("the process socket opened a window with no session to own it")
	} else if errors.Is(err, workbench.ErrWorkbenchUnreachable) {
		t.Fatalf("the refusal arrived as a transport failure: %v", err)
	} else if err.Error() != ErrNoSession.Error() {
		t.Fatalf("refusal = %q, want %q", err, ErrNoSession)
	}

	if err := reg.Adopt(adopted, ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the adopted session on screen", func() bool {
		state := reg.Current()
		return state != nil && state.Slug() == "kraken"
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
	if state := reg.Current(); state == nil || state.Slug() != "crashed" {
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
	r := newFakeClient()
	opts, boot := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg := c.Sessions

	startWorkbench(t, c, opts)
	shownSession(t, reg)

	ghost := filepath.Join(opts.Root, "ghost")
	if err := os.MkdirAll(ghost, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("ghost"); err == nil {
		t.Fatal("a directory with no manifest booted; a session without one must not appear resumable")
	}
	if reg.BySlug("ghost") != nil {
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
	r := newFakeClient()

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
			serve: func(state *SessionState, socket string) (io.Closer, error) {
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
	r := newFakeClient()
	opts, boot := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg := c.Sessions

	startWorkbench(t, c, opts)
	shownSession(t, reg)

	assembled := sessionDir(t, opts.Root, "kraken")
	if err := reg.Adopt(assembled, ""); err != nil {
		t.Fatal(err)
	}

	waitFor(t, "the adopted session on screen", func() bool {
		state := reg.Current()
		return state != nil && state.Slug() == "kraken"
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
	r := newFakeClient()
	opts, _ := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg := c.Sessions

	sessionDir(t, opts.Root, "kraken")
	// The chrome poller freezes the rail on whichever poll gets there first, so
	// the order under test is the stamps' and not that poll's timing.
	if err := session.MarkOpened(opts.SessionRoot, time.Now()); err != nil {
		t.Fatal(err)
	}
	startWorkbench(t, c, opts)
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

func TestReorderMovesSessionsToTheTargetsFormerPosition(t *testing.T) {
	for _, test := range []struct {
		name, source, target string
		want                 []string
	}{
		{"upward", "webhook", "kraken", []string{"webhook", "kraken", "octopus", "clam"}},
		{"downward", "kraken", "webhook", []string{"octopus", "webhook", "kraken", "clam"}},
		{"first-to-last", "kraken", "clam", []string{"octopus", "webhook", "clam", "kraken"}},
		{"last-to-first", "clam", "kraken", []string{"clam", "kraken", "octopus", "webhook"}},
		{"same-row", "octopus", "octopus", []string{"kraken", "octopus", "webhook", "clam"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
			rows := polled("kraken", "octopus", "webhook", "clam")
			for _, row := range rows {
				sessionDir(t, root, row.Slug)
			}
			reg.railOrder(rows)
			if err := reg.Reorder(test.source, test.target); err != nil {
				t.Fatal(err)
			}
			if got := railSlugs(reg, rows); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("reordered rail = %v, want %v", got, test.want)
			}
			if touched := len(reg.touched) != 0; touched != (test.source != test.target) {
				t.Fatalf("chrome wakeup = %v for %s", touched, test.name)
			}
		})
	}
}

func TestReorderRefusesUnknownAndUnsafeRowsWithoutChangingOrder(t *testing.T) {
	reg := newSessions()
	initial := []string{"kraken", "gone", "octopus"}
	reg.railOrder(polled(initial...))
	for _, bad := range []string{"unknown", "unpolled", "", ".", "..", "kraken/../octopus", "../octopus", `kraken\octopus`} {
		for _, pair := range [][2]string{{bad, "kraken"}, {"kraken", bad}, {bad, bad}} {
			if err := reg.Reorder(pair[0], pair[1]); err == nil {
				t.Fatalf("Reorder accepted %q onto %q", pair[0], pair[1])
			}
			if !reflect.DeepEqual(reg.rail, initial) || len(reg.touched) != 0 {
				t.Fatalf("refused reorder changed state: rail %v, wakeups %d", reg.rail, len(reg.touched))
			}
		}
	}
}

func TestReorderUsesKnownRowsWhileManifestsAreBeingWritten(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	source := sessionDir(t, root, "kraken")
	target := sessionDir(t, root, "octopus")
	reg.railOrder(polled("kraken", "octopus"))
	if err := os.Remove(sessionpaths.Manifest(source)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.Manifest(target), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg.boot.root = func(string) string {
		t.Fatal("Reorder read session files")
		return ""
	}
	if err := reg.Reorder("kraken", "octopus"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reg.rail, []string{"octopus", "kraken"}) {
		t.Fatalf("manifest writes blocked reordering: %v", reg.rail)
	}
}

func TestReorderRetainsMissingEntriesAndSurvivesPollsAndFreshSessions(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	for _, slug := range []string{"kraken", "octopus", "webhook", "fresh"} {
		sessionDir(t, root, slug)
	}
	reg.railOrder(polled("kraken", "missing", "octopus", "webhook"))
	if err := reg.Reorder("kraken", "webhook"); err != nil {
		t.Fatal(err)
	}
	want := []string{"missing", "octopus", "webhook", "kraken"}
	if !reflect.DeepEqual(reg.rail, want) {
		t.Fatalf("reorder changed missing entries: %v, want %v", reg.rail, want)
	}
	for _, rows := range [][]status.SessionRow{
		polled("kraken", "octopus", "webhook"),
		polled("webhook", "kraken", "octopus"),
	} {
		if got := railSlugs(reg, rows); !reflect.DeepEqual(got, []string{"octopus", "webhook", "kraken"}) {
			t.Fatalf("poll lost the reordered sequence: %v", got)
		}
	}
	if got := railSlugs(reg, polled("fresh", "kraken", "webhook", "octopus")); !reflect.DeepEqual(got, []string{"fresh", "octopus", "webhook", "kraken"}) {
		t.Fatalf("fresh session did not prepend to the user order: %v", got)
	}
	sessionDir(t, root, "missing")
	if got := railSlugs(reg, polled("missing", "fresh", "kraken", "webhook", "octopus")); !reflect.DeepEqual(got, []string{"fresh", "missing", "octopus", "webhook", "kraken"}) {
		t.Fatalf("returning session lost its retained place: %v", got)
	}
}

func TestReorderPreservesSelectionHistoryAndSessionLifecycleOrder(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	for _, slug := range []string{"kraken", "octopus", "inactive"} {
		sessionDir(t, root, slug)
	}
	reg.railOrder(polled("kraken", "octopus", "inactive"))
	for _, slug := range []string{"kraken", "octopus"} {
		if err := reg.Show(slug); err != nil {
			t.Fatal(err)
		}
	}
	shown := reg.Current()
	history := append([]*SessionState(nil), reg.history...)
	agents, serves := boot.counts()
	if err := reg.Reorder("inactive", "kraken"); err != nil {
		t.Fatal(err)
	}
	if reg.Current() != shown || !reflect.DeepEqual(reg.history, history) || reg.BySlug("inactive") != nil {
		t.Fatal("reordering an inactive row changed selection, history or loaded it")
	}
	if afterAgents, afterServes := boot.counts(); afterAgents != agents || afterServes != serves {
		t.Fatal("Reorder booted a session")
	}
	for _, action := range []func(string) error{reg.Show, reg.Unload, reg.Reload} {
		if err := action("kraken"); err != nil {
			t.Fatal(err)
		}
		if got := railSlugs(reg, polled("kraken", "octopus", "inactive")); !reflect.DeepEqual(got, []string{"inactive", "kraken", "octopus"}) {
			t.Fatalf("session lifecycle changed user order: %v", got)
		}
	}
}

func TestConcurrentRailPollingAndReorderingPreserveEveryEntry(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	for _, slug := range []string{"kraken", "octopus", "webhook", "fresh"} {
		sessionDir(t, root, slug)
	}
	reg.railOrder(polled("kraken", "missing", "octopus", "webhook"))
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for _, pair := range [][2]string{{"kraken", "webhook"}, {"webhook", "octopus"}} {
		wg.Add(1)
		go func(pair [2]string) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if err := reg.Reorder(pair[0], pair[1]); err != nil {
					failures <- err
					return
				}
			}
		}(pair)
	}
	for _, rows := range [][]status.SessionRow{polled("fresh", "kraken", "octopus", "webhook"), polled("webhook", "octopus", "kraken")} {
		wg.Add(1)
		go func(rows []status.SessionRow) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				reg.railOrder(rows)
			}
		}(rows)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	if len(reg.rail) != 5 || reg.rail[0] != "fresh" {
		t.Fatalf("concurrent operations lost entries or fresh placement: %v", reg.rail)
	}
	seen := make(map[string]int)
	for _, slug := range reg.rail {
		seen[slug]++
	}
	for _, slug := range []string{"kraken", "missing", "octopus", "webhook", "fresh"} {
		if seen[slug] != 1 {
			t.Fatalf("entry %q appears %d times in %v", slug, seen[slug], reg.rail)
		}
	}
}

func TestRetiringTheLastActiveSessionStartsTheNextInactiveRailRow(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	for _, slug := range []string{"shown", "next", "later"} {
		sessionDir(t, root, slug)
	}
	reg.railOrder(polled("shown", "next", "later"))
	if err := reg.Show("shown"); err != nil {
		t.Fatal(err)
	}

	reg.retire(reg.Current())

	current := reg.Current()
	if current == nil || current.Slug() != "next" {
		t.Fatalf("the window moved to %v, want the next inactive rail row", current)
	}
	if reg.BySlug("later") != nil {
		t.Fatal("retirement skipped the next inactive rail row")
	}
	if !boot.resumed(t, filepath.Join(root, "next")) {
		t.Fatal("the next inactive session did not resume its conversation")
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
	state := reg.Current()
	term := newTerm(reg, r.Emit)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	pid := ptyPid(state.process)
	socket := boot.socket(t, dir)
	host := (workbench.Handle{Socket: socket, SessionRoot: dir}).WindowHost()
	if _, err := host.Open(context.Background(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "dev", Cwd: dir, Command: []string{"/bin/cat"},
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
	if reg.BySlug("octopus") != nil {
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
	if state := reg.Current(); state == nil || state.Slug() != "webhook" {
		t.Fatalf("cleaning up a background session moved the window to %v", state)
	}
	if err := reg.Cleanup("webhook"); err != nil {
		t.Fatal(err)
	}
	if state := reg.Current(); state == nil || state.Slug() != "kraken" {
		t.Fatalf("the window fell back to %v, want the session shown before it", state)
	}
	if err := reg.Cleanup("kraken"); err != nil {
		t.Fatal(err)
	}
	if state := reg.Current(); state != nil {
		t.Fatalf("cleaning up the last session left %q on screen", state.Slug())
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
	go watch(ctx, reg, root, nil, emit, time.Millisecond, 2*time.Millisecond, time.Hour, counts, noPulls)
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
	shown := reg.Current()
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
	if reg.Current() != shown {
		t.Fatal("revealing a row put it on screen")
	}
	if reg.BySlug("kraken") != nil {
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

// A launcher that builds no command for a session is a workbench that cannot do
// that thing: the page gets the sentinel rather than an empty argv reaching exec.
func TestAWorkbenchWithNoShellCommandRefuses(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	boot.shell = func(string) []string { return nil }
	c := testWorkbench(t, r, opts)
	reg, windows := c.Sessions, c.Windows

	startWorkbench(t, c, opts)
	owner := shownSession(t, reg)
	waitFor(t, "the Overview tab", func() bool { return len(windows.surfaces(owner).Tabs) == 1 })

	if _, err := windows.OpenShell(); !errors.Is(err, ErrNoShellCommand) {
		t.Fatalf("a shell with nothing to run = %v, want ErrNoShellCommand", err)
	}
	if tabs := windows.surfaces(owner).Tabs; len(tabs) != 1 {
		t.Fatalf("the refused shell opened %d tabs", len(tabs))
	}
	if _, err := windows.OpenDocument("thoughts/shared/plans/P1.md"); !errors.Is(err, ErrNoEditorCommand) {
		t.Fatalf("a document with no editor = %v, want ErrNoEditorCommand", err)
	}
}

// A removal resolves its target from the manifest rather than from the directory
// the manifest was read in, so a directory holding another session's manifest
// would take that session's worktrees — one click away from --force.
func TestCleanupRefusesADirectoryHoldingAnotherSessionsManifest(t *testing.T) {
	r := newFakeClient()
	opts, _ := testOptions(t)
	c := testWorkbench(t, r, opts)
	reg := c.Sessions

	startWorkbench(t, c, opts)
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
func polledRail(t *testing.T, r *fakeClient, reg *Sessions, root string) []string {
	t.Helper()
	pushChrome(reg, root, nil, nil, nil, nil, r.Emit)
	var slugs []string
	for _, row := range lastChrome(t, r).Sessions {
		slugs = append(slugs, row.Slug)
	}
	return slugs
}

func lastChrome(t *testing.T, r *fakeClient) status.Fields {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	fields, ok := r.events[chromeEvent].(status.Fields)
	if !ok {
		t.Fatalf("no chrome pushed: %v", r.events)
	}
	return fields
}

// The page names the file, so the guards are what keep a gallery's menu from
// handing an arbitrary path to the clipboard commands.
func TestCopyAssetTakesOnlyAnAssetInsideTheSession(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	var pictures, files []string
	reg.boot.copyImage = func(path string) error {
		pictures = append(pictures, path)
		return nil
	}
	reg.boot.copyFile = func(path string) error {
		files = append(files, path)
		return nil
	}
	kraken := sessionDir(t, root, "kraken")
	assets := filepath.Join(kraken, "thoughts", "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(dir, name string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			t.Fatal(err)
		}
		return real
	}
	shot := write(assets, "shot.png")
	clip := write(assets, "sim.mov")
	notes := write(kraken, "notes.md")
	text := write(assets, "plain.txt")
	outside := write(root, "elsewhere.png")
	disguised := filepath.Join(assets, "disguised.png")
	if err := os.Symlink(text, disguised); err != nil {
		t.Fatal(err)
	}

	if err := reg.CopyAsset("kraken", shot); err != nil {
		t.Fatal(err)
	}
	if err := reg.CopyAsset("kraken", clip); err != nil {
		t.Fatal(err)
	}
	if len(pictures) != 1 || pictures[0] != shot {
		t.Fatalf("pictures %v, want [%q]", pictures, shot)
	}
	if len(files) != 1 || files[0] != clip {
		t.Fatalf("files %v, want [%q]", files, clip)
	}

	for name, call := range map[string]func() error{
		"a slug with no session":      func() error { return reg.CopyAsset("ghost", shot) },
		"a path outside it":           func() error { return reg.CopyAsset("kraken", outside) },
		"a file that is not an asset": func() error { return reg.CopyAsset("kraken", notes) },
		"a png that points at a txt":  func() error { return reg.CopyAsset("kraken", disguised) },
	} {
		if err := call(); err == nil {
			t.Fatalf("%s was copied anyway", name)
		}
	}
	if len(pictures) != 1 || len(files) != 1 {
		t.Fatalf("a refused request still reached the clipboard: %v %v", pictures, files)
	}
}

// exitedPTY is a process that has already ended, as a crashed supervisor's is.
func exitedPTY(t *testing.T) *PTY {
	t.Helper()
	process, err := StartPTY([]string{"/bin/sh", "-c", "exit 0"}, nil, t.TempDir(), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(process.Stop)
	waitFor(t, "the process to exit", process.Done)
	return process
}

// testKernel wires a conversation and a window registry over one session
// registry, emitting at emit, as New does without booting anything.
func testKernel(t *testing.T, emit Emitter) (*Sessions, *Term, *Windows) {
	t.Helper()
	reg := newSessions()
	windows := newWindows(emit, reg)
	t.Cleanup(reg.stopAll)
	t.Cleanup(windows.stopAll)
	return reg, newTerm(reg, emit), windows
}

var errNoAgent = errors.New("no agent command")

// The rail freezes its order at the first poll and prepends anything it has not
// seen, so a session created mid-lifetime takes the front and the first shortcut.
func TestASessionCreatedMidLifetimeTakesTheFrontOfTheRail(t *testing.T) {
	reg := newSessions()
	first := reg.railOrder([]status.SessionRow{{Slug: "older"}})
	if len(first) != 1 || first[0].Slug != "older" {
		t.Fatalf("rail = %+v", first)
	}

	after := reg.railOrder([]status.SessionRow{{Slug: "fresh"}, {Slug: "older"}})
	if len(after) != 2 || after[0].Slug != "fresh" || after[1].Slug != "older" {
		t.Fatalf("a session created mid-lifetime landed at %+v", after)
	}
}

// The page names the path, so a reveal outside the session would hand the file
// manager any directory on the machine.
func TestRevealPathRevealsOnlyAPathInsideItsSession(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	var revealed []string
	reg.boot.reveal = func(path string) error {
		revealed = append(revealed, path)
		return nil
	}
	kraken := sessionDir(t, root, "kraken")
	inside := filepath.Join(kraken, "thoughts")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := reg.RevealPath("kraken", inside); err != nil {
		t.Fatal(err)
	}
	if len(revealed) != 1 || revealed[0] != inside {
		t.Fatalf("revealed %v, want [%q]", revealed, inside)
	}
	if err := reg.RevealPath("kraken", root); !errors.Is(err, ErrPathOutsideSession) {
		t.Fatalf("a path outside the session = %v, want ErrPathOutsideSession", err)
	}
	if err := reg.RevealPath("ghost", inside); err == nil {
		t.Fatal("a slug with no session was revealed anyway")
	}
	if len(revealed) != 1 {
		t.Fatalf("a refused path reached the file manager: %v", revealed)
	}
}

// The chime is for a wait the user is not looking at: the session on screen in
// a focused window stays silent, and it rings once the window is unfocused or
// shows another session.
func TestTheWorkbenchChimesOnlyWhenTheWindowIsNotFocusedOnTheSession(t *testing.T) {
	r := newFakeClient()
	opts, boot := testOptions(t)
	opts.Config = &config.Config{}
	sessionDir(t, opts.Root, "kraken")
	c := testWorkbench(t, r, opts)
	reg := c.Sessions
	startWorkbench(t, c, opts)
	octopus := shownSession(t, reg)
	handle := workbench.Handle{Socket: boot.socket(t, opts.SessionRoot), SessionRoot: opts.SessionRoot}
	attention := func() {
		t.Helper()
		if _, err := handle.WindowHost().Open(context.Background(), workbench.WindowOptions{
			Kind: workbench.KindDocument, Label: "qrouton", Content: "done", Attention: true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	r.inFront.Store(true)
	attention()
	if got := r.chimes(); len(got) != 0 {
		t.Fatalf("the watched session chimed %v", got)
	}
	r.inFront.Store(false)
	attention()
	if got, want := r.chimes(), opts.SessionRoot; len(got) != 1 || got[0] != want {
		t.Fatalf("chimed %v, want %s once", got, want)
	}

	_ = octopus.write([]byte("y"))
	if err := reg.Show("kraken"); err != nil {
		t.Fatal(err)
	}
	r.inFront.Store(true)
	attention()
	if got := r.chimes(); len(got) != 2 || got[1] != opts.SessionRoot {
		t.Fatalf("a focused window showing another session chimed %v, want %s a second time", got, opts.SessionRoot)
	}
}

func (f *fakeClient) chimes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.rung...)
}
