package core

import (
	"context"
	"os"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

type terminals struct {
	events   Events
	registry *registry
	sample   func(*PTY) (busy, ok bool)
}

func newTerminals(events Events, reg *registry) *terminals {
	return &terminals{events: events, registry: reg, sample: (*PTY).Foreground}
}

// attach serves the page: it launches the command if nothing has, and otherwise
// replays the running or finished process to the freshly mounted view.
func (t *terminals) attach(id string, size *Size, from *uint64, deliver func(Output)) error {
	terminal, started, err := t.fork(id, size, true)
	if err != nil || started {
		return err
	}
	return t.replay(id, terminal, size, from, deliver)
}

// launch forks a terminal window's command unless it already has a process.
func (t *terminals) launch(id string, cols, rows int) error {
	_, _, err := t.fork(id, &Size{Cols: cols, Rows: rows}, false)
	return err
}

// fork starts the command at size unless it already has a process; a nil size
// starts nothing.
func (t *terminals) fork(id string, size *Size, served bool) (*terminalContent, bool, error) {
	var started *PTY
	var terminal *terminalContent
	err := t.registry.with(id, func(window *agentWindow) error {
		// A document has no command, and a page that asks anyway must not take the
		// workbench down with it.
		content, ok := window.terminal()
		if !ok {
			return ErrNotATerminal
		}
		terminal = content
		if content.process != nil {
			return nil
		}
		if size == nil {
			return ErrTerminalNotStarted
		}
		process, err := StartPTY(window.opts.Command, terminalEnv(), window.opts.Cwd, size.Cols, size.Rows)
		if err != nil {
			return err
		}
		content.process = process
		content.served = served
		started = process
		return nil
	})
	if err != nil || started == nil {
		return terminal, false, err
	}
	go started.Pump(
		func(b []byte) {
			terminal.stream.publish(b, func(out Output) { t.events.TerminalOutput(TerminalTab, id, out) })
		},
		func(code int) { t.exited(id, code) },
	)
	return terminal, true, nil
}

// replay resizes under the ordering lock as well: a shell repainting for
// SIGWINCH must follow the replay. A finished process has no PTY worth
// resizing, and its output must still replay.
func (t *terminals) replay(id string, terminal *terminalContent, size *Size, from *uint64, deliver func(Output)) error {
	return terminal.stream.attach(from, func(out Output) error {
		var process *PTY
		var state *workbench.TerminalState
		var first bool
		if err := t.registry.with(id, func(window *agentWindow) error {
			if content, ok := window.terminal(); !ok || content != terminal {
				return noSuchWindow(id)
			}
			process, state = terminal.process, terminal.state()
			first = !terminal.served
			return nil
		}); err != nil {
			return err
		}
		running := !process.Done()
		if size != nil && state.ExitCode == nil && running {
			if err := process.Resize(size.Cols, size.Rows); err != nil {
				return err
			}
		}
		out.First, out.ExitCode = running && first, state.ExitCode
		deliver(out)
		_ = t.registry.with(id, func(window *agentWindow) error {
			if content, ok := window.terminal(); ok && content == terminal {
				terminal.served = true
			}
			return nil
		})
		return nil
	})
}

func (t *terminals) write(id string, data []byte) error {
	process, err := t.process(id)
	if err != nil {
		return err
	}
	if process == nil {
		return ErrTerminalNotStarted
	}
	return process.Write(data)
}

func (t *terminals) resize(id string, cols, rows int) error {
	process, err := t.process(id)
	if err != nil || process == nil {
		return nil
	}
	return process.Resize(cols, rows)
}

func (t *terminals) process(id string) (*PTY, error) {
	var process *PTY
	err := t.registry.with(id, func(window *agentWindow) error {
		if terminal, ok := window.terminal(); ok {
			process = terminal.process
		}
		return nil
	})
	return process, err
}

func (t *terminals) exited(id string, code int) {
	var terminal *terminalContent
	if err := t.registry.with(id, func(window *agentWindow) error {
		var ok bool
		terminal, ok = window.terminal()
		if !ok {
			return ErrNotATerminal
		}
		return nil
	}); err != nil {
		return
	}
	var owner *SessionState
	var closeOnExit bool
	var err error
	terminal.stream.ordered(func() {
		err = t.registry.with(id, func(window *agentWindow) error {
			if content, ok := window.terminal(); !ok || content != terminal {
				return noSuchWindow(id)
			}
			terminal.exit = &code
			owner, closeOnExit = window.session, window.opts.CloseOnExit
			return nil
		})
		if err == nil {
			t.events.TerminalExit(TerminalTab, id, code)
		}
	})
	if err != nil {
		return
	}
	if code == 0 && closeOnExit {
		t.registry.discard(id)
		return
	}
	t.registry.announce(owner)
}

func (t *terminals) watchForeground(ctx context.Context) {
	ticker := time.NewTicker(foregroundInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.pollForeground()
		}
	}
}

func (t *terminals) pollForeground() {
	shells := map[string]*PTY{}
	t.registry.each(func(id string, window *agentWindow) {
		if terminal, ok := window.terminal(); ok && terminal.foreground && terminal.process != nil && terminal.exit == nil {
			shells[id] = terminal.process
		}
	})
	changed := map[*SessionState]bool{}
	for id, process := range shells {
		busy, ok := t.sample(process)
		if !ok {
			continue
		}
		_ = t.registry.with(id, func(window *agentWindow) error {
			if terminal, ok := window.terminal(); ok && terminal.exit == nil && terminal.busy != busy {
				terminal.busy = busy
				changed[window.session] = true
			}
			return nil
		})
	}
	for owner := range changed {
		t.registry.announce(owner)
	}
}

func terminalEnv() []string { return WithTerminalEnv(os.Environ()) }
