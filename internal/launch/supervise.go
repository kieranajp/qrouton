package launch

// The agent pane's command is a supervisor rather than the runner itself, so
// escalation can replace the agent's process without touching pane geometry:
// the picker (or `qrouton mode`) rewrites the manifest and signals, and the
// supervisor re-stamps and relaunches from the fresh value. Escalation
// relaunches fresh — the handoff brief, not the conversation, carries over —
// while de-escalation keeps the conversation with the runner's continue flag.

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// terminateGrace is how long a signalled runner gets to exit on SIGTERM before
// it is killed outright.
const terminateGrace = 3 * time.Second

// How long the quick-reference panel waits for someone to be looking before it
// floats anyway, and how long it then keeps waiting to correct the geometry it
// floated with. The launcher creates the session detached and only then
// attaches, so at supervisor start the client is usually a beat away; vars so
// tests need not wait out the ceiling.
var (
	clientPollInterval = 100 * time.Millisecond
	clientWaitTimeout  = 5 * time.Second
	lateClientTimeout  = 2 * time.Minute
)

// showHelp floats the quick-reference panel over a freshly attached session; a
// package variable so tests can stub the pane driver out, as they do runAgent.
var showHelp = spawnHelp

// helpPaneHost resolves the driver spawnHelp floats the panel through. A
// package variable for the same reason as showHelp: the real one needs zellij
// on PATH and a live session, and spawnHelp's own logic is worth testing
// without either.
var helpPaneHost = func(h mux.Handle) (mux.PaneHost, error) { return h.PaneHost() }

// spawnHelp waits for a client and then floats the panel. Every failure here is
// swallowed: a missing greeting is not a reason to fail the session it greets.
//
// Waiting first, rather than floating first and correcting after, keeps the
// common case flicker-free: the client is normally a beat away, and a panel
// sized correctly from birth never visibly snaps.
func spawnHelp(dir string, h mux.Handle, warning string) {
	host, err := helpPaneHost(h)
	if err != nil {
		return
	}
	onTime := waitAttached(host, clientWaitTimeout)
	opts := HelpSpawn(dir, warning)
	id, err := host.Spawn(context.Background(), opts)
	if err != nil || onTime {
		return
	}
	// Floated with nobody looking, so its percentages resolved against the
	// server's own default viewport rather than a real terminal. That used to be
	// permanent, which is why this settled for "squished beats absent"; now the
	// geometry is re-applied the moment someone does attach.
	if waitAttached(host, lateClientTimeout) {
		_ = host.Reposition(context.Background(), id, opts.Geometry)
	}
}

// waitAttached polls until a client is viewing the session, reporting whether
// one arrived inside the timeout.
func waitAttached(host mux.PaneHost, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if attached(host) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(clientPollInterval)
	}
}

func attached(host mux.PaneHost) bool {
	ctx, cancel := context.WithTimeout(context.Background(), clientWaitTimeout)
	defer cancel()
	yes, err := host.Attached(ctx)
	return err == nil && yes
}

// Supervise owns the agent pane: it stamps the session's assets, launches the
// runner, and on SIGUSR1 kills it and relaunches from the manifest as it now
// stands. An unsignalled exit is a real exit — the pane closes rather than
// resurrecting the agent.
//
// ponytail: pid file + SIGUSR1 — fine for one supervisor per session; a name
// per pane if a session ever hosts two agent panes.
func Supervise(dir string, r Runner, handle mux.Handle, editor EditorCommand, resume bool) error {
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
	// Greet the session once, in the background: the panel waits for a client
	// and must not hold the agent's own launch up while it does.
	go showHelp(dir, handle, codexWarning(r.Command))
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
		env = mux.WithEnv(env, EditorEnvVar, editor.Marshal())
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
