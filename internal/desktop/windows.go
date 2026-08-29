package desktop

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kieranajp/qrouton/internal/diagram"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// Windows is the registry behind the agent's window tools and the service its
// page calls over the Wails bridge.
type Windows struct {
	emit     emitter
	sessions *Sessions
	// newShell reopens the shell after the user closes its tab.
	newShell    func() (string, error)
	newDocument func(name string) (string, error)
	// sourceMu serialises the check and open for windows with a Source.
	sourceMu sync.Mutex

	diagrams      *diagram.Renderer
	stopRendering context.CancelFunc
	renderCtx     context.Context
	rendering     sync.WaitGroup

	mu             sync.Mutex
	seq            int
	open           map[string]*agentWindow
	selected       map[*sessionState]string
	diagramsClosed bool
}

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

// ViewportReport is one browser measurement of a rendered Markdown tab.
type ViewportReport struct {
	Epoch     uint64                   `json:"epoch"`
	Seq       uint64                   `json:"seq"`
	Available bool                     `json:"available"`
	Selected  bool                     `json:"selected"`
	Intervals []workbench.LineInterval `json:"intervals"`
}

func newWindows(emit emitter, reg *Sessions) *Windows {
	ctx, stop := context.WithCancel(context.Background())
	w := &Windows{
		emit: emit, sessions: reg, open: map[string]*agentWindow{},
		selected:  map[*sessionState]string{},
		diagrams:  diagram.New(diagram.DefaultTimeout),
		renderCtx: ctx, stopRendering: stop,
	}
	go w.follow(ctx)
	return w
}

// follow keeps open documents current. A stat a second buys what a file
// watcher would, and reads a write-then-rename the same as a write in place.
func (w *Windows) follow(ctx context.Context) {
	ticker := time.NewTicker(documentPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.rescan()
		}
	}
}

func (w *Windows) shown() *sessionState { return w.sessions.current() }

func (w *Windows) openWindow(owner *sessionState, opts workbench.WindowOptions) (string, error) {
	if opts.Source != "" {
		w.sourceMu.Lock()
		defer w.sourceMu.Unlock()
		if id, ok := w.showing(owner, opts.Source); ok {
			w.discard(id)
		}
	}
	return w.spawn(owner, opts, true)
}

// openStructural opens a tab the workbench owns rather than the agent. It
// leaves the selection alone: every caller either selects the tab itself once
// it has the id, or is the unasked-for first shell, which has nothing to steal
// the selection from.
func (w *Windows) openStructural(owner *sessionState, opts workbench.WindowOptions) (string, error) {
	return w.spawn(owner, opts, false)
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
		return id, w.selectWindow(owner, id)
	}
	if w.newDocument == nil {
		return "", ErrNoEditorCommand
	}
	id, err := w.newDocument(name)
	if err != nil {
		return "", err
	}
	return id, w.selectWindow(owner, id)
}

// Select records a user-driven tab selection for one session.
func (w *Windows) Select(slug, id string) error {
	owner := w.sessions.bySlug(slug)
	if owner == nil {
		return unknownSession(slug)
	}
	return w.selectWindow(owner, id)
}

