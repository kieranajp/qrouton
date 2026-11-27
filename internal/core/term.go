package core

// Term is the conversation terminals' Go half; a page calls these methods over
// the Wails bridge, naming the terminal it draws. A terminal's child is a
// session's agent supervisor, so its stdio is the PTY that runner inherits.
type Term struct {
	sessions *Sessions
	events   Events

	exited func(state *SessionState, code int)
}

func newTerm(reg *Sessions, events Events) *Term {
	return &Term{sessions: reg, events: events}
}

// whenChildExits registers what happens once a conversation's process ends. Set
// it before the page can call Attach.
func (t *Term) whenChildExits(f func(state *SessionState, code int)) { t.exited = f }

func (t *Term) Attach(id string, size *Size, from *uint64, deliver func(Output)) error {
	state, ok := t.sessions.ByTerminal(id)
	if !ok {
		return noSuchTerminal(id)
	}
	return state.attach(t.events, t.exited, size, from, deliver)
}

func (t *Term) Write(id string, data []byte) error {
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
