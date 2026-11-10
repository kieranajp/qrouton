package core

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/randomid"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// agentWindow is one tab: what it was opened with, whose session it belongs to,
// where it sits in the strip, and the content behind it.
type agentWindow struct {
	opts    workbench.WindowOptions
	session *SessionState
	seq     int
	order   int
	content windowContent
	// asset names a document window in the deck asset route's URLs. Window ids
	// are sequential, so one session could otherwise guess another's.
	asset string
}

// windowContent is the half of a tab that differs by kind. Callers ask for the
// kind they can work with, so document state cannot be reached through a
// terminal or the other way round.
type windowContent interface {
	// tabStatus is the mark the tab strip draws, and empty where a kind has none.
	tabStatus() string
}

type terminalContent struct {
	buffer *Ring
	// stream orders a terminal's retained bytes and emitted chunks. A remounted
	// page can therefore reset to one replay without racing live output around it.
	stream  sync.Mutex
	process *PTY
	// served is set once a page has shown the output, live or replayed.
	served bool
	// exit is nil while the process is still running.
	exit *int
	// foreground marks a user shell, which is busy only while a job holds its
	// terminal. busy is the last read of its terminal.
	foreground bool
	busy       bool
}

// documentContent holds a rendered tab's measurements. viewport is nil for a
// format the page cannot map back to source lines.
type documentContent struct {
	viewport      *workbench.DocumentViewport
	viewportEpoch uint64
	viewportSeq   uint64
	assets        []workbench.AssetRef
	currentAsset  int
	assetRevision uint64
	// read is the file as the window last saw it, so a rescan can tell a
	// rewritten document from an untouched one.
	read struct {
		at   time.Time
		size int64
	}
}

func (t *terminalContent) tabStatus() string {
	if t.exit == nil && t.foreground && !t.busy {
		return tabStatusIdle
	}
	return t.lifecycleStatus()
}

func (t *terminalContent) lifecycleStatus() string {
	switch {
	case t.exit == nil:
		return tabStatusRunning
	case *t.exit == 0:
		return tabStatusSucceeded
	default:
		return tabStatusFailed
	}
}

func (*documentContent) tabStatus() string { return "" }

func (window *agentWindow) terminal() (*terminalContent, bool) {
	content, ok := window.content.(*terminalContent)
	return content, ok
}

func (window *agentWindow) document() (*documentContent, bool) {
	content, ok := window.content.(*documentContent)
	return content, ok
}

// registry holds the open tabs and which one each session has selected. Every
// field of a window that changes after it opens is guarded by mu, so a reader
// reaches one through with or each and never through the map.
type registry struct {
	events   Events
	sessions *Sessions
	// sourceMu serialises the check and open for windows with a Source.
	sourceMu     sync.Mutex
	assetFocusMu sync.Mutex

	mu       sync.Mutex
	seq      int
	open     map[string]*agentWindow
	selected map[*SessionState]string
	closed   bool
}

func newRegistry(events Events, sessions *Sessions) *registry {
	return &registry{
		events: events, sessions: sessions,
		open:     map[string]*agentWindow{},
		selected: map[*SessionState]string{},
	}
}

func (r *registry) shown() *SessionState { return r.sessions.Current() }

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

func (r *registry) openWindow(owner *SessionState, opts workbench.WindowOptions) (string, error) {
	if opts.Kind == workbench.KindOverview {
		return "", ErrOverviewWindowReserved
	}
	var err error
	opts, err = admittedWindowOptions(owner, opts)
	if err != nil {
		return "", err
	}
	if opts.Source != "" {
		r.sourceMu.Lock()
		defer r.sourceMu.Unlock()
		if id, ok := r.showing(owner, opts.Source); ok {
			r.discard(id)
		}
	}
	return r.spawn(owner, opts, opts.Select, false)
}

// openStructural opens a tab the workbench owns rather than the agent. It
// leaves the selection alone: every caller either selects the tab itself once
// it has the id, or is the Overview tab, which has nothing to steal
// the selection from.
func (r *registry) openStructural(owner *SessionState, opts workbench.WindowOptions) (string, error) {
	return r.openOwned(owner, opts, false)
}

// openShell opens a user shell, whose tab reports the terminal's foreground job
// rather than spinning for the shell's whole life.
func (r *registry) openShell(owner *SessionState, opts workbench.WindowOptions) (string, error) {
	return r.openOwned(owner, opts, true)
}

