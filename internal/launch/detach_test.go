package launch

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// shortDir is a directory for unix sockets. macOS caps the path at 104 bytes and
// the test temp root eats most of that budget.
func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "qrdetach")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func listen(t *testing.T, socket string) {
	t.Helper()
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
}

func TestDetachReturnsOnceTheSocketAnswersAndKeepsTheLog(t *testing.T) {
	dir := shortDir(t)
	socket := filepath.Join(dir, "control.sock")
	listen(t, socket)
	log := filepath.Join(dir, "nested", "workbench.log")

	if err := Detach([]string{"/bin/sh", "-c", "echo up; sleep 5"}, os.Environ(), socket, log); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		b, err := os.ReadFile(log)
		if err == nil && strings.Contains(string(b), "up") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("log %q never carried the child's output (%v)", log, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A detach that returned zero while the child died is a prompt with no window
// and no explanation, which is worse than blocking the terminal.
func TestDetachFailsWhenTheChildDiesFirst(t *testing.T) {
	dir := shortDir(t)
	log := filepath.Join(dir, "workbench.log")

	err := Detach([]string{"/bin/sh", "-c", "echo boom >&2; exit 3"}, os.Environ(),
		filepath.Join(dir, "never.sock"), log)
	if !errors.Is(err, ErrWorkbenchExited) {
		t.Fatalf("error = %v, want %v", err, ErrWorkbenchExited)
	}
	if !strings.Contains(err.Error(), log) {
		t.Fatalf("error %q does not point at the log %q", err, log)
	}
	b, readErr := os.ReadFile(log)
	if readErr != nil || !strings.Contains(string(b), "boom") {
		t.Fatalf("log = %q (%v), want the child's stderr", b, readErr)
	}
}

func TestDetachKillsAWorkbenchThatNeverAnswers(t *testing.T) {
	dir := shortDir(t)
	log := filepath.Join(dir, "workbench.log")

	err := detach([]string{"/bin/sh", "-c", "sleep 0.4; echo late"}, os.Environ(),
		filepath.Join(dir, "never.sock"), log, 100*time.Millisecond, 10*time.Millisecond)
	if !errors.Is(err, ErrWorkbenchNotReady) {
		t.Fatalf("error = %v, want %v", err, ErrWorkbenchNotReady)
	}
	// The child and everything it spawned share its new session, so the sleep
	// dies with it and the log never gains a second line.
	time.Sleep(700 * time.Millisecond)
	if b, _ := os.ReadFile(log); strings.Contains(string(b), "late") {
		t.Fatalf("log = %q, want a child that was killed before it got there", b)
	}
}

func TestWaitReadyReportsWhatWentWrong(t *testing.T) {
	dir := shortDir(t)
	socket := filepath.Join(dir, "control.sock")
	listen(t, socket)
	if err := waitReady(socket, make(chan error), time.Second, 10*time.Millisecond); err != nil {
		t.Fatalf("waitReady on a listening socket = %v, want nil", err)
	}

	exited := make(chan error, 1)
	exited <- errors.New("exit status 1")
	if err := waitReady(filepath.Join(dir, "never.sock"), exited, time.Second, 10*time.Millisecond); !errors.Is(err, ErrWorkbenchExited) {
		t.Fatalf("waitReady after a dead child = %v, want %v", err, ErrWorkbenchExited)
	}

	err := waitReady(filepath.Join(dir, "never.sock"), make(chan error), 50*time.Millisecond, 10*time.Millisecond)
	if !errors.Is(err, ErrWorkbenchNotReady) {
		t.Fatalf("waitReady with no child = %v, want %v", err, ErrWorkbenchNotReady)
	}
}

// The spec is the only thing the detached process is told, so a field lost in
// the round trip is a workbench that opens on nothing.
func TestWorkbenchSpecRoundTrips(t *testing.T) {
	spec := WorkbenchSpec{SessionRoot: "/sessions/api", Socket: "/tmp/qrouton-sock/501/ab.sock",
		Runner: "codex", Resume: true}

	argv := WorkbenchArgv("/bin/qrouton", spec)
	if argv[0] != "/bin/qrouton" || argv[1] != "--workbench-spec" || len(argv) != 3 {
		t.Fatalf("workbench argv = %v, want the binary, the marker and one spec", argv)
	}
	parsed, err := ParseWorkbenchSpec(argv[2])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SessionRoot != spec.SessionRoot || parsed.Socket != spec.Socket ||
		parsed.Runner != spec.Runner || parsed.Resume != spec.Resume {
		t.Fatalf("parsed spec = %#v, want %#v", parsed, spec)
	}
}

func TestParseWorkbenchSpecRejectsAnIncompleteOne(t *testing.T) {
	// A workbench with no session is the ordinary case — that window's content is
	// the assembly overlay — so the socket is the only thing it cannot open without.
	for _, spec := range []WorkbenchSpec{
		{SessionRoot: "/sessions/api", Socket: "/tmp/a.sock"},
		{Socket: "/tmp/a.sock"},
	} {
		if _, err := ParseWorkbenchSpec(spec.Marshal()); err != nil {
			t.Fatalf("ParseWorkbenchSpec(%#v) = %v, want it accepted", spec, err)
		}
	}
	if _, err := ParseWorkbenchSpec(WorkbenchSpec{SessionRoot: "/sessions/api"}.Marshal()); !errors.Is(err, ErrWorkbenchSpecIncomplete) {
		t.Fatalf("a spec with no socket = %v, want %v", err, ErrWorkbenchSpecIncomplete)
	}
	if _, err := ParseWorkbenchSpec("not json"); err == nil {
		t.Fatal("ParseWorkbenchSpec accepted a non-JSON spec")
	}
}
