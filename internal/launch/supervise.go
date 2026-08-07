package launch

// The conversation terminal's command is a supervisor rather than the runner
// itself, so escalation can replace the agent's process without touching the
// terminal: the picker (or `qrouton mode`) rewrites the manifest and signals,
// and the supervisor re-stamps and relaunches from the fresh value. Escalation
// relaunches fresh — the handoff brief, not the conversation, carries over —
// while de-escalation keeps the conversation with the runner's continue flag.

import (
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// terminateGrace is how long a signalled runner gets to exit on SIGTERM before
// it is killed outright.
const terminateGrace = 3 * time.Second

// Supervise owns the conversation terminal: it stamps the session's assets,
// launches the runner, and on SIGUSR1 kills it and relaunches from the manifest
// as it now stands. An unsignalled exit is a real exit — the terminal ends
// rather than resurrecting the agent.
//
// ponytail: pid file + SIGUSR1 — fine for one supervisor per session; a name
// per terminal if a session ever hosts two agents.
func Supervise(dir string, r Runner, handle workbench.Handle, editor EditorCommand, resume bool) error {
	if err := writePID(dir); err != nil {
		return err
	}
	defer os.Remove(sessionpaths.AgentPID(dir))
	qroutonBin, err := os.Executable()
	if err != nil {
		return err
	}
	relaunch := make(chan os.Signal, 1)
	signal.Notify(relaunch, syscall.SIGUSR1)
	defer signal.Stop(relaunch)
	for {
		if err := StampAssets(dir); err != nil {
			return err
		}
		// An escalation hands over the brief, not the conversation. The marker is
		// on disk, so a restart between the escalation and this launch still gets
		// the fresh context the handoff promised — the decision used to live in
		// this loop's memory, and any relaunch it did not perform resumed a
		// conversation the new orchestrator was never meant to see.
		if tookHandoff(dir) {
			resume = false
		}
		argv, env, err := runnerLaunch(r, qroutonBin, dir, editor, handle, resume)
		if err != nil {
			return err
		}
		env = workbench.WithEnv(env, EditorEnvVar, editor.Marshal())
		signalled, err := runAgent(argv, env, dir, relaunch)
		if err != nil || !signalled {
			return err
		}
		// Every later relaunch keeps the conversation: de-escalation, and a second
		// trip through the picker that merely adds repositories.
		resume = true
	}
}

// tookHandoff consumes the pending-handoff marker, reporting whether this launch
// is the one that owes a fresh conversation. Remove succeeds for exactly one
// caller, so the marker cannot be claimed twice.
func tookHandoff(dir string) bool {
	return os.Remove(sessionpaths.HandoffPending(dir)) == nil
}

// runAgent runs one runner until it exits on its own (false) or a relaunch
// signal arrives (true), terminating the child first in the latter case. A
// package variable so tests can swap the exec for a closure.
var runAgent = func(argv, env []string, dir string, relaunch <-chan os.Signal) (bool, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return false, err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		return false, err
	case <-relaunch:
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(terminateGrace):
			_ = cmd.Process.Kill()
			<-exited
		}
		return true, nil
	}
}

// writePID records the supervisor's pid for SignalSupervisor.
func writePID(dir string) error {
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(sessionpaths.AgentPID(dir), []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// SignalSupervisor pokes a session's agent supervisor to relaunch its runner
// from the fresh manifest. Best-effort: with no pid file or no live
// supervisor, the mode change still takes effect on the next launch.
func SignalSupervisor(dir string) {
	b, err := os.ReadFile(sessionpaths.AgentPID(dir))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGUSR1)
}
