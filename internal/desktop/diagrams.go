package desktop

import (
	"context"
	"sync"

	"github.com/kieranajp/qrouton/internal/diagram"
)

// renderedDiagram is one d2 fence as the page receives it: the document line
// its opening marker sits on, and either the SVG, the reason there is none, or
// neither while it is still being laid out.
type renderedDiagram struct {
	Line  int    `json:"line"`
	SVG   string `json:"svg,omitempty"`
	Error string `json:"error,omitempty"`
}

// diagramWorker lays a document's d2 fences out off the goroutine that asked
// for them, and holds the cancellation a quit reaches them through.
type diagramWorker struct {
	emit      emitter
	renderer  *diagram.Renderer
	ctx       context.Context
	cancel    context.CancelFunc
	rendering sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

func newDiagramWorker(emit emitter) *diagramWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &diagramWorker{
		emit:     emit,
		renderer: diagram.New(diagram.DefaultTimeout),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// render names every d2 fence in a document, carrying the SVG of the ones
// already rendered; the rest are laid out off this goroutine and arrive on
// windowDiagramEvent as they land. Opening a document costs a scan, never a
// layout.
func (d *diagramWorker) render(id, text string) []renderedDiagram {
	found := []renderedDiagram{}
	var misses []diagram.Fence
	for _, fence := range diagram.Scan(text) {
		if svg, hit := d.renderer.Cached(fence.Source); hit {
			found = append(found, renderedDiagram{Line: fence.Line, SVG: svg})
			continue
		}
		found = append(found, renderedDiagram{Line: fence.Line})
		misses = append(misses, fence)
	}
	d.layOut(id, misses)
	return found
}

// layOut renders one document's misses in document order, emitting each as it
// finishes. It declines once the renderer is stopping, so a quit cannot race a
// send at a worker that is shutting down.
func (d *diagramWorker) layOut(id string, fences []diagram.Fence) {
	if len(fences) == 0 {
		return
	}
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.rendering.Add(1)
	d.mu.Unlock()
	go func() {
		defer d.rendering.Done()
		for _, fence := range fences {
			out := d.renderer.Render(d.ctx, fence)
			if d.ctx.Err() != nil {
				return
			}
			d.emit(windowDiagramEvent+id, drawnDiagram(out))
		}
	}()
}

func drawnDiagram(out diagram.Result) renderedDiagram {
	if out.Err != nil {
		return renderedDiagram{Line: out.Line, Error: out.Err.Error()}
	}
	return renderedDiagram{Line: out.Line, SVG: out.SVG}
}

// stop cancels what is in flight before closing the worker: the cancellation
// reaches the layout already underway, so quitting mid-diagram does not wait
// out the render budget.
func (d *diagramWorker) stop() {
	d.mu.Lock()
	closed := d.closed
	d.closed = true
	d.mu.Unlock()
	if closed {
		return
	}
	d.cancel()
	d.rendering.Wait()
	d.renderer.Close()
}