func (r *registry) openOwned(owner *SessionState, opts workbench.WindowOptions, foreground bool) (string, error) {
	var err error
	opts, err = admittedWindowOptions(owner, opts)
	if err != nil {
		return "", err
	}
	return r.spawn(owner, opts, false, foreground)
}

func (r *registry) showOrOpen(owner *SessionState, source string, open func() (string, error)) (string, error) {
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
	owner := r.sessions.BySlug(slug)
	if owner == nil {
		return UnknownSession(slug)
	}
	return r.selectWindow(owner, id)
}

func (r *registry) selectWindow(owner *SessionState, id string) error {
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

func (r *registry) reorder(slug, id string, to int) error {
	owner := r.sessions.BySlug(slug)
	if owner == nil {
		return UnknownSession(slug)
	}
	r.mu.Lock()
	window, ok := r.open[id]
	if !ok || window.session != owner {
		r.mu.Unlock()
		return noSuchWindow(id)
	}
	live := r.ordered(owner)
	from := slices.Index(live, window)
	to = min(max(to, 0), len(live)-1)
	if pinned(window) || (to == 0 && pinned(live[0])) {
		r.mu.Unlock()
		return ErrOverviewWindowPinned
	}
	if to == from {
		r.mu.Unlock()
		return nil
	}
	// The session's own order values, handed back out positionally rather than
	// renumbered, so a tab opened later still sorts to the right of them all.
	orders := make([]int, len(live))
	for i, tab := range live {
		orders[i] = tab.order
	}
	for i, tab := range slices.Insert(slices.Delete(live, from, from+1), to, window) {
		tab.order = orders[i]
	}
	r.mu.Unlock()
	r.announce(owner)
	return nil
}

func (r *registry) showing(owner *SessionState, source string) (string, bool) {
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

func (r *registry) spawn(owner *SessionState, opts workbench.WindowOptions, selects, foreground bool) (string, error) {
	if opts.Kind == workbench.KindTerminal && len(opts.Command) == 0 {
		return "", ErrNoWindowCommand
	}
	asset := ""
	if opts.Kind == workbench.KindDocument {
		asset = assetToken()
		if opts.Format == workbench.FormatAssets && asset == "" {
			return "", ErrGalleryAssetToken
		}
	}
	r.mu.Lock()
	if r.closed || owner.admissionClosed() {
		r.mu.Unlock()
		if opts.Format == workbench.FormatAssets {
			return "", ErrAssetGalleryClosed
		}
		return "", ErrSessionUnloaded
	}
	r.seq++
	id := fmt.Sprintf(windowIDFormat, r.seq)
	window := &agentWindow{opts: opts, session: owner, seq: r.seq, order: r.seq, content: contentFor(opts)}
	window.asset = asset
	if terminal, ok := window.terminal(); ok {
		terminal.foreground = foreground
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

// deckDirectory answers the asset route. A token belonging to a window that is
// not a deck resolves to nothing, so the route stays a media server for open
// decks rather than a reader for any window's neighbours.
func (r *registry) deckDirectory(token string) (root, dir string, ok bool) {
	if token == "" {
		return "", "", false
	}
	r.each(func(_ string, window *agentWindow) {
		if ok || window.asset != token || !window.opts.Deck || window.opts.Source == "" {
			return
		}
		home := window.session.Root()
		if home == "" {
			return
		}
		root, dir, ok = home, path.Dir(window.opts.Source), true
	})
	return root, dir, ok
}

func assetToken() string { return randomid.Hex(assetTokenBytes) }

func (r *registry) readWindow(id string, full bool) (workbench.WindowRead, error) {
	var result workbench.WindowRead
	var terminal *terminalContent
	if err := r.with(id, func(window *agentWindow) error {
		if content, ok := window.terminal(); ok {
			terminal = content
			return nil
		}
		result.Text = window.opts.Content
		if rendered, ok := window.document(); ok && window.opts.Format == workbench.FormatAssets {
			result.Text = assetManifest(rendered.assets, rendered.currentAsset)
		}
		return nil
	}); err != nil {
		return workbench.WindowRead{}, err
	}
	if terminal == nil {
		return result, nil
	}
	terminal.stream.Lock()
	err := r.with(id, func(window *agentWindow) error {
		if content, ok := window.terminal(); !ok || content != terminal {
			return noSuchWindow(id)
		}
		result.Terminal = terminal.state()
		return nil
	})
	if err != nil {
		terminal.stream.Unlock()
		return workbench.WindowRead{}, err
	}
	raw := terminal.buffer.Bytes()
	terminal.stream.Unlock()
	result.Text = terminalText(raw, full)
	return result, nil
}

// state copies lifecycle metadata while the registry lock is held.
func (t *terminalContent) state() *workbench.TerminalState {
	state := &workbench.TerminalState{Status: t.lifecycleStatus()}
	if t.exit != nil {
		code := *t.exit
		state.ExitCode = &code
	}
	return state
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

func (r *registry) discard(id string) {
	r.mu.Lock()
	window, ok := r.open[id]
	delete(r.open, id)
	var process *PTY
	if ok {
		if terminal, live := window.terminal(); live {
			process = terminal.process
		}
		if r.selected[window.session] == id {
			if fallback := r.leftmost(window.session); fallback != "" {
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
		process.Stop()
	}
	r.announce(window.session)
}

func (r *registry) leftmost(owner *SessionState) string {
	live := r.ordered(owner)
	if len(live) == 0 {
		return ""
	}
	return fmt.Sprintf(windowIDFormat, live[0].seq)
}

// ordered lists one session's windows left to right. Callers hold mu.
func (r *registry) ordered(owner *SessionState) []*agentWindow {
	live := make([]*agentWindow, 0, len(r.open))
	for _, window := range r.open {
		if window.session == owner {
			live = append(live, window)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].order < live[j].order })
	return live
}

func (r *registry) stop(owner *SessionState) {
	r.mu.Lock()
	if owner != nil {
		owner.windowsClosed.Store(true)
	}
	r.mu.Unlock()
	for _, id := range r.list() {
		if window, ok := r.window(id); ok && window.session == owner {
			r.discard(id)
		}
	}
}

func (r *registry) stopAll() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	for _, id := range r.list() {
		r.discard(id)
	}
}

// announce tells every page what one session has open. The payload names that
// session, and a page draws only the tab strip of the one it is showing.
func (r *registry) announce(owner *SessionState) {
	r.events.WindowsOpened(r.surfaces(owner))
}

type drawnWindow struct {
	ID     string               `json:"id"`
	Label  string               `json:"label"`
	Badge  string               `json:"badge,omitempty"`
	Icon   workbench.WindowIcon `json:"icon,omitempty"`
	Kind   string               `json:"kind"`
	Status string               `json:"status,omitempty"`
	Pinned bool                 `json:"pinned,omitempty"`

	// Artifact colours the badge. Kind is the window's own, terminal or
	// document, which says nothing about which artifact a document holds.
	Artifact string `json:"artifact,omitempty"`
}

// Surfaces names one session's open tabs in tab order.
type Surfaces struct {
	Session  string        `json:"session"`
	Selected string        `json:"selected"`
	Tabs     []drawnWindow `json:"tabs"`
}

func (r *registry) surfacesBySlug(slug string) Surfaces {
	return r.surfaces(r.sessions.BySlug(slug))
}

func (r *registry) surfaces(owner *SessionState) Surfaces {
	r.mu.Lock()
	defer r.mu.Unlock()
	live := r.ordered(owner)
	out := Surfaces{Session: owner.Slug(), Selected: r.selected[owner], Tabs: []drawnWindow{}}
	for _, window := range live {
		drawn := drawnWindow{
			ID: fmt.Sprintf(windowIDFormat, window.seq), Label: window.opts.Label,
			Badge: window.opts.Badge, Icon: window.opts.Icon, Kind: string(window.opts.Kind), Status: tabStatus(window),
			Pinned: pinned(window),
		}
		if window.opts.Badge != "" {
			drawn.Artifact = status.DocumentKind(window.opts.Source)
		}
		out.Tabs = append(out.Tabs, drawn)
	}
	return out
}

// pinned is the Overview tab, which neither the user nor the agent can close or
// move; only session teardown removes it.
func pinned(window *agentWindow) bool { return window.opts.Kind == workbench.KindOverview }

func tabStatus(window *agentWindow) string {
	if window.opts.Attention {
		return tabStatusWaiting
	}
	if window.content == nil {
		return ""
	}
	return window.content.tabStatus()
}
