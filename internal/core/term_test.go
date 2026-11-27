package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// recorder collects what the Go side would have pushed at the page.
type recorder struct {
	mu     sync.Mutex
	events []string
	data   strings.Builder
}

func (r *recorder) emit(event string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	if strings.HasPrefix(event, ptyDataName) {
		if out, ok := payload.(Output); ok {
			r.data.Write(out.Data)
		}
	}
}

func (r *recorder) output() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.data.String()
}

func (r *recorder) saw(event string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, seen := range r.events {
		if seen == event {
			return true
		}
	}
	return false
}

// testTerm is a conversation for one registered session, and the session it
// addresses.
func testTerm(t *testing.T, emit emitFunc, root string, argv, env []string) (*Term, *SessionState) {
	t.Helper()
	reg := newSessions()
	t.Cleanup(reg.stopAll)
	return newTerm(reg, emitFunc(emit)), reg.add(root, argv, WithTerminalEnv(env))
}

// waitCeiling is not a latency any test here asserts: a child answers in
// milliseconds or never, and a shared runner spawning this many processes can
// stall for whole seconds between the two.
const waitCeiling = 30 * time.Second

// macOS discards a PTY's unread output when the last slave descriptor closes,
// and the pump that reads it starts a goroutine behind the child. So a child
// here holds the terminal open until the test has what it came for, rather
// than printing and exiting into that race.

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

// The PTY carries a real terminal: the child sees a tty and its output reaches
// the page.
func TestTermPumpsChildOutputToThePage(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "test -t 0 && printf 'on a tty\\n'; read done"}, os.Environ())

	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's output", func() bool { return strings.Contains(rec.output(), "on a tty") })
	if err := term.Write(state.terminal, []byte("\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the exit event", func() bool { return rec.saw(ptyExitName + state.terminal) })
}

func TestTermTellsTheChildWhatToRenderFor(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "printf '%s %s <%s>\\n' \"$TERM\" \"$COLORTERM\" \"$NO_COLOR\"; exec cat"},
		[]string{"TERM=dumb", "NO_COLOR=1"})

	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's environment", func() bool {
		return strings.Contains(rec.output(), "xterm-256color truecolor <>")
	})
}

