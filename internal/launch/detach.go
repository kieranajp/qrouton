package launch

// The workbench runs in a process of its own so the terminal comes straight
// back. Only the event loop is handed over; the parent does not return until the
// child answers on its control socket, because a prompt with no window behind it
// is worse than a blocked terminal.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

const (
	// readyTimeout bounds the wait for the workbench to serve its socket.
	readyTimeout  = 20 * time.Second
	readyInterval = 25 * time.Millisecond

	logMode = 0o600
	dirMode = 0o755
)

// WorkbenchSpec is what the detached process is told to open: its session, its
// control socket, its runner, and whether that runner has a conversation to
// resume. An empty SessionRoot opens on no session at all, which is where the
// assembly overlay draws. It builds each session's own command as it boots it.
type WorkbenchSpec struct {
	SessionRoot string `json:"session_root,omitempty"`
	Socket      string `json:"socket"`
	Runner      string `json:"runner,omitempty"`
	Resume      bool   `json:"resume,omitempty"`
	LinearIssue string `json:"linear_issue,omitempty"`
	// Empty when the editor could not be resolved, which costs the document chip
	// and must not keep the window shut.
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
	if spec.Socket == "" {
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
	return detach(argv, env, socket, log, readyTimeout, readyInterval, workbench.Published)
}

func detach(argv, env []string, socket, log string, timeout, interval time.Duration,
	ready func(string) bool,
) error {
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

	if err := waitReady(socket, exited, timeout, interval, ready); err != nil {
		// A workbench that never answered has no window and no way to be found
		// again, so it does not get to linger.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return fmt.Errorf(workbenchFailureFormat, err, log)
	}
	return nil
}

// waitReady blocks until the process endpoint is published, the child dies, or
// the deadline passes. Readiness is checked before the child's fate so a process
// that published and then exited still counts as started.
func waitReady(socket string, exited <-chan error, timeout, interval time.Duration,
	ready func(string) bool,
) error {
	deadline := time.After(timeout)
	for {
		if ready(socket) {
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
