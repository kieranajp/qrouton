package desktop

import (
	"encoding/base64"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type terminals struct {
	emit     emitter
	registry *registry
}

func newTerminals(emit emitter, reg *registry) *terminals {
	return &terminals{emit: emit, registry: reg}
}

// start launches a terminal window's command once its page has measured itself.
// The page calls it on load, so a reload must not fork a second process.
func (t *terminals) start(id string, cols, rows int) error {
	var started *ptyProcess
	var buffer *ring
	err := t.registry.with(id, func(window *agentWindow) error {
		// A document has no command, and a page that asks anyway must not take the
		// workbench down with it.
		if window.opts.Kind != workbench.KindTerminal {
			return ErrNotATerminal
		}
		if window.process != nil {
			return nil
		}
		process, err := startPTY(window.opts.Command, terminalEnv(), window.opts.Cwd, cols, rows)
		if err != nil {
			return err
		}
		window.process = process
		started, buffer = process, window.buffer
		return nil
	})
	// started is nil when the window already had a process, which a reload does.
	if err != nil || started == nil {
		return err
	}
	go started.pump(
		func(b []byte) {
			buffer.write(b)
			t.emit(windowDataEvent+id, base64.StdEncoding.EncodeToString(b))
		},
		func(code int) { t.exited(id, code) },
	)
	return nil
}

func (t *terminals) write(id, encoded string) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
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
		process = window.process
		return nil
	})
	return process, err
}

// exited applies the lifecycle rule: a clean exit closes the window, a failure
// leaves it open so the error stays readable.
func (t *terminals) exited(id string, code int) {
	t.emit(windowExitEvent+id, code)
	var exited *agentWindow
	if err := t.registry.with(id, func(window *agentWindow) error {
		window.exit = &code
		exited = window
		return nil
	}); err != nil {
		return
	}
	if code == 0 && exited.opts.CloseOnExit {
		t.registry.discard(id)
		return
	}
	t.registry.announce(exited.session)
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

// text renders the buffer as the agent reads it: escape sequences stripped, and
// each line reduced to what a carriage return last left on it.
func (r *ring) text(full bool) string {
	r.mu.Lock()
	raw := string(r.buf)
	r.mu.Unlock()

	lines := strings.Split(strings.ReplaceAll(ansi.Strip(raw), "\r\n", "\n"), "\n")
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
