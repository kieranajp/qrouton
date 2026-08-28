package diagram

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/d2lang/d2/d2graph"
	"github.com/d2lang/d2/d2layouts/d2dagrelayout"
	"github.com/d2lang/d2/d2lib"
	"github.com/d2lang/d2/d2parser"
	"github.com/d2lang/d2/d2renderers/d2svg"
	d2log "github.com/d2lang/d2/lib/log"
	"github.com/d2lang/d2/lib/textmeasure"
)

const (
	// DefaultTimeout bounds one diagram's compile and layout.
	DefaultTimeout = 5 * time.Second

	cacheEntries = 128
	pad          = int64(20)

	// d2 sizes by content, so an unscaled diagram dwarfs the prose around it.
	scale = 0.65
)

// Result is one fence's outcome, carrying the document line it was found on so
// the page can place it. Exactly one of SVG and Err is set.
type Result struct {
	Line int
	SVG  string
	Err  error
}

// Renderer turns d2 source into inline SVG. Renders are serialised through one
// goroutine: the font ruler carries mutable state and no lock.
type Renderer struct {
	jobs chan job
	// Render selects on closing rather than on a closed jobs channel: a send
	// case on a closed channel is ready in a select, and panics if it is chosen.
	closing chan struct{}
	stopped chan struct{}
	once    sync.Once
	cache   *cache
}

type job struct {
	ctx   context.Context
	fence Fence
	reply chan Result
}

// New starts a renderer. Close it to stop the worker.
func New(timeout time.Duration) *Renderer {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	r := &Renderer{
		jobs:    make(chan job),
		closing: make(chan struct{}),
		stopped: make(chan struct{}),
		cache:   newCache(cacheEntries),
	}
	go (&worker{
		jobs:    r.jobs,
		closing: r.closing,
		stopped: r.stopped,
		cache:   r.cache,
		timeout: timeout,
	}).run()
	return r
}

// Render queues one fence and waits for it, one caller at a time. One that
// arrives after Close has begun is answered with context.Canceled.
func (r *Renderer) Render(ctx context.Context, fence Fence) Result {
	reply := make(chan Result, 1)
	select {
	case r.jobs <- job{ctx: ctx, fence: fence, reply: reply}:
	case <-ctx.Done():
		return Result{Line: fence.Line, Err: ctx.Err()}
	case <-r.closing:
		return Result{Line: fence.Line, Err: context.Canceled}
	}
	select {
	case out := <-reply:
		return out
	case <-ctx.Done():
		return Result{Line: fence.Line, Err: ctx.Err()}
	}
}

// Cached answers from what has already been rendered without queueing, so a
// caller that must return promptly can serve the hits and wait for the misses.
func (r *Renderer) Cached(source string) (string, bool) {
	done, ok := r.cache.get(cacheKey(source))
	if !ok || done.err != nil {
		return "", false
	}
	return done.svg, true
}

// Close stops the worker and waits for it. It may be called more than once, and
// concurrently with Render.
func (r *Renderer) Close() {
	r.once.Do(func() { close(r.closing) })
	<-r.stopped
}

type worker struct {
	jobs    <-chan job
	closing <-chan struct{}
	stopped chan struct{}
	cache   *cache
	timeout time.Duration
	ruler   *textmeasure.Ruler
}

func (w *worker) run() {
	defer close(w.stopped)
	for {
		select {
		case j := <-w.jobs:
			j.reply <- w.one(j)
		case <-w.closing:
			return
		}
	}
}

func (w *worker) one(j job) Result {
	key := cacheKey(j.fence.Source)
	if done, ok := w.cache.get(key); ok {
		return Result{Line: j.fence.Line, SVG: done.svg, Err: done.err}
	}

	ruler, err := w.rule()
	if err != nil {
		return Result{Line: j.fence.Line, Err: err}
	}

	ctx, cancel := context.WithTimeout(d2log.With(j.ctx, silent), w.timeout)
	defer cancel()

	done := make(chan Result, 1)
	go func() { done <- render(ctx, ruler, j.fence) }()

	select {
	case out := <-done:
		w.cache.put(key, outcome{svg: out.SVG, err: out.Err})
		return out
	case <-ctx.Done():
		// Dagre never reads the context, so the deadline is enforced here. The
		// abandoned goroutine keeps measuring against this ruler, so it must
		// never be handed to another diagram.
		w.ruler = nil
		out := Result{Line: j.fence.Line, Err: phrased{
			message: timedOutError,
			err:     fmt.Errorf("%w: %w", ErrTimedOut, ctx.Err()),
		}}
		// A caller that walked away is not a verdict on the source.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			w.cache.put(key, outcome{err: out.Err})
		}
		return out
	}
}

