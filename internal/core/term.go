package core

// Term is the conversation terminals' Go half; a page calls these methods over
// the Wails bridge, naming the terminal it draws. A terminal's child is a
// session's agent supervisor, so its stdio is the PTY that runner inherits.
type Term struct {
	sessions *Sessions
	emit     Emitter

	exited func(state *SessionState, code int)
}

func newTerm(reg *Sessions, emit Emitter) *Term {
	return &Term{sessions: reg, emit: emit}
}

// whenChildExits registers what happens once a conversation's process ends. Set
// it before the page can call Start.
func (t *Term) whenChildExits(f func(state *SessionState, code int)) { t.exited = f }

func (t *Term) Start(id string, cols, rows int) error {
	state, ok := t.sessions.ByTerminal(id)
	if !ok {
		return noSuchTerminal(id)
	}
	return state.start(t.emit, t.exited, cols, rows)
}

func (t *Term) Write(id, encoded string) error {
	data, err := DecodeInput(encoded)
	if err != nil {
		return err
	}
	state, ok := t.sessions.ByTerminal(id)
	if !ok {
		return noSuchTerminal(id)
	}
	return state.write(data)
}

func (t *Term) Resize(id string, cols, rows int) error {
	state, ok := t.sessions.ByTerminal(id)
	if !ok {
		return nil
	}
	return state.resize(cols, rows)
}
