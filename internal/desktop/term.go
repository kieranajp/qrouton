package desktop

import (
	"encoding/base64"
	"sync"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// emitter delivers a payload to the windows' pages.
type emitter func(event string, payload any)

// Term is the conversation terminal's Go half; the page calls these methods
// over the Wails bridge by their fully qualified names. Its child is the agent
// supervisor, whose stdio is therefore the PTY the runner inherits.
type Term struct {
	argv []string
	env  []string
	dir  string
	emit emitter

	exited func(code int)

	mu      sync.Mutex
	process *ptyProcess
}

func newTerm(opts Options, emit emitter) *Term {
	env := workbench.WithEnv(opts.Env, termEnvVar, termValue)
	return &Term{
		argv: opts.Argv,
		env:  workbench.WithEnv(env, colorTermEnvVar, colorTermValue),
		dir:  opts.SessionRoot,
		emit: emit,
	}
}

// whenChildExits registers what happens once the conversation's process ends.
// Set it before the page can call Start.
func (t *Term) whenChildExits(f func(code int)) { t.exited = f }

// Start launches the supervisor under a PTY sized to the terminal displaying
// it. The page calls it on load, so a reload must not fork a second agent.
func (t *Term) Start(cols, rows int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.process != nil {
		return nil
	}
	process, err := startPTY(t.argv, t.env, t.dir, cols, rows)
	if err != nil {
		return err
	}
	t.process = process
	// Base64 because a raw PTY chunk is not valid UTF-8 at its boundary and
	// JSON marshalling corrupts it.
	go process.pump(
		func(b []byte) { t.emit(ptyDataEvent, base64.StdEncoding.EncodeToString(b)) },
		t.childExited,
	)
	return nil
}

func (t *Term) childExited(code int) {
	t.emit(ptyExitEvent, code)
	if t.exited != nil {
		t.exited(code)
	}
}

func (t *Term) Write(encoded string) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	t.mu.Lock()
	process := t.process
	t.mu.Unlock()
	if process == nil {
		return ErrTerminalNotStarted
	}
	return process.write(data)
}

// Resize retells the child how big its terminal is, so dragging the window edge
// reflows the agent's layout.
func (t *Term) Resize(cols, rows int) error {
	t.mu.Lock()
	process := t.process
	t.mu.Unlock()
	if process == nil {
		return nil
	}
	return process.resize(cols, rows)
}

// Stop ends the session's process tree.
func (t *Term) Stop() {
	t.mu.Lock()
	process := t.process
	t.process = nil
	t.mu.Unlock()
	if process != nil {
		process.stop()
	}
}
