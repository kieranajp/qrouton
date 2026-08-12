package desktop

import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// Windows is the registry behind the agent's window tools and the service its
// pages call over the Wails bridge.
type Windows struct {
	renderer renderer
	emit     emitter
	sessions *Sessions
	// dock sends the agent's windows to the tab strip instead of the screen.
	dock bool
	// newShell reopens the shell after the user closes its tab.
	newShell    func() (string, error)
	newDocument func(name string) (string, error)
	// sourceMu serialises the check and open for windows with a Source.
	sourceMu sync.Mutex

	mu      sync.Mutex
	seq     int
	open    map[string]*agentWindow
	changed func(owner *sessionState)
}

type agentWindow struct {
	opts    workbench.WindowOptions
	session *sessionState
	seq     int
	buffer  *ring
	process *ptyProcess
	timer   *time.Timer
	// recorded excludes the windows the workbench opens for itself from the
	// manifest's record, since a resume rebuilds those without being told to.
	recorded bool
	// docked windows live in the tab strip, so no renderer window backs them.
	docked bool
	// exit is nil while the process is still running.
	exit *int
}

func newWindows(r renderer, emit emitter, dock bool, reg *Sessions) *Windows {
	return &Windows{renderer: r, emit: emit, sessions: reg, dock: dock, open: map[string]*agentWindow{}}
}

func (w *Windows) shown() *sessionState { return w.sessions.current() }

// A picker nobody can see is a session that hangs, so anything asking for focus
// gets a real window.
func (w *Windows) openWindow(owner *sessionState, opts workbench.WindowOptions) (string, error) {
	if opts.Source != "" {
		w.sourceMu.Lock()
		defer w.sourceMu.Unlock()
		if id, ok := w.showing(owner, opts.Source); ok {
			w.dismiss(id)
		}
	}
	return w.spawn(owner, opts, true, w.dock && !opts.Focus)
}

// openStructural opens a window the workbench owns rather than the agent: the
// session shell, always a tab.
func (w *Windows) openStructural(owner *sessionState, opts workbench.WindowOptions) (string, error) {
	return w.spawn(owner, opts, false, true)
}

// OpenShell opens another terminal in the session's right pane and returns its
// id, so the page can select the tab the user just asked for.
func (w *Windows) OpenShell() (string, error) {
	if w.newShell == nil {
		return "", ErrNoShellCommand
	}
	return w.newShell()
}

// OpenDocument returns the window already showing the named document, or opens
// one — so a single click both opens and selects. The name is session-relative.
func (w *Windows) OpenDocument(name string) (string, error) {
	if name == "" {
		return "", ErrNoDocumentName
	}
	w.sourceMu.Lock()
	defer w.sourceMu.Unlock()
	owner := w.shown()
	if id, ok := w.showing(owner, name); ok {
		w.emit(selectEvent, selection{Session: owner.slug(), ID: id})
		return id, nil
	}
	if w.newDocument == nil {
		return "", ErrNoEditorCommand
	}
	return w.newDocument(name)
}

