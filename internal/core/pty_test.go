package core

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

func startTestPTY(t *testing.T, argv ...string) *PTY {
	t.Helper()
	p, err := StartPTY(argv, os.Environ(), t.TempDir(), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	// Every child here ignores SIGTERM, so each stop waits out the grace.
	p.grace = 50 * time.Millisecond
	t.Cleanup(p.Stop)
	go p.Pump(func([]byte) {}, func(int) {})
	return p
}

// Catches Foreground comparing the terminal's group against the wrong pid, or
// answering for a process that has been stopped.
func TestForegroundReportsThePTYsForegroundJob(t *testing.T) {
	p := startTestPTY(t, "/bin/sh", "-i")
	settle := func(want bool) {
		waitFor(t, "the foreground job to settle", func() bool {
			busy, ok := p.Foreground()
			return ok && busy == want
		})
	}

	settle(false)
	if err := p.Write([]byte("set -m\nsleep 30\n")); err != nil {
		t.Fatal(err)
	}
	settle(true)
	p.Stop()
	if _, ok := p.Foreground(); ok {
		t.Fatal("a stopped PTY still reported a foreground job")
	}
}

// Catches a stop that only sends SIGTERM, or only signals the child rather
// than its whole process group: a runner that ignores the one outlives its tab.
func TestStopKillsAGroupThatIgnoresSIGTERMAfterTheGrace(t *testing.T) {
	marker := t.TempDir() + "/grandchild.pid"
	p := startTestPTY(t, "/bin/sh", "-c", "trap '' TERM HUP; sleep 300 & echo $! > "+marker+"; wait")
	var grandchild int
	waitFor(t, "the grandchild's pid", func() bool {
		b, err := os.ReadFile(marker)
		if err != nil {
			return false
		}
		grandchild, err = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil && grandchild > 0
	})

	began := time.Now()
	stopped := make(chan struct{})
	go func() {
		p.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(waitCeiling):
		t.Fatal("stop never returned: the group was not killed after the grace")
	}

	if elapsed := time.Since(began); elapsed < p.grace {
		t.Fatalf("stop returned after %v, before the %v grace: SIGTERM was not ignored", elapsed, p.grace)
	}
	if !p.Done() {
		t.Fatal("stop returned with the child still running")
	}
	waitFor(t, "the grandchild to die", func() bool { return syscall.Kill(grandchild, 0) != nil })
}

// ptyPid is the child's pid, and 0 once the PTY is stopped.
func ptyPid(p *PTY) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// ptySize is the size the child's terminal reports.
func ptySize(p *PTY) (cols, rows int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return 0, 0, ErrTerminalNotStarted
	}
	size, err := pty.GetsizeFull(p.file)
	if err != nil {
		return 0, 0, err
	}
	return int(size.Cols), int(size.Rows), nil
}

// Catches a switch copying the conversation while the runner the supervisor
// started is still writing it: Stop returns once the supervisor alone has exited.
func TestStopGroupReturnsOnlyOnceAGrandchildIgnoringSIGTERMIsGone(t *testing.T) {
	marker := t.TempDir() + "/grandchild.pid"
	p := startTestPTY(t, deafGrandchild(marker)...)
	grandchild := readPID(t, marker)
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })

	stopped := make(chan error, 1)
	go func() { stopped <- p.StopGroup() }()
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("StopGroup = %v, want a drained group", err)
		}
	case <-time.After(waitCeiling):
		t.Fatal("StopGroup never returned")
	}
	if executing(t, grandchild) {
		t.Fatal("StopGroup returned with the grandchild still running")
	}
}

// Catches StopGroup reporting a drained group while a process it could not kill
// is still running, which lets a switch copy under a live writer.
func TestStopGroupReportsAGroupThatOutlivesSIGKILL(t *testing.T) {
	marker := t.TempDir() + "/grandchild.pid"
	p := startTestPTY(t, deafGrandchild(marker)...)
	grandchild := readPID(t, marker)
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })
	p.signal, p.drain = ignoreSIGKILL, 50*time.Millisecond

	if err := p.StopGroup(); !errors.Is(err, ErrGroupNotDrained) {
		t.Fatalf("StopGroup = %v, want %v", err, ErrGroupNotDrained)
	}
}

func ignoreSIGKILL(pid int, sig syscall.Signal) error {
	if sig == syscall.SIGKILL {
		return nil
	}
	return syscall.Kill(pid, sig)
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	var pid int
	waitFor(t, "a pid in "+path, func() bool {
		b, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil && pid > 0
	})
	return pid
}

// executing is false for a pid that has gone and for a zombie, which kill(pid, 0)
// still answers for until its parent reaps it.
func executing(t *testing.T, pid int) bool {
	t.Helper()
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if _, exited := err.(*exec.ExitError); err != nil && !exited {
		t.Fatalf("ps: %v", err)
	}
	state := strings.TrimSpace(string(out))
	return state != "" && !strings.HasPrefix(state, "Z")
}

// deafGrandchild starts a grandchild that ignores SIGTERM and SIGHUP, and writes
// its pid to marker only once it does, so no stop can land before the trap.
func deafGrandchild(marker string) []string {
	return []string{"/bin/sh", "-c", "/bin/sh -c 'trap \"\" TERM HUP; echo $$ > " + marker + "; exec sleep 300' & wait"}
}
