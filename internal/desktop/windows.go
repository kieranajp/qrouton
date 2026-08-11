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

	mu      sync.Mutex
	seq     int
	open    map[string]*agentWindow
	changed func()
}

type agentWindow struct {
	opts    workbench.WindowOptions
	seq     int
	buffer  *ring
	process *ptyProcess
	timer   *time.Timer
	// recorded excludes the windows the workbench opens for itself from the
	// manifest's record, since a resume rebuilds those without being told to.
	recorded bool
}

func newWindows(r renderer, emit emitter) *Windows {
	return &Windows{renderer: r, emit: emit, open: map[string]*agentWindow{}}
}

func (w *Windows) openWindow(opts workbench.WindowOptions) (string, error) {
	return w.spawn(opts, true)
}

// openStructural opens a window the workbench owns rather than the agent.
func (w *Windows) openStructural(opts workbench.WindowOptions) (string, error) {
	return w.spawn(opts, false)
}

func (w *Windows) spawn(opts workbench.WindowOptions, recorded bool) (string, error) {
	if opts.Kind == workbench.KindTerminal && len(opts.Command) == 0 {
		return "", ErrNoWindowCommand
	}
	w.mu.Lock()
	w.seq++
	id := fmt.Sprintf(windowIDFormat, w.seq)
	window := &agentWindow{opts: opts, seq: w.seq, recorded: recorded}
	if opts.Kind == workbench.KindTerminal {
		window.buffer = &ring{limit: windowScrollback}
	}
	w.open[id] = window
	w.mu.Unlock()

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
	if opts.Kind == workbench.KindDocument && opts.TTL > 0 {
		w.mu.Lock()
		if _, live := w.open[id]; live {
			window.timer = time.AfterFunc(opts.TTL, func() { w.dismiss(id) })
		}
		w.mu.Unlock()
	}
	w.announce()
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

// document is a document window's text and how its page should render it.
type document struct {
	Text   string `json:"text"`
	Format string `json:"format"`
}

// Content is a document window's text, fetched by its page on load.
func (w *Windows) Content(id string) (document, error) {
	window, ok := w.window(id)
	if !ok {
		return document{}, noSuchWindow(id)
	}
	return document{Text: window.opts.Content, Format: string(window.opts.Format)}, nil
}

// processExited applies the lifecycle rule: a clean exit closes the window, a
// failure leaves it open so the error stays readable.
func (w *Windows) processExited(id string, code int) {
	w.emit(windowExitEvent+id, code)
	window, ok := w.window(id)
	if !ok {
		return
	}
	if code == 0 && window.opts.CloseOnExit {
		w.dismiss(id)
	}
}

func (w *Windows) closeWindow(id string) error {
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
	if w.discard(id) {
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
	w.announce()
	return true
}

// observe registers what to tell when the open set changes. Clearing it before
// teardown keeps the session's own ending out of the record: the manifest is
// meant to say which windows were open, not that closing the conversation
// closed them.
func (w *Windows) observe(changed func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.changed = changed
}

func (w *Windows) announce() {
	w.mu.Lock()
	changed := w.changed
	w.mu.Unlock()
	if changed != nil {
		changed()
	}
}

// snapshot describes the agent's open windows, oldest first.
func (w *Windows) snapshot() []workbench.WindowOptions {
	w.mu.Lock()
	defer w.mu.Unlock()
	live := make([]*agentWindow, 0, len(w.open))
	for _, window := range w.open {
		if window.recorded {
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

func (w *Windows) window(id string) (*agentWindow, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	window, ok := w.open[id]
	return window, ok
}

func terminalEnv() []string {
	env := workbench.WithEnv(os.Environ(), termEnvVar, termValue)
	return workbench.WithEnv(env, colorTermEnvVar, colorTermValue)
}

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
