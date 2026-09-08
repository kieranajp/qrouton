package desktop

import "sync"

// notesRenderer is the slice of the toolkit a presentation needs: a second
// window, a way to reach that window alone, and a way to end it. Quit is
// deliberately absent — closing the notes window must leave the app running.
type notesRenderer interface {
	Open(spec windowSpec) error
	Send(name, event string, payload any)
	Close(name string)
}

// Presenter is the speaker-notes window: a real OS window the presenter can
// drag to a second display, so the notes stay out of a screen share.
type Presenter struct {
	renderer notesRenderer
	emit     emitter

	mu    sync.Mutex
	open  bool
	epoch uint64
	note  noteView
}

func newPresenter(r notesRenderer, emit emitter) *Presenter {
	return &Presenter{renderer: r, emit: emit}
}

// Open puts the notes window on screen, or leaves the one already there alone:
// the window manager matches by name and dedupes nothing, so a second open
// would leave two windows answering to one name.
func (p *Presenter) Open() error {
	p.mu.Lock()
	if p.open {
		p.mu.Unlock()
		return nil
	}
	p.open = true
	p.epoch++
	at := p.epoch
	p.mu.Unlock()

	spec := windowSpec{
		Name:   notesWindowName,
		Title:  notesWindowTitle,
		URL:    notesRoot,
		Width:  notesWindowWidth,
		Height: notesWindowHeight,
		// The keyboard stays with the presenting window, or the press that opened
		// this one would be the last arrow the deck heard.
		Focus:   false,
		OnClose: func() { p.closed(at) },
	}
	if err := p.renderer.Open(spec); err != nil {
		p.mu.Lock()
		p.open = false
		p.mu.Unlock()
		return err
	}
	return nil
}

func (p *Presenter) Close() error {
	p.mu.Lock()
	closing := p.open
	p.open = false
	p.epoch++
	p.mu.Unlock()
	if closing {
		p.renderer.Close(notesWindowName)
	}
	return nil
}

// Show retains the note and puts it in front of the notes window alone. With no
// window up it only retains, so the presenting page can call it on every step.
func (p *Presenter) Show(note noteView) error {
	p.mu.Lock()
	p.note = note
	showing := p.open
	p.mu.Unlock()
	if showing {
		p.renderer.Send(notesWindowName, presenterNotesEvent, note)
	}
	return nil
}

// Note answers the notes page's pull on mount, which closes the race with a
// step dispatched before that page had subscribed.
func (p *Presenter) Note() noteView {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.note
}

// closed is the window going without being asked, which the presenting page's
// own control has to hear about. The toolkit tears a window down after Close
// returns, so a window closing at its own pace names the epoch it was opened
// in and a stale one says nothing.
func (p *Presenter) closed(at uint64) {
	p.mu.Lock()
	announce := p.open && p.epoch == at
	if announce {
		p.open = false
	}
	p.mu.Unlock()
	if announce && p.emit != nil {
		p.emit(presenterClosedEvent, nil)
	}
}
