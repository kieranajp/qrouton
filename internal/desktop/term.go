package desktop

import (
	"encoding/base64"

	"github.com/kieranajp/qrouton/internal/workbench"
)

type emitter func(event string, payload any)

// Term is the conversation terminals' Go half; a page calls these methods over
// the Wails bridge, naming the terminal it draws. A terminal's child is a
// session's agent supervisor, so its stdio is the PTY that runner inherits.
type Term struct {
	sessions *Sessions
	emit     emitter

	exited func(state *sessionState, code int)
}

func newTerm(reg *Sessions, emit emitter) *Term {
	return &Term{sessions: reg, emit: emit}
}

// whenChildExits registers what happens once a conversation's process ends. Set
// it before the page can call Start.
func (t *Term) whenChildExits(f func(state *sessionState, code int)) { t.exited = f }

func (t *Term) Start(id string, cols, rows int) error {
	state, ok := t.sessions.byTerminal(id)
	if !ok {
		return noSuchTerminal(id)
	}
	return state.start(t.emit, t.exited, cols, rows)
}

func (t *Term) Write(id, encoded string) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	state, ok := t.sessions.byTerminal(id)
	if !ok {
		return noSuchTerminal(id)
	}
	return state.write(data)
}

func (t *Term) Resize(id string, cols, rows int) error {
	state, ok := t.sessions.byTerminal(id)
	if !ok {
		return nil
	}
	return state.resize(cols, rows)
}

func withTerminalEnv(env []string) []string {
	env = workbench.WithoutEnv(env, noColorEnvVar)
	return workbench.WithEnv(workbench.WithEnv(env, termEnvVar, termValue), colorTermEnvVar, colorTermValue)
}
