package diagram

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	d2log "github.com/d2lang/d2/lib/log"
	"github.com/d2lang/d2/lib/textmeasure"

	"github.com/kieranajp/qrouton/internal/theme"
)

func renderer(t *testing.T, timeout time.Duration) *Renderer {
	t.Helper()
	r := New(timeout)
	t.Cleanup(r.Close)
	return r
}

func rendered(t *testing.T, r *Renderer, source string) string {
	t.Helper()
	out := r.Render(context.Background(), Fence{Line: 1, Source: source})
	if out.Err != nil {
		t.Fatalf("render: %v", out.Err)
	}
	return out.SVG
}

// Geometry is not a contract — d2's native layout ports move nodes and reroute
// edges between releases — so the SVG is asserted on by palette, not by bytes.
func TestRenderPaintsTheDiagramInThePalette(t *testing.T) {
	svg := rendered(t, renderer(t, 0), "a -> b: edge\nc: {shape: cylinder}\nd: {shape: diamond}\ng: {inner}\n")

	for name, colour := range map[string]string{
		"the background": theme.Base,
		"labels":         theme.Text,
		"borders":        theme.Roles[theme.RoleAccentAction],
	} {
		if !strings.Contains(strings.ToLower(svg), strings.ToLower(colour)) {
			t.Errorf("%s are not %s", name, colour)
		}
	}

	for name, colour := range map[string]string{
		"d2's own border blue": "#0D32B2",
		"d2's own ink":         "#0A0F25",
		"d2's own paper":       "#FFFFFF",
	} {
		if strings.Contains(strings.ToUpper(svg), colour) {
			t.Errorf("%s (%s) survived the overrides", name, colour)
		}
	}
}

func TestRenderOmitsTheXMLPrologSoItCanBeInsertedInline(t *testing.T) {
	svg := rendered(t, renderer(t, 0), "a -> b\n")
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("SVG starts with %q", svg[:min(40, len(svg))])
	}
}

func TestRenderRefusesWhatTheWebviewWouldTrust(t *testing.T) {
	cases := map[string]struct {
		source string
		want   error
	}{
		"a markdown block reaches the page as a foreignObject": {
			source: "a: |md\n  # heading\n|\n",
			want:   ErrEmbeddedMarkup,
		},
		"a remote icon would be fetched": {
			source: "a: {icon: https://icons.terrastruct.com/aws/Compute/AWS-Lambda.svg}\n",
			want:   ErrRemoteImage,
		},
		"a javascript link would reach the OS from the context menu": {
			source: "a: {link: \"javascript:alert(1)\"}\n",
			want:   ErrUnsafeLink,
		},
	}

	r := renderer(t, 0)
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out := r.Render(context.Background(), Fence{Line: 1, Source: tc.source})
			if !errors.Is(out.Err, tc.want) {
				t.Fatalf("err = %v, want %v", out.Err, tc.want)
			}
			if out.SVG != "" {
				t.Error("a refused diagram still carries SVG")
			}
		})
	}
}

func TestRenderAllowsAnHTTPLinkAndTheEmbeddedFontDataURIs(t *testing.T) {
	svg := rendered(t, renderer(t, 0), "a: {link: https://example.com}\n")
	if !strings.Contains(svg, "data:application/font-woff") {
		t.Error("the embedded font is gone, so the guard is rejecting d2's own data URIs")
	}
}

// d2 numbers a parse error from the top of the fence; the page needs a document
// line, or the message points at whatever happens to sit near the top of the file.
func TestSyntaxErrorsNameADocumentLine(t *testing.T) {
	out := renderer(t, 0).Render(context.Background(), Fence{Line: 40, Source: "a -> b\nc: {\n"})
	if out.Err == nil {
		t.Fatal("bad d2 compiled")
	}
	if !strings.Contains(out.Err.Error(), "42:") {
		t.Errorf("err = %q, want a line 42 (fence line 40 plus the source's line 2)", out.Err)
	}
}

func TestACacheHitDoesNotRenderAgain(t *testing.T) {
	r := renderer(t, 0)
	source := "cached -> twice\n"

	first := rendered(t, r, source)
	if got, ok := r.Cached(source); !ok || got != first {
		t.Fatalf("the first render did not reach the cache")
	}
	if second := rendered(t, r, source); second != first {
		t.Error("the cached SVG differs from the rendered one")
	}

	// Nothing but the cache can answer with this, so a render that returns it
	// never reached d2.
	planted := "plant -> me\n"
	r.cache.put(cacheKey(planted), outcome{svg: "<svg>planted</svg>"})
	if got := rendered(t, r, planted); got != "<svg>planted</svg>" {
		t.Errorf("Render() drew the diagram instead of answering from the cache")
	}
}

