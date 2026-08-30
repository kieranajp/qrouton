package desktop

import (
	"context"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// Windows is the service the agent's window tools and the session's page both
// reach a tab through: the registry of open tabs, plus the document poll, the
// terminal processes and the diagram worker, each owning what only it touches.
type Windows struct {
	*registry
	documents *documents
	terminals *terminals
	diagrams  *diagramWorker
	// newShell reopens the shell after the user closes its tab.
	newShell    func() (string, error)
	newDocument func(name string) (string, error)
	stopFollow  context.CancelFunc
}

func newWindows(emit emitter, sessions *Sessions) *Windows {
	tabs := newRegistry(emit, sessions)
	ctx, stop := context.WithCancel(context.Background())
	w := &Windows{
		registry:   tabs,
		documents:  newDocuments(emit, tabs),
		terminals:  newTerminals(emit, tabs),
		diagrams:   newDiagramWorker(emit),
		stopFollow: stop,
	}
	go w.documents.follow(ctx)
	return w
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
	return w.showOrOpen(w.shown(), name, func() (string, error) {
		if w.newDocument == nil {
			return "", ErrNoEditorCommand
		}
		return w.newDocument(name)
	})
}

// Select records a user-driven tab selection for one session.
func (w *Windows) Select(slug, id string) error { return w.selectBySlug(slug, id) }

func (w *Windows) Start(id string, cols, rows int) error { return w.terminals.start(id, cols, rows) }

func (w *Windows) Write(id, encoded string) error { return w.terminals.write(id, encoded) }

func (w *Windows) Resize(id string, cols, rows int) error { return w.terminals.resize(id, cols, rows) }

// Content is a document window's text, fetched by its page on load.
func (w *Windows) Content(id string) (document, error) { return w.documents.content(id) }

// RenderDiagrams names every d2 fence in a document window, carrying the SVG of
// the ones already rendered; the rest arrive on windowDiagramEvent as they land.
// A window with nothing to draw answers with an empty list rather than an error:
// every Markdown pane calls this, diagrams or not.
func (w *Windows) RenderDiagrams(id string) ([]renderedDiagram, error) {
	text, markdown, err := w.documents.markdown(id)
	if err != nil {
		return nil, err
	}
	if !markdown {
		return []renderedDiagram{}, nil
	}
	return w.diagrams.render(id, text), nil
}

// ReportViewport stores the newest browser measurement for a Markdown tab.
func (w *Windows) ReportViewport(id string, report ViewportReport) error {
	return w.documents.report(id, report)
}

// Close serves the agent's window tool and the tab strip's close control. Wails
// binds exported methods only, so unexporting this silently breaks the tab.
func (w *Windows) Close(id string) error {
	if !w.exists(id) {
		return noSuchWindow(id)
	}
	w.discard(id)
	return nil
}

// Surfaces answers a session's page on load: the event only fires on a change,
// and the shell opens before anything is subscribed.
func (w *Windows) Surfaces(slug string) surfaces { return w.surfacesBySlug(slug) }

func (w *Windows) viewport(owner *sessionState, id string) (*workbench.DocumentViewport, error) {
	return w.documents.viewport(owner, id)
}

// stopAll tears every window down, so closing the conversation leaves nothing
// running behind it.
func (w *Windows) stopAll() {
	w.registry.stopAll()
	w.stopFollow()
	w.diagrams.stop()
}