func (w *worker) rule() (*textmeasure.Ruler, error) {
	if w.ruler != nil {
		return w.ruler, nil
	}
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoRuler, err)
	}
	w.ruler = ruler
	return ruler, nil
}

// silent keeps d2 off stdout: with no logger in the context it writes a warning
// and a full goroutine stack on every render.
var silent = slog.New(slog.DiscardHandler)

func render(ctx context.Context, ruler *textmeasure.Ruler, fence Fence) Result {
	opts := &d2svg.RenderOpts{
		Pad:            ptr(pad),
		ThemeID:        ptr(neutralDefaultTheme),
		ThemeOverrides: overrides(),
		// Without NoXMLTag the string leads with an <?xml?> prolog and cannot be
		// inserted inline. OmitVersion keeps output stable across d2 upgrades.
		NoXMLTag:    ptr(true),
		OmitVersion: ptr(true),
		Scale:       ptr(scale),
	}
	compiled, _, err := d2lib.Compile(ctx, fence.Source, &d2lib.CompileOptions{
		Ruler:          ruler,
		LayoutResolver: dagre,
	}, opts)
	if err != nil {
		return Result{Line: fence.Line, Err: relocate(err, fence.Line)}
	}
	out, err := d2svg.Render(compiled, opts)
	if err != nil {
		return Result{Line: fence.Line, Err: err}
	}
	svg := string(out)
	if err := guard(svg); err != nil {
		return Result{Line: fence.Line, Err: err}
	}
	return Result{Line: fence.Line, SVG: svg}
}

// dagre is the only layout engine qrouton will resolve. ELK is EPL-2.0.
func dagre(string) (d2graph.LayoutGraph, error) { return d2dagrelayout.DefaultLayout, nil }

// relocate rewrites a parse error's line:col prefixes, which d2 numbers from
// the top of the fence, so they name a line of the document instead.
func relocate(err error, line int) error {
	var parse *d2parser.ParseError
	if !errors.As(err, &parse) || parse.Empty() {
		return err
	}
	messages := make([]string, 0, len(parse.Errors))
	for _, one := range parse.Errors {
		at := fmt.Sprintf("%d:%d", one.Range.Start.Line+line+1, one.Range.Start.Column+1)
		messages = append(messages, at+": "+strings.TrimPrefix(one.Message, one.Range.String()+": "))
	}
	return phrased{message: strings.Join(messages, "\n"), err: err}
}

// phrased carries the line a pane prints over a chain errors.Is still reads:
// what the document says is not what a Go caller matches on.
type phrased struct {
	message string
	err     error
}

func (p phrased) Error() string { return p.message }
func (p phrased) Unwrap() error { return p.err }

func cacheKey(source string) string {
	sum := sha256.Sum256([]byte(paletteRevision + "\x00" + source))
	return hex.EncodeToString(sum[:])
}

// cache holds every verdict for the life of the process, oldest out once full.
// A palette edit bumps paletteRevision, which is part of every key.
type cache struct {
	mu      sync.Mutex
	entries map[string]outcome
	order   []string
	limit   int
}

// outcome is what the worker decided about one source. A refusal is as final as
// a drawing, so it is kept too: nothing about the source will change.
type outcome struct {
	svg string
	err error
}

func newCache(limit int) *cache {
	return &cache{entries: make(map[string]outcome, limit), limit: limit}
}

func (c *cache) get(key string) (outcome, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	done, ok := c.entries[key]
	return done, ok
}

func (c *cache) put(key string, done outcome) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, seen := c.entries[key]; seen {
		return
	}
	if len(c.order) >= c.limit {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.entries[key] = done
	c.order = append(c.order, key)
}
