package launch

// The workbench runs in a process of its own so the terminal comes straight
// back. Only the event loop is handed over; the parent does not return until the
// child answers on its control socket, because a prompt with no window behind it
// is worse than a blocked terminal.

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// readyTimeout bounds the wait for the workbench to serve its socket.
	readyTimeout  = 20 * time.Second
	readyInterval = 25 * time.Millisecond

	logMode = 0o600
	dirMode = 0o755
)

// WorkbenchSpec is what the detached process is told to open: the session it
// belongs to (empty until onboarding chooses one), the control socket it serves,
// and the command its conversation terminal runs. Everything it needs is here,
// so the child never repeats the assembly the parent already did.
type WorkbenchSpec struct {
	SessionRoot string   `json:"session_root,omitempty"`
	Socket      string   `json:"socket"`
	Argv        []string `json:"argv"`
	Dock        bool     `json:"dock,omitempty"`
	// Empty on the landing list, which has not resolved one yet.
	Editor EditorCommand `json:"editor"`
}

func (s WorkbenchSpec) Marshal() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func ParseWorkbenchSpec(s string) (WorkbenchSpec, error) {
	var spec WorkbenchSpec
	if err := json.Unmarshal([]byte(s), &spec); err != nil {
		return WorkbenchSpec{}, fmt.Errorf("%s: %w", specParseError, err)
	}
	if spec.Socket == "" || len(spec.Argv) == 0 {
		return WorkbenchSpec{}, fmt.Errorf("%w: %q", ErrWorkbenchSpecIncomplete, s)
	}
	return spec, nil
}

// WorkbenchArgv is the detached process's own command: this binary again, with
// the marker that makes it run the event loop instead of assembling a session.
func WorkbenchArgv(qroutonBin string, spec WorkbenchSpec) []string {
	return []string{qroutonBin, workbenchSpecFlag, spec.Marshal()}
}

// Detach starts argv in a session of its own and returns once it answers on
// socket. Setsid is what lets the workbench outlive the shell that started it,
// including a terminal closed the moment the prompt comes back; its stdio goes
// to log for the same reason.
func Detach(argv, env []string, socket, log string) error {
	return detach(argv, env, socket, log, readyTimeout, readyInterval)
}

func detach(argv, env []string, socket, log string, timeout, interval time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(log), dirMode); err != nil {
		return err
	}
	file, err := os.OpenFile(log, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, logMode)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = file, file
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := waitReady(socket, exited, timeout, interval); err != nil {
		// A workbench that never answered has no window and no way to be found
		// again, so it does not get to linger.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return fmt.Errorf(workbenchFailureFormat, err, log)
	}
	return nil
}

// waitReady blocks until the socket accepts a connection, the child dies, or
// the deadline passes. Dialling is checked before the child's fate so a process
// that answered and then exited still counts as started.
func waitReady(socket string, exited <-chan error, timeout, interval time.Duration) error {
	deadline := time.After(timeout)
	for {
		if answered(socket) {
			return nil
		}
		select {
		case err := <-exited:
			if err == nil {
				return ErrWorkbenchExited
			}
			return fmt.Errorf("%w (%v)", ErrWorkbenchExited, err)
		case <-deadline:
			return ErrWorkbenchNotReady
		case <-time.After(interval):
		}
	}
}

func answered(socket string) bool {
	conn, err := net.Dial(socketNetwork, socket)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