func (w *Windows) selectWindow(owner *sessionState, id string) error {
	w.mu.Lock()
	window, ok := w.open[id]
	if !ok || window.session != owner {
		w.mu.Unlock()
		return noSuchWindow(id)
	}
	w.selected[owner] = id
	w.mu.Unlock()
	w.announce(owner)
	return nil
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

func (w *Windows) spawn(owner *sessionState, opts workbench.WindowOptions, selects bool) (string, error) {
	if opts.Kind == workbench.KindTerminal && len(opts.Command) == 0 {
		return "", ErrNoWindowCommand
	}
	w.mu.Lock()
	w.seq++
	id := fmt.Sprintf(windowIDFormat, w.seq)
	window := &agentWindow{opts: opts, session: owner, seq: w.seq}
	if opts.Kind == workbench.KindTerminal {
		window.buffer = &ring{limit: windowScrollback}
	}
	if opts.Kind == workbench.KindDocument && opts.Format == workbench.FormatMarkdown {
		window.viewport = &workbench.DocumentViewport{Source: opts.Source, Intervals: []workbench.LineInterval{}}
	}
	// The content arrived from a read taken before this stat. A size that no
	// longer matches it means the file moved in between, so it is left unseen
	// for the first rescan to pick up rather than recorded as already read.
	if info, err := os.Stat(window.sourcePath()); err == nil && info.Size() == int64(len(opts.Content)) {
		window.read.at, window.read.size = info.ModTime(), info.Size()
	}
	w.open[id] = window
	if selects {
		w.selected[owner] = id
	}
	w.mu.Unlock()

	w.announce(owner)
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

// document is a document window's text, how its page should render it, the
// session file it came from, if it came from one, and the source lines the page
// should scroll to and mark. Zero lines leave the page at the top.
type document struct {
	Text          string `json:"text"`
	Format        string `json:"format"`
	Source        string `json:"source"`
	Path          string `json:"path,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Line          int    `json:"line"`
	To            int    `json:"to"`
	ViewportEpoch uint64 `json:"viewportEpoch,omitempty"`
}

func (window *agentWindow) sourcePath() string {
	if window.opts.Source == "" || window.session == nil {
		return ""
	}
	return filepath.Join(window.session.root(), filepath.FromSlash(window.opts.Source))
}

// documentFor is a window as its page receives it. It builds and never mutates,
// so the epoch it carries is whatever the caller has already decided on.
func documentFor(window *agentWindow) document {
	first, last, _ := window.opts.Span.Bounds()
	var kind string
	path := window.sourcePath()
	if path != "" {
		kind = status.DocumentKind(window.opts.Source)
	}
	var viewportEpoch uint64
	if window.viewport != nil {
		viewportEpoch = window.viewportEpoch
	}
	return document{
		Text:          window.opts.Content,
		Format:        string(window.opts.Format),
		Source:        window.opts.Source,
		Path:          path,
		Kind:          kind,
		Line:          first,
		To:            last,
		ViewportEpoch: viewportEpoch,
	}
}

// Content is a document window's text, fetched by its page on load. A load
// restarts the page's sequence counter, so the epoch moves and the viewport
// starts again from nothing measured.
func (w *Windows) Content(id string) (document, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	window, ok := w.open[id]
	if !ok {
		return document{}, noSuchWindow(id)
	}
	if window.viewport != nil {
		window.viewportEpoch++
		window.viewportSeq = 0
		window.viewport = &workbench.DocumentViewport{
			Source: window.opts.Source, Intervals: []workbench.LineInterval{},
		}
	}
	return documentFor(window), nil
}

// rescan pushes every open document whose file has changed to its page. A push
// is not a reload: the same controller keeps reporting against a monotonic
// sequence, so the epoch is reused and the sequence left where it is. Bumping
// either would fence off reports the page is still entitled to send.
func (w *Windows) rescan() {
	type push struct {
		id  string
		doc document
	}
	var pushes []push
	w.mu.Lock()
	for id, window := range w.open {
		if window.opts.Kind != workbench.KindDocument {
			continue
		}
		path := window.sourcePath()
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		// An unreadable file leaves the tab showing what it last held. A file
		// caught between truncation and rewrite still reads empty for a tick.
		if err != nil || info.IsDir() || info.Size() > workbench.DocumentLimit {
			continue
		}
		if info.Size() == window.read.size && info.ModTime().Equal(window.read.at) {
			continue
		}
		text, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		window.read.at, window.read.size = info.ModTime(), info.Size()
		window.opts.Content = string(text)
		pushes = append(pushes, push{id: id, doc: documentFor(window)})
	}
	w.mu.Unlock()
	for _, sent := range pushes {
		w.emit(windowContentEvent+sent.id, sent.doc)
	}
}

// renderedDiagram is one d2 fence as the page receives it: the document line
// its opening marker sits on, and either the SVG, the reason there is none, or
// neither while it is still being laid out.
type renderedDiagram struct {
	Line  int    `json:"line"`
	SVG   string `json:"svg,omitempty"`
	Error string `json:"error,omitempty"`
}

// RenderDiagrams names every d2 fence in a document window, carrying the SVG of
// the ones already rendered; the rest are laid out off this goroutine and
// arrive on windowDiagramEvent as they land. Opening a document costs a scan,
// never a layout. A window with nothing to draw answers with an empty list
// rather than an error: every Markdown pane calls this, diagrams or not.
func (w *Windows) RenderDiagrams(id string) ([]renderedDiagram, error) {
	window, ok := w.window(id)
	if !ok {
		return nil, noSuchWindow(id)
	}
	if window.opts.Kind != workbench.KindDocument || window.opts.Format != workbench.FormatMarkdown {
		return []renderedDiagram{}, nil
	}
	found := []renderedDiagram{}
	var misses []diagram.Fence
	for _, fence := range diagram.Scan(window.opts.Content) {
		if svg, hit := w.diagrams.Cached(fence.Source); hit {
			found = append(found, renderedDiagram{Line: fence.Line, SVG: svg})
			continue
		}
		found = append(found, renderedDiagram{Line: fence.Line})
		misses = append(misses, fence)
	}
	w.layOut(id, misses)
	return found, nil
}

// layOut renders one document's misses in document order, emitting each as it
// finishes. It declines once the renderer is stopping, so a quit cannot race a
// send at a worker that is shutting down.
func (w *Windows) layOut(id string, fences []diagram.Fence) {
	if len(fences) == 0 {
		return
	}
	w.mu.Lock()
	if w.diagramsClosed {
		w.mu.Unlock()
		return
	}
	w.rendering.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.rendering.Done()
		for _, fence := range fences {
			out := w.diagrams.Render(w.renderCtx, fence)
			if w.renderCtx.Err() != nil {
				return
			}
			w.emit(windowDiagramEvent+id, drawnDiagram(out))
		}
	}()
}

func drawnDiagram(out diagram.Result) renderedDiagram {
	if out.Err != nil {
		return renderedDiagram{Line: out.Line, Error: out.Err.Error()}
	}
	return renderedDiagram{Line: out.Line, SVG: out.SVG}
}

// ReportViewport stores the newest browser measurement for a Markdown tab.
func (w *Windows) ReportViewport(id string, report ViewportReport) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	window, ok := w.open[id]
	if !ok {
		return noSuchWindow(id)
	}
	if window.viewport == nil {
		return ErrNoViewport
	}
	if report.Epoch != window.viewportEpoch {
		return nil
	}
	if report.Seq <= window.viewportSeq {
		return nil
	}
	intervals, err := normalizedIntervals(report.Intervals)
	if err != nil {
		return err
	}
	available := report.Available && report.Selected
	if !available {
		intervals = []workbench.LineInterval{}
	}
	window.viewportSeq = report.Seq
	window.viewport = &workbench.DocumentViewport{
		Source:    window.opts.Source,
		Available: available,
		Selected:  report.Selected,
		Intervals: intervals,
	}
	return nil
}

func normalizedIntervals(intervals []workbench.LineInterval) ([]workbench.LineInterval, error) {
	if len(intervals) == 0 {
		return []workbench.LineInterval{}, nil
	}
	out := append([]workbench.LineInterval(nil), intervals...)
	for _, interval := range out {
		if interval.Line < 1 || interval.To < interval.Line {
			return nil, fmt.Errorf("%w: %d-%d", ErrInvalidViewport, interval.Line, interval.To)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Line == out[j].Line {
			return out[i].To < out[j].To
		}
		return out[i].Line < out[j].Line
	})
	merged := out[:1]
	for _, interval := range out[1:] {
		last := &merged[len(merged)-1]
		if interval.Line <= last.To+1 {
			if interval.To > last.To {
				last.To = interval.To
			}
			continue
		}
		merged = append(merged, interval)
	}
	return merged, nil
}

func (w *Windows) viewport(owner *sessionState, id string) (*workbench.DocumentViewport, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	window, ok := w.open[id]
	if !ok || window.session != owner {
		return nil, noSuchWindow(id)
	}
	if window.viewport == nil {
		return nil, nil
	}
	view := *window.viewport
	view.Intervals = append([]workbench.LineInterval{}, window.viewport.Intervals...)
	return &view, nil
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
		w.discard(id)
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
	w.discard(id)
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

// discard forgets a window and stops whatever it was running.
func (w *Windows) discard(id string) {
	w.mu.Lock()
	window, ok := w.open[id]
	delete(w.open, id)
	var process *ptyProcess
	if ok {
		process = window.process
		if w.selected[window.session] == id {
			if fallback := w.oldest(window.session); fallback != "" {
				w.selected[window.session] = fallback
			} else {
				delete(w.selected, window.session)
			}
		}
	}
	w.mu.Unlock()
	if !ok {
		return
	}
	if process != nil {
		process.stop()
	}
	w.announce(window.session)
}

func (w *Windows) oldest(owner *sessionState) string {
	oldestID := ""
	oldestSeq := 0
	for id, window := range w.open {
		if window.session == owner && (oldestID == "" || window.seq < oldestSeq) {
			oldestID = id
			oldestSeq = window.seq
		}
	}
	return oldestID
}

// announce tells one session's pages what it has open. A background session
// docking a tab must not redraw the foreground's tab strip.
func (w *Windows) announce(owner *sessionState) {
	w.emit(windowsEvent, w.surfaces(owner))
}

// drawnWindow is one window as its surface draws it.
type drawnWindow struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Status string `json:"status,omitempty"`
}

// surfaces names one session's open tabs, oldest first so the shell stays
// leftmost.
type surfaces struct {
	Session  string        `json:"session"`
	Selected string        `json:"selected"`
	Tabs     []drawnWindow `json:"tabs"`
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
	out := surfaces{Session: owner.slug(), Selected: w.selected[owner], Tabs: []drawnWindow{}}
	for _, window := range live {
		drawn := drawnWindow{
			ID: fmt.Sprintf(windowIDFormat, window.seq), Label: window.opts.Label,
			Kind: string(window.opts.Kind), Status: tabStatus(window),
		}
		out.Tabs = append(out.Tabs, drawn)
	}
	return out
}

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

// stopAll tears every window down, so closing the conversation leaves nothing
// running behind it.
func (w *Windows) stopAll() {
	for _, id := range w.list() {
		w.discard(id)
	}
	w.stopDiagrams()
}

// stopDiagrams cancels what is in flight before closing the worker: the
// cancellation reaches the layout already underway, so quitting mid-diagram
// does not wait out the render budget.
func (w *Windows) stopDiagrams() {
	w.mu.Lock()
	closed := w.diagramsClosed
	w.diagramsClosed = true
	w.mu.Unlock()
	if closed {
		return
	}
	w.stopRendering()
	w.rendering.Wait()
	w.diagrams.Close()
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
