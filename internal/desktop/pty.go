package desktop

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// ptyProcess is a command running under a PTY the workbench owns.
type ptyProcess struct {
	mu     sync.Mutex
	file   *os.File
	cmd    *exec.Cmd
	exited chan struct{}
	code   int
}

func startPTY(argv, env []string, dir string, cols, rows int) (*ptyProcess, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = dir, env
	file, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, err
	}
	p := &ptyProcess{file: file, cmd: cmd, exited: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		p.mu.Lock()
		p.code = exitCode(err)
		p.mu.Unlock()
		close(p.exited)
	}()
	return p, nil
}

// pump forwards the PTY's output until it closes, then reports the exit status.
func (p *ptyProcess) pump(onData func([]byte), onExit func(code int)) {
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

func (p *ptyProcess) write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return ErrTerminalNotStarted
	}
	_, err := p.file.Write(data)
	return err
}

func (p *ptyProcess) resize(cols, rows int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.file == nil {
		return nil
	}
	return pty.Setsize(p.file, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// stop ends the process tree. pty.Start puts the child in a session of its own,
// so signalling the negative pid reaches everything it spawned.
func (p *ptyProcess) stop() {
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
	case <-time.After(terminateGrace):
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