func (w *Windows) showing(owner *sessionState, source string) (string, bool) {
	if source == "" {
		return "", false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, window := range w.open {
		if window.session == owner && window.opts.Source == source {
			return id, true
		}
	}
	return "", false
}

func (w *Windows) spawn(owner *sessionState, opts workbench.WindowOptions, recorded, docked bool) (string, error) {
	if opts.Kind == workbench.KindTerminal && len(opts.Command) == 0 {
		return "", ErrNoWindowCommand
	}
	w.mu.Lock()
	w.seq++
	id := fmt.Sprintf(windowIDFormat, w.seq)
	window := &agentWindow{opts: opts, session: owner, seq: w.seq, recorded: recorded, docked: docked}
	if opts.Kind == workbench.KindTerminal {
		window.buffer = &ring{limit: windowScrollback}
	}
	w.open[id] = window
	w.mu.Unlock()

	// A docked window's surface is the tab strip, so nothing is opened here.
	if !docked {
		if err := w.renderer.Open(windowSpec{
			Name:    id,
			Title:   opts.Label,
			URL:     pageURL(opts.Kind, id),
			Width:   agentWindowWidth,
			Height:  agentWindowHeight,
			Focus:   opts.Focus,
			OnClose: func() { w.discard(id) },
		}); err != nil {
			w.discard(id)
			return "", err
		}
	}
	if !docked && opts.Kind == workbench.KindDocument && opts.TTL > 0 {
		w.mu.Lock()
		if _, live := w.open[id]; live {
			window.timer = time.AfterFunc(opts.TTL, func() { w.dismiss(id) })
		}
		w.mu.Unlock()
	}
	w.announce(owner)
	// A document the agent opened behind the shell tab is a document nobody
	// reads. Terminals are left where they are: the tab strip focuses the
	// terminal it selects, which would take the keyboard off the conversation.
	if docked && opts.Kind == workbench.KindDocument {
		w.emit(selectEvent, selection{Session: owner.slug(), ID: id})
	}
	return id, nil
}

// Start launches a terminal window's command once its page has measured itself.
// The page calls it on load, so a reload must not fork a second process.
func (w *Windows) Start(id string, cols, rows int) error {
	w.mu.Lock()
	window, ok := w.open[id]
	if !ok {
		w.mu.Unlock()
		return noSuchWindow(id)
	}
	// A document has no command, and a page that asks anyway must not take the
	// workbench down with it.
	if window.opts.Kind != workbench.KindTerminal {
		w.mu.Unlock()
		return ErrNotATerminal
	}
	if window.process != nil {
		w.mu.Unlock()
		return nil
	}
	process, err := startPTY(window.opts.Command, terminalEnv(), window.opts.Cwd, cols, rows)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	window.process = process
	w.mu.Unlock()
	go process.pump(
		func(b []byte) {
			window.buffer.write(b)
			w.emit(windowDataEvent+id, base64.StdEncoding.EncodeToString(b))
		},
		func(code int) { w.processExited(id, code) },
	)
	return nil
}

func (w *Windows) Write(id, encoded string) error {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	window, ok := w.window(id)
	if !ok {
		return noSuchWindow(id)
	}
	w.mu.Lock()
	process := window.process
	w.mu.Unlock()
	if process == nil {
		return ErrTerminalNotStarted
	}
	return process.write(data)
}

func (w *Windows) Resize(id string, cols, rows int) error {
	window, ok := w.window(id)
	if !ok {
		return nil
	}
	w.mu.Lock()
	process := window.process
	w.mu.Unlock()
	if process == nil {
		return nil
	}
	return process.resize(cols, rows)
}

// document is a document window's text, how its page should render it, and the
// session file it came from, if it came from one.
type document struct {
	Text   string `json:"text"`
	Format string `json:"format"`
	Source string `json:"source"`
}

// Content is a document window's text, fetched by its page on load.
func (w *Windows) Content(id string) (document, error) {
	window, ok := w.window(id)
	if !ok {
		return document{}, noSuchWindow(id)
	}
	return document{
		Text:   window.opts.Content,
		Format: string(window.opts.Format),
		Source: window.opts.Source,
	}, nil
}

// processExited applies the lifecycle rule: a clean exit closes the window, a
// failure leaves it open so the error stays readable.
func (w *Windows) processExited(id string, code int) {
	w.emit(windowExitEvent+id, code)
	window, ok := w.window(id)
	if !ok {
		return
	}
	w.mu.Lock()
	window.exit = &code
	w.mu.Unlock()
	if code == 0 && window.opts.CloseOnExit {
		w.dismiss(id)
		return
	}
	w.announce(window.session)
}

// Close serves the agent's window tool and the tab strip's close control. Wails
// binds exported methods only, so unexporting this silently breaks the tab.
func (w *Windows) Close(id string) error {
	if _, ok := w.window(id); !ok {
		return noSuchWindow(id)
	}
	w.dismiss(id)
	return nil
}

func (w *Windows) readWindow(id string, full bool) (string, error) {
	window, ok := w.window(id)
	if !ok {
		return "", noSuchWindow(id)
	}
	if window.opts.Kind == workbench.KindDocument {
		return window.opts.Content, nil
	}
	return window.buffer.text(full), nil
}

func (w *Windows) exists(id string) bool {
	_, ok := w.window(id)
	return ok
}

func (w *Windows) list() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	ids := make([]string, 0, len(w.open))
	for id := range w.open {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// dismiss tears the window down and takes it off the screen.
func (w *Windows) dismiss(id string) {
	window, ok := w.window(id)
	if !ok {
		return
	}
	if w.discard(id) && !window.docked {
		w.renderer.Close(id)
	}
}

// discard forgets a window and stops whatever it was running, reporting whether
// this call was the one that did it.
func (w *Windows) discard(id string) bool {
	w.mu.Lock()
	window, ok := w.open[id]
	delete(w.open, id)
	var timer *time.Timer
	var process *ptyProcess
	if ok {
		timer, process = window.timer, window.process
	}
	w.mu.Unlock()
	if !ok {
		return false
	}
	if timer != nil {
		timer.Stop()
	}
	if process != nil {
		process.stop()
	}
	w.announce(window.session)
	return true
}

// observe registers what to tell when a session's open set changes. Clearing it
// before teardown keeps a session's own ending out of the manifest's record.
func (w *Windows) observe(changed func(owner *sessionState)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.changed = changed
}

// announce tells one session's pages what it has open. A background session
// docking a tab must not redraw the foreground's tab strip.
func (w *Windows) announce(owner *sessionState) {
	w.mu.Lock()
	changed := w.changed
	w.mu.Unlock()
	w.emit(windowsEvent, w.surfaces(owner))
	if changed != nil {
		changed(owner)
	}
}

// drawnWindow is one window as its surface draws it.
type drawnWindow struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Status string `json:"status,omitempty"`
}

// surfaces splits one session's open windows by where they are drawn, oldest
// first so the shell stays leftmost, and names the session it describes.
type surfaces struct {
	Session  string        `json:"session"`
	Tabs     []drawnWindow `json:"tabs"`
	Floating []drawnWindow `json:"floating"`
}

// selection is the window a page is being asked to bring forward.
type selection struct {
	Session string `json:"session"`
	ID      string `json:"id"`
}

func (w *Windows) surfaces(owner *sessionState) surfaces {
	w.mu.Lock()
	defer w.mu.Unlock()
	live := make([]*agentWindow, 0, len(w.open))
	for _, window := range w.open {
		if window.session == owner {
			live = append(live, window)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].seq < live[j].seq })
	out := surfaces{Session: owner.slug(), Tabs: []drawnWindow{}, Floating: []drawnWindow{}}
	for _, window := range live {
		drawn := drawnWindow{
			ID: fmt.Sprintf(windowIDFormat, window.seq), Label: window.opts.Label,
			Kind: string(window.opts.Kind), Status: tabStatus(window),
		}
		if window.docked {
			out.Tabs = append(out.Tabs, drawn)
		} else {
			out.Floating = append(out.Floating, drawn)
		}
	}
	return out
}

