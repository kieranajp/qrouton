package desktop

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type agentWindow struct {
	opts          workbench.WindowOptions
	session       *sessionState
	seq           int
	buffer        *ring
	process       *ptyProcess
	viewport      *workbench.DocumentViewport
	viewportEpoch uint64
	viewportSeq   uint64
	// read is the file as the window last saw it, so a rescan can tell a
	// rewritten document from an untouched one.
	read struct {
		at   time.Time
		size int64
	}
	// exit is nil while the process is still running.
	exit *int
}

// registry holds the open tabs and which one each session has selected. Every
// field of a window that changes after it opens is guarded by mu, so a reader
// reaches one through with or each and never through the map.
type registry struct {
	emit     emitter
	sessions *Sessions
	// sourceMu serialises the check and open for windows with a Source.
	sourceMu sync.Mutex

	mu       sync.Mutex
	seq      int
	open     map[string]*agentWindow
	selected map[*sessionState]string
}

func newRegistry(emit emitter, sessions *Sessions) *registry {
	return &registry{
		emit: emit, sessions: sessions,
		open:     map[string]*agentWindow{},
		selected: map[*sessionState]string{},
	}
}

func (r *registry) shown() *sessionState { return r.sessions.current() }

// with runs fn against one open window under the registry lock.
func (r *registry) with(id string, fn func(*agentWindow) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	window, ok := r.open[id]
	if !ok {
		return noSuchWindow(id)
	}
	return fn(window)
}

// each runs fn against every open window under the registry lock.
func (r *registry) each(fn func(id string, window *agentWindow)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, window := range r.open {
		fn(id, window)
	}
}

func (r *registry) openWindow(owner *sessionState, opts workbench.WindowOptions) (string, error) {
	if opts.Source != "" {
		r.sourceMu.Lock()
		defer r.sourceMu.Unlock()
		if id, ok := r.showing(owner, opts.Source); ok {
			r.discard(id)
		}
	}
	return r.spawn(owner, opts, true)
}

// openStructural opens a tab the workbench owns rather than the agent. It
// leaves the selection alone: every caller either selects the tab itself once
// it has the id, or is the unasked-for first shell, which has nothing to steal
// the selection from.
func (r *registry) openStructural(owner *sessionState, opts workbench.WindowOptions) (string, error) {
	return r.spawn(owner, opts, false)
}

// showOrOpen selects the window already showing source, or the one open leaves
// behind — so a single click both opens and selects.
func (r *registry) showOrOpen(owner *sessionState, source string, open func() (string, error)) (string, error) {
	r.sourceMu.Lock()
	defer r.sourceMu.Unlock()
	if id, ok := r.showing(owner, source); ok {
		return id, r.selectWindow(owner, id)
	}
	id, err := open()
	if err != nil {
		return "", err
	}
	return id, r.selectWindow(owner, id)
}

func (r *registry) selectBySlug(slug, id string) error {
	owner := r.sessions.bySlug(slug)
	if owner == nil {
		return unknownSession(slug)
	}
	return r.selectWindow(owner, id)
}

func (r *registry) selectWindow(owner *sessionState, id string) error {
	r.mu.Lock()
	window, ok := r.open[id]
	if !ok || window.session != owner {
		r.mu.Unlock()
		return noSuchWindow(id)
	}
	r.selected[owner] = id
	r.mu.Unlock()
	r.announce(owner)
	return nil
}

