package diagram

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
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
)

// Result is one fence's outcome, carrying the document line it was found on so
// the page can place it. Exactly one of SVG and Err is set.
type Result struct {
	Line int
	SVG  string
	Err  error
}

// Renderer turns d2 source into inline SVG. Renders are serialised through one
// goroutine: a textmeasure.Ruler carries mutable state and no lock, so it
// cannot be shared, and one worker also bounds CPU when a document opens with a
// dozen diagrams in it. The cache sits outside that goroutine so a caller can
// read it without joining the queue.
type Renderer struct {
	jobs    chan job
	stopped chan struct{}
	renders atomic.Int64
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
	r := &Renderer{jobs: make(chan job), stopped: make(chan struct{}), cache: newCache(cacheEntries)}
	go (&worker{
		jobs:    r.jobs,
		stopped: r.stopped,
		renders: &r.renders,
		cache:   r.cache,
		timeout: timeout,
	}).run()
	return r
}

// Render queues one fence and waits for it. Concurrent callers are served one
// at a time, in the order the worker takes them.
func (r *Renderer) Render(ctx context.Context, fence Fence) Result {
	reply := make(chan Result, 1)
	select {
	case r.jobs <- job{ctx: ctx, fence: fence, reply: reply}:
	case <-ctx.Done():
		return Result{Line: fence.Line, Err: ctx.Err()}
	case <-r.stopped:
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
// caller that must return promptly can serve the hits and wait only for the
// misses.
func (r *Renderer) Cached(source string) (string, bool) {
	return r.cache.get(cacheKey(source))
}

func (r *Renderer) Close() {
	close(r.jobs)
	<-r.stopped
}

type worker struct {
	jobs    <-chan job
	stopped chan struct{}
	renders *atomic.Int64
	cache   *cache
	timeout time.Duration
	ruler   *textmeasure.Ruler
}

func (w *worker) run() {
	defer close(w.stopped)
	for j := range w.jobs {
		j.reply <- w.one(j)
	}
}

func (w *worker) one(j job) Result {
	key := cacheKey(j.fence.Source)
	if svg, ok := w.cache.get(key); ok {
		return Result{Line: j.fence.Line, SVG: svg}
	}

	ruler, err := w.rule()
	if err != nil {
		return Result{Line: j.fence.Line, Err: err}
	}

	ctx, cancel := context.WithTimeout(d2log.With(j.ctx, silent), w.timeout)
	defer cancel()

	w.renders.Add(1)
	done := make(chan Result, 1)
	go func() { done <- render(ctx, ruler, j.fence) }()

	select {
	case out := <-done:
		if out.Err == nil {
			w.cache.put(key, out.SVG)
		}
		return out
	case <-ctx.Done():
		// Dagre never reads the context, so the deadline is enforced here. The
		// abandoned goroutine keeps measuring against this ruler, so it must
		// never be handed to another diagram.
		w.ruler = nil
		return Result{Line: j.fence.Line, Err: phrased{
			message: timedOutError,
			err:     fmt.Errorf("%w: %w", ErrTimedOut, ctx.Err()),
		}}
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

// cache holds rendered SVG for the life of the process, oldest out once full.
// Nothing else invalidates it; a palette edit bumps paletteRevision, which is
// part of every key. The worker writes it and Cached reads it, so it locks.
type cache struct {
	mu      sync.Mutex
	entries map[string]string
	order   []string
	limit   int
}

func newCache(limit int) *cache {
	return &cache{entries: make(map[string]string, limit), limit: limit}
}

func (c *cache) get(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	svg, ok := c.entries[key]
	return svg, ok
}

func (c *cache) put(key, svg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, seen := c.entries[key]; seen {
		return
	}
	if len(c.order) >= c.limit {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.entries[key] = svg
	c.order = append(c.order, key)
}

// The rendered SVG is swapped straight into the DOM, past rehype-sanitize, into
// a webview with no content policy and a context menu that hands href to the
// OS. Go is the only thing between d2's output and that, so the scan below
// rejects on doubt rather than parsing precisely.
var (
	scriptTag  = regexp.MustCompile(`(?i)<\s*script\b`)
	eventAttr  = regexp.MustCompile(`(?i)\son[a-z]+\s*=\s*["']`)
	foreignTag = regexp.MustCompile(`(?i)<\s*foreignobject\b`)
	imageTag   = regexp.MustCompile(`(?is)<\s*image\b[^>]*>`)
	anchorTag  = regexp.MustCompile(`(?is)<\s*a\b[^>]*>`)
	hrefAttr   = regexp.MustCompile(`(?is)\b(?:xlink:)?href\s*=\s*("[^"]*"|'[^']*')`)
)

func guard(svg string) error {
	if scriptTag.MatchString(svg) || eventAttr.MatchString(svg) {
		return ErrEmbeddedScript
	}
	if foreignTag.MatchString(svg) {
		return ErrEmbeddedMarkup
	}
	for _, tag := range imageTag.FindAllString(svg, -1) {
		for _, target := range targets(tag) {
			// d2's embedded fonts are legitimate data URIs; an icon: URL is not.
			if !strings.HasPrefix(target, dataScheme) {
				return ErrRemoteImage
			}
		}
	}
	for _, tag := range anchorTag.FindAllString(svg, -1) {
		for _, target := range targets(tag) {
			if !strings.HasPrefix(target, httpScheme) && !strings.HasPrefix(target, httpsScheme) {
				return ErrUnsafeLink
			}
		}
	}
	// The tag patterns above stop at the first ">", so a value carrying one
	// unescaped would hide the attributes after it. This sweep reads every href
	// in the document and cannot be split that way; the checks above run first
	// only so their message names the construct.
	for _, target := range targets(svg) {
		if !strings.HasPrefix(target, dataScheme) &&
			!strings.HasPrefix(target, httpScheme) &&
			!strings.HasPrefix(target, httpsScheme) {
			return ErrUnsafeLink
		}
	}
	return nil
}

// targets is one tag's href values as a browser would resolve them: entities
// decoded, whitespace and control characters dropped wherever they sit, and the
// scheme lowercased. Anything less lets `&#106;ava&#9;script:` through.
func targets(tag string) []string {
	var found []string
	for _, match := range hrefAttr.FindAllStringSubmatch(tag, -1) {
		quoted := match[1]
		found = append(found, normalise(quoted[1:len(quoted)-1]))
	}
	return found
}

func normalise(target string) string {
	target = html.UnescapeString(target)
	target = strings.Map(func(r rune) rune {
		if r <= ' ' {
			return -1
		}
		return r
	}, target)
	return strings.ToLower(target)
}