// Every relative path the agent is given is read against its cwd, so a
// conversation running anywhere but the session root is a session working on the
// wrong tree.
func TestTheConversationRunsInTheSessionRoot(t *testing.T) {
	rec := &recorder{}
	root := t.TempDir()
	term, state := testTerm(t, rec.emit, root, []string{"/bin/sh", "-c", "pwd; exec cat"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	want := resolved(t, root)
	deadline := time.Now().Add(waitCeiling)
	for !strings.Contains(rec.output(), want) {
		if time.Now().After(deadline) {
			t.Fatalf("the child wrote %q, wanted %q in it", rec.output(), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A page subscribes per terminal, so a stream announced under a bare name is a
// stream nobody hears.
func TestTheConversationsStreamsAreKeyedByTerminal(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "printf 'keyed\\n'"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the terminal's data event", func() bool { return rec.saw(ptyDataName + state.terminal) })
	waitFor(t, "the terminal's exit event", func() bool { return rec.saw(ptyExitName + state.terminal) })
	if !strings.Contains(rec.output(), "keyed") {
		t.Fatalf("the keyed data event carried %q", rec.output())
	}
	for _, unkeyed := range []string{"pty:data", "pty:exit", ptyDataName, ptyExitName} {
		if rec.saw(unkeyed) {
			t.Fatalf("%q was emitted, which names no terminal and reaches no page", unkeyed)
		}
	}
}

// resolved is the path a child reports for a directory. A temp directory is
// reached through a symlink, and the child names what is behind it.
func resolved(t *testing.T, dir string) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return physical
}

// The page attaches on load, so a reload must not fork a second agent.
func TestTermAttachStartsOnlyOnce(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(), []string{"/bin/cat"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	first := ptyPid(state.process)
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	if ptyPid(state.process) != first {
		t.Fatal("a second Attach forked another agent")
	}
}

// A conversation is addressed by its own terminal id, so two sessions are two
// agents rather than two views of one.
func TestTermStartsOneProcessPerTerminal(t *testing.T) {
	rec := &recorder{}
	reg := newSessions()
	t.Cleanup(reg.stopAll)
	one := reg.add(t.TempDir(), []string{"/bin/cat"}, os.Environ())
	two := reg.add(t.TempDir(), []string{"/bin/cat"}, os.Environ())
	term := newTerm(reg, emitFunc(rec.emit))

	if one.terminal == two.terminal {
		t.Fatalf("both sessions were minted %q", one.terminal)
	}
	for _, state := range []*SessionState{one, two} {
		if err := startTerm(term, state.terminal, 80, 24); err != nil {
			t.Fatal(err)
		}
	}
	if ptyPid(one.process) == ptyPid(two.process) {
		t.Fatal("two conversations share one process")
	}
}

func TestTermWritesKeystrokesToTheChild(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(), []string{"/bin/cat"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}

	if err := term.Write(state.terminal, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the echoed keystrokes", func() bool { return strings.Contains(rec.output(), "hello") })
}

func TestTermRejectsWritesBeforeItHasAPTY(t *testing.T) {
	term, state := testTerm(t, func(string, any) {}, t.TempDir(), []string{"/bin/cat"}, nil)
	keystroke := []byte("x")
	if err := term.Write(state.terminal, keystroke); err != ErrTerminalNotStarted {
		t.Fatalf("Write before Attach returned %v, want ErrTerminalNotStarted", err)
	}
	if err := term.Write("term-99", keystroke); err == nil {
		t.Fatal("a write to a terminal nobody registered succeeded")
	}
	if err := startTerm(term, "term-99", 80, 24); err == nil {
		t.Fatal("a start of a terminal nobody registered succeeded")
	}
}

// Ending a session ends the runner too, and the runner is a grandchild of the
// supervisor — so the signal has to reach the whole process group, not just the
// process the workbench started.
func TestStopKillsTheWholeProcessTree(t *testing.T) {
	rec := &recorder{}
	dir := t.TempDir()
	marker := dir + "/grandchild.pid"
	term, state := testTerm(t, rec.emit, dir,
		[]string{"/bin/sh", "-c", "sleep 300 & echo $! > " + marker + "; wait"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	var grandchild int
	waitFor(t, "the grandchild's pid", func() bool {
		b, err := os.ReadFile(marker)
		if err != nil {
			return false
		}
		grandchild, err = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil && grandchild > 0
	})

	state.stop()

	waitFor(t, "the grandchild to die", func() bool {
		return syscall.Kill(grandchild, 0) != nil
	})
}

func TestBothTerminalKindsShareOneOutputTransport(t *testing.T) {
	var mu sync.Mutex
	var agentPayload any
	term, state := testTerm(t, func(event string, payload any) {
		if strings.HasPrefix(event, ptyDataName) {
			mu.Lock()
			agentPayload = payload
			mu.Unlock()
		}
	}, t.TempDir(), []string{"/bin/sh", "-c", "printf 'chunk\\n'; sleep 5"}, os.Environ())
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the agent terminal's data event", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return agentPayload != nil
	})
	mu.Lock()
	_, agentOutput := agentPayload.(Output)
	mu.Unlock()
	if !agentOutput {
		t.Fatalf("agent terminal emitted %T, want Output", agentPayload)
	}

	w, renderer := testWindows(t)
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "shell", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := attachTab(w, nil, id, 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(id, []byte("echo\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the tab terminal's live output", func() bool {
		renderer.mu.Lock()
		defer renderer.mu.Unlock()
		chunk, ok := renderer.events[windowDataName+id].(Output)
		return ok && !chunk.Replay
	})
}
