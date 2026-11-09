package desktop

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type terminals struct {
	emit     emitter
	registry *registry
	sample   func(*ptyProcess) (busy, ok bool)
}

type terminalChunk struct {
	Encoded  string `json:"encoded"`
	Replay   bool   `json:"replay,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	// First marks a running process's first replay, whose queries the page
	// answers as if live.
	First bool `json:"first,omitempty"`
}

// Base64 because a raw PTY chunk is not valid UTF-8 at its boundary and JSON
// marshalling corrupts it.
func encodeChunk(b []byte) terminalChunk {
	return terminalChunk{Encoded: base64.StdEncoding.EncodeToString(b)}
}

func decodeInput(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

func newTerminals(emit emitter, reg *registry) *terminals {
	return &terminals{emit: emit, registry: reg, sample: (*ptyProcess).foreground}
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
	var started *ptyProcess
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
		process, err := startPTY(window.opts.Command, terminalEnv(), window.opts.Cwd, cols, rows)
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
	go started.pump(
		func(b []byte) {
			terminal.stream.Lock()
			terminal.buffer.write(b)
			t.emit(windowDataEvent+id, encodeChunk(b))
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
	var process *ptyProcess
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
	running := !process.done()
	if state.ExitCode == nil && running {
		if err := process.resize(cols, rows); err != nil {
			return err
		}
	}
	chunk := encodeChunk(terminal.buffer.bytes())
	chunk.Replay, chunk.First, chunk.ExitCode = true, running && first, state.ExitCode
	t.emit(windowDataEvent+id, chunk)
	_ = t.registry.with(id, func(window *agentWindow) error {
		if content, ok := window.terminal(); ok && content == terminal {
			terminal.served = true
		}
		return nil
	})
	return nil
}

func (t *terminals) write(id, encoded string) error {
	data, err := decodeInput(encoded)
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
	return process.write(data)
}

func (t *terminals) resize(id string, cols, rows int) error {
	process, err := t.process(id)
	if err != nil || process == nil {
		return nil
	}
	return process.resize(cols, rows)
}

func (t *terminals) process(id string) (*ptyProcess, error) {
	var process *ptyProcess
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
	terminal.stream.Lock()
	var owner *sessionState
	var closeOnExit bool
	err := t.registry.with(id, func(window *agentWindow) error {
		if content, ok := window.terminal(); !ok || content != terminal {
			return noSuchWindow(id)
		}
		terminal.exit = &code
		owner, closeOnExit = window.session, window.opts.CloseOnExit
		return nil
	})
	if err == nil {
		t.emit(windowExitEvent+id, code)
	}
	terminal.stream.Unlock()
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
	shells := map[string]*ptyProcess{}
	t.registry.each(func(id string, window *agentWindow) {
		if terminal, ok := window.terminal(); ok && terminal.foreground && terminal.process != nil && terminal.exit == nil {
			shells[id] = terminal.process
		}
	})
	changed := map[*sessionState]bool{}
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

func terminalEnv() []string { return withTerminalEnv(os.Environ()) }

type ring struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (r *ring) write(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, b...)
	if len(r.buf) > r.limit {
		r.buf = r.buf[len(r.buf)-r.limit:]
	}
}

func (r *ring) bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.buf...)
}

func (r *ring) text(full bool) string {
	return terminalText(r.bytes(), full)
}

func terminalText(raw []byte, full bool) string {
	lines := strings.Split(strings.ReplaceAll(ansi.Strip(string(raw)), "\r\n", "\n"), "\n")
	for i, line := range lines {
		if at := strings.LastIndex(line, "\r"); at >= 0 {
			lines[i] = line[at+1:]
		}
	}
	if !full && len(lines) > windowScreenLines {
		lines = lines[len(lines)-windowScreenLines:]
	}
	return strings.Join(lines, "\n")
}
