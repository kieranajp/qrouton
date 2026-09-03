package desktop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

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

type ViewportReport struct {
	Epoch     uint64                   `json:"epoch"`
	Seq       uint64                   `json:"seq"`
	Available bool                     `json:"available"`
	Selected  bool                     `json:"selected"`
	Intervals []workbench.LineInterval `json:"intervals"`
}

type documents struct {
	emit     emitter
	registry *registry
}

func newDocuments(emit emitter, reg *registry) *documents {
	return &documents{emit: emit, registry: reg}
}

func (window *agentWindow) sourcePath() string {
	if window.opts.Source == "" || window.session == nil {
		return ""
	}
	return filepath.Join(window.session.root(), filepath.FromSlash(window.opts.Source))
}

func beginDocument(window *agentWindow) {
	if window.opts.Kind == workbench.KindDocument && window.opts.Format == workbench.FormatMarkdown {
		window.viewport = workbench.UnmeasuredViewport(window.opts.Source)
	}
	// The content arrived from a read taken before this stat. A size that no
	// longer matches it means the file moved in between, so it is left unseen
	// for the first rescan to pick up rather than recorded as already read.
	if info, err := os.Stat(window.sourcePath()); err == nil && info.Size() == int64(len(window.opts.Content)) {
		window.read.at, window.read.size = info.ModTime(), info.Size()
	}
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

// follow keeps open documents current. A stat a second buys what a file
// watcher would, and reads a write-then-rename the same as a write in place.
func (d *documents) follow(ctx context.Context) {
	ticker := time.NewTicker(documentPoll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.rescan()
		}
	}
}

// rescan pushes every open document whose file has changed to its page. A push
// is not a reload: the same controller keeps reporting against a monotonic
// sequence, so the epoch is reused and the sequence left where it is. Bumping
// either would fence off reports the page is still entitled to send.
func (d *documents) rescan() {
	type push struct {
		id  string
		doc document
	}
	var pushes []push
	d.registry.each(func(id string, window *agentWindow) {
		if window.opts.Kind != workbench.KindDocument {
			return
		}
		path := window.sourcePath()
		if path == "" {
			return
		}
		info, err := os.Stat(path)
		// An unreadable file leaves the tab showing what it last held. A file
		// caught between truncation and rewrite still reads empty for a tick.
		if err != nil || info.IsDir() || info.Size() > workbench.DocumentLimit {
			return
		}
		if info.Size() == window.read.size && info.ModTime().Equal(window.read.at) {
			return
		}
		text, err := os.ReadFile(path)
		if err != nil {
			return
		}
		window.read.at, window.read.size = info.ModTime(), info.Size()
		window.opts.Content = string(text)
		pushes = append(pushes, push{id: id, doc: documentFor(window)})
	})
	for _, sent := range pushes {
		d.emit(windowContentEvent+sent.id, sent.doc)
	}
}

// content is a document window's text, fetched by its page on load. A load
// restarts the page's sequence counter, so the epoch moves and the viewport
// starts again from nothing measured.
func (d *documents) content(id string) (document, error) {
	var doc document
	err := d.registry.with(id, func(window *agentWindow) error {
		if window.viewport != nil {
			window.viewportEpoch++
			window.viewportSeq = 0
			window.viewport = workbench.UnmeasuredViewport(window.opts.Source)
		}
		doc = documentFor(window)
		return nil
	})
	if err != nil {
		return document{}, err
	}
	return doc, nil
}

// markdown is a window's text and whether it is Markdown, which is the only
// format anything is drawn into rather than shown as written.
func (d *documents) markdown(id string) (string, bool, error) {
	var text string
	var rendered bool
	err := d.registry.with(id, func(window *agentWindow) error {
		rendered = window.opts.Kind == workbench.KindDocument && window.opts.Format == workbench.FormatMarkdown
		text = window.opts.Content
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return text, rendered, nil
}

func (d *documents) report(id string, report ViewportReport) error {
	return d.registry.with(id, func(window *agentWindow) error {
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
			intervals = workbench.NoIntervals()
		}
		window.viewportSeq = report.Seq
		window.viewport = &workbench.DocumentViewport{
			Source:    window.opts.Source,
			Available: available,
			Selected:  report.Selected,
			Intervals: intervals,
		}
		return nil
	})
}

func (d *documents) viewport(owner *sessionState, id string) (*workbench.DocumentViewport, error) {
	var view *workbench.DocumentViewport
	err := d.registry.with(id, func(window *agentWindow) error {
		if window.session != owner {
			return noSuchWindow(id)
		}
		if window.viewport == nil {
			return nil
		}
		measured := *window.viewport
		measured.Intervals = append(workbench.NoIntervals(), window.viewport.Intervals...)
		view = &measured
		return nil
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

func normalizedIntervals(intervals []workbench.LineInterval) ([]workbench.LineInterval, error) {
	if len(intervals) == 0 {
		return workbench.NoIntervals(), nil
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