func (w *Windows) tabs(owner *sessionState) []drawnWindow { return w.surfaces(owner).Tabs }

// Surfaces answers a session's page on load: the event only fires on a change,
// and the shell opens before anything is subscribed.
func (w *Windows) Surfaces(slug string) surfaces { return w.surfaces(w.sessions.bySlug(slug)) }

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

// snapshot describes one session's open agent windows, oldest first.
func (w *Windows) snapshot(owner *sessionState) []workbench.WindowOptions {
	w.mu.Lock()
	defer w.mu.Unlock()
	live := make([]*agentWindow, 0, len(w.open))
	for _, window := range w.open {
		if window.session == owner && window.recorded {
			live = append(live, window)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].seq < live[j].seq })
	out := make([]workbench.WindowOptions, 0, len(live))
	for _, window := range live {
		out = append(out, window.opts)
	}
	return out
}

// stopAll tears every window down, so closing the conversation leaves nothing
// running behind it.
func (w *Windows) stopAll() {
	for _, id := range w.list() {
		w.discard(id)
	}
}

// stop tears down one session's windows, so retiring it leaves the rest alone.
func (w *Windows) stop(owner *sessionState) {
	for _, id := range w.list() {
		if window, ok := w.window(id); ok && window.session == owner {
			w.discard(id)
		}
	}
}

func (w *Windows) window(id string) (*agentWindow, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	window, ok := w.open[id]
	return window, ok
}

func terminalEnv() []string { return withTerminalEnv(os.Environ()) }

func pageURL(kind workbench.WindowKind, id string) string {
	page := terminalPage
	if kind == workbench.KindDocument {
		page = documentPage
	}
	return page + windowIDQuery + id
}

// ring keeps the tail of a window's output.
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