func (r *registry) showing(owner *sessionState, source string) (string, bool) {
	if source == "" {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, window := range r.open {
		if window.session == owner && window.opts.Source == source {
			return id, true
		}
	}
	return "", false
}

func (r *registry) spawn(owner *sessionState, opts workbench.WindowOptions, selects bool) (string, error) {
	if opts.Kind == workbench.KindTerminal && len(opts.Command) == 0 {
		return "", ErrNoWindowCommand
	}
	r.mu.Lock()
	r.seq++
	id := fmt.Sprintf(windowIDFormat, r.seq)
	window := &agentWindow{opts: opts, session: owner, seq: r.seq}
	if opts.Kind == workbench.KindTerminal {
		window.buffer = &ring{limit: windowScrollback}
	}
	beginDocument(window)
	r.open[id] = window
	if selects {
		r.selected[owner] = id
	}
	r.mu.Unlock()

	r.announce(owner)
	return id, nil
}

func (r *registry) readWindow(id string, full bool) (string, error) {
	var text string
	var buffer *ring
	if err := r.with(id, func(window *agentWindow) error {
		if window.opts.Kind == workbench.KindDocument {
			text = window.opts.Content
			return nil
		}
		buffer = window.buffer
		return nil
	}); err != nil {
		return "", err
	}
	if buffer == nil {
		return text, nil
	}
	return buffer.text(full), nil
}

func (r *registry) window(id string) (*agentWindow, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	window, ok := r.open[id]
	return window, ok
}

func (r *registry) exists(id string) bool {
	_, ok := r.window(id)
	return ok
}

func (r *registry) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.open))
	for id := range r.open {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// discard forgets a window and stops whatever it was running.
func (r *registry) discard(id string) {
	r.mu.Lock()
	window, ok := r.open[id]
	delete(r.open, id)
	var process *ptyProcess
	if ok {
		process = window.process
		if r.selected[window.session] == id {
			if fallback := r.oldest(window.session); fallback != "" {
				r.selected[window.session] = fallback
			} else {
				delete(r.selected, window.session)
			}
		}
	}
	r.mu.Unlock()
	if !ok {
		return
	}
	if process != nil {
		process.stop()
	}
	r.announce(window.session)
}

func (r *registry) oldest(owner *sessionState) string {
	oldestID := ""
	oldestSeq := 0
	for id, window := range r.open {
		if window.session == owner && (oldestID == "" || window.seq < oldestSeq) {
			oldestID = id
			oldestSeq = window.seq
		}
	}
	return oldestID
}

// stop tears down one session's windows, so retiring it leaves the rest alone.
func (r *registry) stop(owner *sessionState) {
	for _, id := range r.list() {
		if window, ok := r.window(id); ok && window.session == owner {
			r.discard(id)
		}
	}
}

func (r *registry) stopAll() {
	for _, id := range r.list() {
		r.discard(id)
	}
}

// announce tells every page what one session has open. The payload names that
// session, and a page draws only the tab strip of the one it is showing.
func (r *registry) announce(owner *sessionState) {
	r.emit(windowsEvent, r.surfaces(owner))
}

// drawnWindow is one window as its surface draws it.
type drawnWindow struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Badge  string `json:"badge,omitempty"`
	Kind   string `json:"kind"`
	Status string `json:"status,omitempty"`

	// Artifact colours the badge. Kind is the window's own, terminal or
	// document, which says nothing about which artifact a document holds.
	Artifact string `json:"artifact,omitempty"`
}

// surfaces names one session's open tabs, oldest first so the shell stays
// leftmost.
type surfaces struct {
	Session  string        `json:"session"`
	Selected string        `json:"selected"`
	Tabs     []drawnWindow `json:"tabs"`
}

func (r *registry) surfacesBySlug(slug string) surfaces {
	return r.surfaces(r.sessions.bySlug(slug))
}

func (r *registry) surfaces(owner *sessionState) surfaces {
	r.mu.Lock()
	defer r.mu.Unlock()
	live := make([]*agentWindow, 0, len(r.open))
	for _, window := range r.open {
		if window.session == owner {
			live = append(live, window)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].seq < live[j].seq })
	out := surfaces{Session: owner.slug(), Selected: r.selected[owner], Tabs: []drawnWindow{}}
	for _, window := range live {
		drawn := drawnWindow{
			ID: fmt.Sprintf(windowIDFormat, window.seq), Label: window.opts.Label,
			Badge: window.opts.Badge, Kind: string(window.opts.Kind), Status: tabStatus(window),
		}
		if window.opts.Badge != "" {
			drawn.Artifact = status.DocumentKind(window.opts.Source)
		}
		out.Tabs = append(out.Tabs, drawn)
	}
	return out
}

func tabStatus(window *agentWindow) string {
	switch {
	case window.opts.Attention:
		return tabStatusWaiting
	case window.opts.Kind != workbench.KindTerminal:
		return ""
	case window.exit == nil:
		return tabStatusRunning
	case *window.exit == 0:
		return tabStatusSucceeded
	default:
		return tabStatusFailed
	}
}
