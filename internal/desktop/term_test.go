package desktop

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
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
	if strings.HasPrefix(event, ptyDataEvent) {
		decoded, err := base64.StdEncoding.DecodeString(payload.(string))
		if err != nil {
			return
		}
		r.data.Write(decoded)
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
func testTerm(t *testing.T, emit emitter, root string, argv, env []string) (*Term, *sessionState) {
	t.Helper()
	reg := newSessions()
	t.Cleanup(reg.stopAll)
	return newTerm(reg, emit), reg.add(root, argv, withTerminalEnv(env))
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The PTY carries a real terminal: the child sees a tty and its output reaches
// the page base64-encoded, because a raw chunk is not valid UTF-8 at its
// boundary.
func TestTermPumpsChildOutputToThePage(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "test -t 0 && printf 'on a tty\\n'"}, os.Environ())

	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's output", func() bool { return strings.Contains(rec.output(), "on a tty") })
	waitFor(t, "the exit event", func() bool { return rec.saw(ptyExitEvent + state.terminal) })
}

func TestTermTellsTheChildWhatToRenderFor(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "printf '%s %s\\n' \"$TERM\" \"$COLORTERM\""}, []string{"TERM=dumb"})

	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's environment", func() bool {
		return strings.Contains(rec.output(), termValue+" "+colorTermValue)
	})
}

// Every relative path the agent is given is read against its cwd, so a
// conversation running anywhere but the session root is a session working on the
// wrong tree.
func TestTheConversationRunsInTheSessionRoot(t *testing.T) {
	rec := &recorder{}
	root := t.TempDir()
	term, state := testTerm(t, rec.emit, root, []string{"/bin/pwd"}, os.Environ())
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's own directory", func() bool {
		return strings.Contains(rec.output(), resolved(t, root))
	})
}

// The landing path registers its conversation before onboarding has chosen a
// session, and the agent it is already running has to end up in the session it
// chose.
func TestTheConversationRunsInASessionAdoptedAfterItWasRegistered(t *testing.T) {
	rec := &recorder{}
	reg := newSessions()
	state := reg.add("", []string{"/bin/pwd"}, withTerminalEnv(os.Environ()))
	term := newTerm(reg, rec.emit)

	adopted := t.TempDir()
	if err := reg.adopt(adopted, false); err != nil {
		t.Fatal(err)
	}
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's own directory", func() bool {
		return strings.Contains(rec.output(), resolved(t, adopted))
	})
}

// A page subscribes per terminal, so a stream announced under a bare name is a
// stream nobody hears.
func TestTheConversationsStreamsAreKeyedByTerminal(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(),
		[]string{"/bin/sh", "-c", "printf 'keyed\\n'"}, os.Environ())
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the terminal's data event", func() bool { return rec.saw(ptyDataEvent + state.terminal) })
	waitFor(t, "the terminal's exit event", func() bool { return rec.saw(ptyExitEvent + state.terminal) })
	if !strings.Contains(rec.output(), "keyed") {
		t.Fatalf("the keyed data event carried %q", rec.output())
	}
	for _, unkeyed := range []string{"pty:data", "pty:exit", ptyDataEvent, ptyExitEvent} {
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

// The page calls Start on load, so a reload must not fork a second agent.
func TestTermStartIsIdempotent(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(), []string{"/bin/cat"}, os.Environ())
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	first := state.process.cmd.Process.Pid
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	if state.process.cmd.Process.Pid != first {
		t.Fatal("a second Start forked another agent")
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
	term := newTerm(reg, rec.emit)

	if one.terminal == two.terminal {
		t.Fatalf("both sessions were minted %q", one.terminal)
	}
	for _, state := range []*sessionState{one, two} {
		if err := term.Start(state.terminal, 80, 24); err != nil {
			t.Fatal(err)
		}
	}
	if one.process.cmd.Process.Pid == two.process.cmd.Process.Pid {
		t.Fatal("two conversations share one process")
	}
}

func TestTermWritesKeystrokesToTheChild(t *testing.T) {
	rec := &recorder{}
	term, state := testTerm(t, rec.emit, t.TempDir(), []string{"/bin/cat"}, os.Environ())
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}

	if err := term.Write(state.terminal, base64.StdEncoding.EncodeToString([]byte("hello\n"))); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the echoed keystrokes", func() bool { return strings.Contains(rec.output(), "hello") })
}

func TestTermRejectsWritesBeforeItHasAPTY(t *testing.T) {
	term, state := testTerm(t, func(string, any) {}, t.TempDir(), []string{"/bin/cat"}, nil)
	keystroke := base64.StdEncoding.EncodeToString([]byte("x"))
	if err := term.Write(state.terminal, keystroke); err != ErrTerminalNotStarted {
		t.Fatalf("Write before Start returned %v, want ErrTerminalNotStarted", err)
	}
	if err := term.Write("term-99", keystroke); err == nil {
		t.Fatal("a write to a terminal nobody registered succeeded")
	}
	if err := term.Start("term-99", 80, 24); err == nil {
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
	if err := term.Start(state.terminal, 80, 24); err != nil {
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
