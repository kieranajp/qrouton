package core

import (
	"context"
	"os"
	"time"
)

type terminals struct {
	emit     Emitter
	registry *registry
	sample   func(*PTY) (busy, ok bool)
}

func newTerminals(emit Emitter, reg *registry) *terminals {
	return &terminals{emit: emit, registry: reg, sample: (*PTY).Foreground}
}

// start serves the page: it launches the command if nothing has, and otherwise
// attaches the freshly mounted view to the running or finished process.
func (t *terminals) start(id string, cols, rows int) error {
	terminal, started, err := t.fork(id, cols, rows, true)
	if err != nil || started {
		return err
	}
	return t.attach(id, terminal, cols, rows)
}

// launch forks a terminal window's command unless it already has a process.
func (t *terminals) launch(id string, cols, rows int) error {
	_, _, err := t.fork(id, cols, rows, false)
	return err
}

func (t *terminals) fork(id string, cols, rows int, served bool) (*terminalContent, bool, error) {
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
		process, err := StartPTY(window.opts.Command, terminalEnv(), window.opts.Cwd, cols, rows)
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
			terminal.stream.Lock()
			terminal.buffer.Write(b)
			t.emit(windowDataEvent+id, EncodeChunk(b))
			terminal.stream.Unlock()
		},
		func(code int) { t.exited(id, code) },
	)
	return terminal, true, nil
}

// attach resets a freshly mounted view to the retained byte stream before any
// later chunk can pass it. Resize under the same ordering lock: a shell
// repainting for SIGWINCH must follow replay. A finished process has no PTY
// worth resizing, and its output must still replay.
func (t *terminals) attach(id string, terminal *terminalContent, cols, rows int) error {
	terminal.stream.Lock()
	defer terminal.stream.Unlock()
	running := !terminal.process.Done()
	if running {
		if err := terminal.process.Resize(cols, rows); err != nil {
			return err
		}
	}
	chunk := EncodeChunk(terminal.buffer.Bytes())
	chunk.Replay = true
	chunk.First = running && !terminal.served
	t.emit(windowDataEvent+id, chunk)
	terminal.served = true
	return nil
}

func (t *terminals) write(id, encoded string) error {
	data, err := DecodeInput(encoded)
	if err != nil {
		return err
	}
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

// exited applies the lifecycle rule: a clean exit closes the window, a failure
// leaves it open so the error stays readable.
func (t *terminals) exited(id string, code int) {
	t.emit(windowExitEvent+id, code)
	var owner *SessionState
	var closeOnExit bool
	if err := t.registry.with(id, func(window *agentWindow) error {
		terminal, ok := window.terminal()
		if !ok {
			return ErrNotATerminal
		}
		terminal.exit = &code
		owner, closeOnExit = window.session, window.opts.CloseOnExit
		return nil
	}); err != nil {
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
