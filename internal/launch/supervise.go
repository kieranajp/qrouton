package launch

import (
	"context"
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
// it is killed outright. desktop has its own for the PTYs it owns; the two bound
// different processes and need not agree.
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
	initialPrompt, err := takeInitialPrompt(dir)
	if err != nil {
		return err
	}
	if initialPrompt != "" {
		resume = false
	}
	relaunch := make(chan os.Signal, 1)
	signal.Notify(relaunch, syscall.SIGUSR1)
	defer signal.Stop(relaunch)
	generation := firstRunnerGeneration()
	if generation == 0 {
		generation = 1
	}
	for {
		if err := StampAssets(dir); err != nil {
			return err
		}
		// An escalation hands over the brief, not the conversation. The marker is on
		// disk rather than in this loop, so a restart between the escalation and
		// this launch still starts the orchestrator fresh.
		if tookHandoff(dir) {
			resume = false
		}
		notice, err := takeAgentNotice(dir)
		if err != nil {
			return err
		}
		prompt := initialPrompt
		if resume {
			prompt = notice
		}
		if err := announceRunnerGeneration(handle, r.ID, generation); err != nil {
			return err
		}
		argv, env, err := runnerLaunch(r, qroutonBin, dir, editor, handle, generation, resume, prompt)
		if err != nil {
			return err
		}
		initialPrompt = ""
		env = workbench.WithEnv(env, EditorEnvVar, editor.Marshal())
		signalled, err := runAgent(argv, env, dir, relaunch)
		if err != nil || !signalled {
			return err
		}
		// Every later relaunch keeps the conversation: de-escalation, and a second
		// trip through the picker that merely adds repositories.
		generation++
		resume = true
	}
}

var firstRunnerGeneration = func() uint64 { return uint64(time.Now().UnixNano()) }

var announceRunnerGeneration = func(handle workbench.Handle, provider string, generation uint64) error {
	ctx, cancel := context.WithTimeout(context.Background(), generationSignalTimeout)
	defer cancel()
	return handle.RunnerGeneration(ctx, provider, generation)
}

func takeInitialPrompt(dir string) (string, error) {
	return takePrompt(sessionpaths.InitialPrompt(dir))
}

func takeAgentNotice(dir string) (string, error) {
	return takePrompt(sessionpaths.AgentNotice(dir))
}

func takePrompt(path string) (string, error) {
	prompt, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return string(prompt), nil
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
