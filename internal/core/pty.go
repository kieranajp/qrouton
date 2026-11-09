package core

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// PTY is one child process running under a pseudo-terminal.
type PTY struct {
	mu     sync.Mutex
	file   *os.File
	cmd    *exec.Cmd
	exited chan struct{}
	code   int
	grace  time.Duration
}

func StartPTY(argv, env []string, dir string, cols, rows int) (*PTY, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	p := &PTY{file: file, cmd: cmd, exited: make(chan struct{}), grace: terminateGrace}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.code = exitCode(err)
		p.mu.Unlock()
		close(p.exited)
	}()
	return p, nil
}

func (p *PTY) Pump(onData func([]byte), onExit func(code int)) {
	p.mu.Lock()
	file := p.file
	p.mu.Unlock()
	if file == nil {
		return
	}
	buffer := make([]byte, ptyReadBuffer)
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			onData(buffer[:n])
		}
		if err != nil {
			<-p.exited
			p.mu.Lock()
			code := p.code
			p.mu.Unlock()
			onExit(code)
			return
		}
	}
}

func (p *PTY) Done() bool {
	select {
	case <-p.exited:
		return true
	default:
		return false
	}
}

func (p *PTY) Write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return ErrTerminalNotStarted
	}
	_, err := p.file.Write(data)
	return err
}

// Foreground reports whether a job other than the PTY child holds the terminal.
// ok is false when the terminal could not be read.
func (p *PTY) Foreground() (busy, ok bool) {
	// A write blocked on a full terminal holds the lock; skip this read rather
	// than stall every other shell's.
	if !p.mu.TryLock() {
		return false, false
	}
	defer p.mu.Unlock()
	if p.file == nil || p.cmd == nil || p.cmd.Process == nil {
		return false, false
	}
	pgid, err := unix.IoctlGetInt(int(p.file.Fd()), unix.TIOCGPGRP)
	if err != nil || pgid == 0 {
		return false, false
	}
	return pgid != p.cmd.Process.Pid, true
}

// Pid is the child's, and 0 once the PTY is stopped.
func (p *PTY) Pid() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

func (p *PTY) Size() (cols, rows int, err error) {
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

func (p *PTY) Resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return nil
	}
	return pty.Setsize(p.file, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// Stop ends the process tree. pty.Start puts the child in a session of its own,
// so signalling the negative pid reaches everything it spawned.
func (p *PTY) Stop() {
	p.mu.Lock()
	cmd, file := p.cmd, p.file
	p.file, p.cmd = nil, nil
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-p.exited:
	case <-time.After(p.grace):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-p.exited
	}
	_ = file.Close()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return -1
}
