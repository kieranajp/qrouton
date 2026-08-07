package desktop

import (
	"encoding/base64"
	"os"
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
	if event == ptyDataEvent {
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
	term := newTerm(Options{
		SessionRoot: t.TempDir(),
		Argv:        []string{"/bin/sh", "-c", "test -t 0 && printf 'on a tty\\n'"},
		Env:         os.Environ(),
	}, rec.emit)

	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's output", func() bool { return strings.Contains(rec.output(), "on a tty") })
	waitFor(t, "the exit event", func() bool { return rec.saw(ptyExitEvent) })
}

func TestTermTellsTheChildWhatToRenderFor(t *testing.T) {
	rec := &recorder{}
	term := newTerm(Options{
		SessionRoot: t.TempDir(),
		Argv:        []string{"/bin/sh", "-c", "printf '%s %s\\n' \"$TERM\" \"$COLORTERM\""},
		Env:         []string{"TERM=dumb"},
	}, rec.emit)

	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the child's environment", func() bool {
		return strings.Contains(rec.output(), termValue+" "+colorTermValue)
	})
}

// The page calls Start on load, so a reload must not fork a second agent.
func TestTermStartIsIdempotent(t *testing.T) {
	rec := &recorder{}
	term := newTerm(Options{SessionRoot: t.TempDir(), Argv: []string{"/bin/cat"}, Env: os.Environ()}, rec.emit)
	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	first := term.process.cmd.Process.Pid
	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	if term.process.cmd.Process.Pid != first {
		t.Fatal("a second Start forked another agent")
	}
	term.Stop()
}

func TestTermWritesKeystrokesToTheChild(t *testing.T) {
	rec := &recorder{}
	term := newTerm(Options{SessionRoot: t.TempDir(), Argv: []string{"/bin/cat"}, Env: os.Environ()}, rec.emit)
	if err := term.Start(80, 24); err != nil {
		t.Fatal(err)
	}
	defer term.Stop()

	if err := term.Write(base64.StdEncoding.EncodeToString([]byte("hello\n"))); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the echoed keystrokes", func() bool { return strings.Contains(rec.output(), "hello") })
}

func TestTermRejectsWritesBeforeItHasAPTY(t *testing.T) {
	term := newTerm(Options{SessionRoot: t.TempDir(), Argv: []string{"/bin/cat"}}, func(string, any) {})
	if err := term.Write(base64.StdEncoding.EncodeToString([]byte("x"))); err != ErrTerminalNotStarted {
		t.Fatalf("Write before Start returned %v, want ErrTerminalNotStarted", err)
	}
}

// Closing the conversation window ends the session, and the runner is a
// grandchild of the supervisor — so the signal has to reach the whole process
// group, not just the process the workbench started.
func TestStopKillsTheWholeProcessTree(t *testing.T) {
	rec := &recorder{}
	dir := t.TempDir()
	marker := dir + "/grandchild.pid"
	term := newTerm(Options{
		SessionRoot: dir,
		Argv:        []string{"/bin/sh", "-c", "sleep 300 & echo $! > " + marker + "; wait"},
		Env:         os.Environ(),
	}, rec.emit)
	if err := term.Start(80, 24); err != nil {
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

	term.Stop()

	waitFor(t, "the grandchild to die", func() bool {
		return syscall.Kill(grandchild, 0) != nil
	})
}