func TestCachedAnswersWithoutJoiningTheQueue(t *testing.T) {
	r := renderer(t, 0)
	source := "probe -> me\n"
	if _, ok := r.Cached(source); ok {
		t.Fatal("an unrendered source is cached")
	}
	svg := rendered(t, r, source)

	// The queue is unbuffered, so a send that completes means the worker has
	// taken the job and is inside it.
	reply := make(chan Result, 1)
	r.jobs <- job{ctx: context.Background(), fence: Fence{Line: 1, Source: fortyNodes()}, reply: reply}

	got, ok := r.Cached(source)
	if !ok || got != svg {
		t.Errorf("Cached() = %t while the worker is busy, want the rendered SVG", ok)
	}
	<-reply
}

func TestTheCacheEvictsOldestFirst(t *testing.T) {
	c := newCache(2)
	c.put("a", outcome{svg: "1"})
	c.put("b", outcome{svg: "2"})
	c.put("c", outcome{svg: "3"})
	if _, ok := c.get("a"); ok {
		t.Error("the oldest entry survived")
	}
	for _, key := range []string{"b", "c"} {
		if _, ok := c.get(key); !ok {
			t.Errorf("%q was evicted early", key)
		}
	}
}

func TestARenderThatOverrunsItsBudgetFails(t *testing.T) {
	out := renderer(t, time.Nanosecond).Render(context.Background(), Fence{Line: 1, Source: "a -> b\n"})
	if !errors.Is(out.Err, ErrTimedOut) {
		t.Fatalf("err = %v, want %v", out.Err, ErrTimedOut)
	}
	if !errors.Is(out.Err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to carry the deadline", out.Err)
	}
	// The pane prints this beside the fence, where Go's own phrasing for a
	// deadline is noise the author cannot act on.
	if got := out.Err.Error(); got != timedOutError {
		t.Errorf("the printed reason is %q, want %q", got, timedOutError)
	}
	if out.SVG != "" {
		t.Error("a diagram that ran out of time still carries SVG")
	}
}

// The goroutine a timeout abandons keeps measuring against the ruler it was
// handed, so the diagram after it has to be given a fresh one.
func TestATimeoutDoesNotPoisonTheNextDiagram(t *testing.T) {
	w := &worker{cache: newCache(cacheEntries), timeout: time.Nanosecond}

	first := w.one(job{ctx: context.Background(), fence: Fence{Line: 1, Source: "a -> b\n"}})
	if !errors.Is(first.Err, ErrTimedOut) {
		t.Fatalf("err = %v, want %v", first.Err, ErrTimedOut)
	}
	if w.ruler != nil {
		t.Error("the abandoned goroutine's ruler was kept")
	}

	w.timeout = DefaultTimeout
	second := w.one(job{ctx: context.Background(), fence: Fence{Line: 2, Source: "c -> d\n"}})
	if second.Err != nil || !strings.HasPrefix(second.SVG, "<svg") {
		t.Fatalf("the diagram after a timeout failed: %v", second.Err)
	}
}

// ELK is EPL-2.0 and goja is a JavaScript interpreter. Neither may reach the
// binary through a d2 import added later.
func TestTheDependencyTreeStaysClean(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/kieranajp/qrouton/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, banned := range []string{"elk-go", "dop251/goja"} {
		if strings.Contains(string(out), banned) {
			t.Errorf("%s is in the dependency tree", banned)
		}
	}
}

// Sizes the render timeout; it is not a gate.
func BenchmarkRenderFortyNodes(b *testing.B) {
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		b.Fatal(err)
	}
	fence := Fence{Line: 1, Source: fortyNodes()}
	ctx := d2log.With(b.Context(), silent)
	if out := render(ctx, ruler, fence); out.Err != nil {
		b.Fatal(out.Err)
	}

	for b.Loop() {
		if out := render(ctx, ruler, fence); out.Err != nil {
			b.Fatal(out.Err)
		}
	}
}

func fortyNodes() string {
	var b strings.Builder
	for group := 0; group < 8; group++ {
		fmt.Fprintf(&b, "g%d: Group %d {\n", group, group)
		for node := 0; node < 5; node++ {
			fmt.Fprintf(&b, "  n%d: Node %d-%d\n", node, group, node)
		}
		for node := 0; node < 4; node++ {
			fmt.Fprintf(&b, "  n%d -> n%d\n", node, node+1)
		}
		b.WriteString("}\n")
	}
	for group := 0; group < 7; group++ {
		fmt.Fprintf(&b, "g%d.n4 -> g%d.n0: hands off\n", group, group+1)
	}
	return b.String()
}

// Close closes nothing a Render can be parked on a send to, so a quit that
// races a mount cannot panic on a closed channel.
func TestCloseDoesNotRaceARenderInFlight(t *testing.T) {
	r := New(0)
	var pending sync.WaitGroup
	for range 8 {
		pending.Add(1)
		go func() {
			defer pending.Done()
			r.Render(context.Background(), Fence{Line: 1, Source: fortyNodes()})
		}()
	}
	r.Close()
	pending.Wait()
	r.Close()
}
