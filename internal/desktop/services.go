package desktop

import (
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type Ollama struct{ ollama *core.Ollama }

func (o *Ollama) Check() core.OllamaStatus { return o.ollama.Check() }

func (o *Ollama) Pull() { o.ollama.Pull() }

func (o *Ollama) CancelPull() { o.ollama.CancelPull() }

type Sessions struct{ sessions *core.Sessions }

func (s *Sessions) Show(slug string) error { return s.sessions.Show(slug) }

func (s *Sessions) Reload(slug string) error { return s.sessions.Reload(slug) }

func (s *Sessions) Unload(slug string) error { return s.sessions.Unload(slug) }

func (s *Sessions) Reorder(slug, targetSlug string) error {
	return s.sessions.Reorder(slug, targetSlug)
}

func (s *Sessions) CycleSticker(slug string) (session.Sticker, error) {
	return s.sessions.CycleSticker(slug)
}

func (s *Sessions) Reveal(slug string) error { return s.sessions.Reveal(slug) }

func (s *Sessions) RevealPath(slug, path string) error { return s.sessions.RevealPath(slug, path) }

func (s *Sessions) CopyAsset(slug, path string) error { return s.sessions.CopyAsset(slug, path) }

func (s *Sessions) Uncommitted(slug string) ([]string, error) { return s.sessions.Uncommitted(slug) }

func (s *Sessions) Cleanup(slug string) error { return s.sessions.Cleanup(slug) }

type Term struct{ term *core.Term }

func (t *Term) Start(id string, cols, rows int) error { return t.term.Start(id, cols, rows) }

func (t *Term) Write(id, encoded string) error { return t.term.Write(id, encoded) }

func (t *Term) Resize(id string, cols, rows int) error { return t.term.Resize(id, cols, rows) }

type Windows struct{ windows *core.Windows }

func (w *Windows) OpenShell() (string, error) { return w.windows.OpenShell() }

func (w *Windows) OpenDocument(name string) (string, error) { return w.windows.OpenDocument(name) }

func (w *Windows) Select(slug, id string) error { return w.windows.Select(slug, id) }

func (w *Windows) Reorder(slug, id string, to int) error { return w.windows.Reorder(slug, id, to) }

func (w *Windows) Start(id string, cols, rows int) error { return w.windows.Start(id, cols, rows) }

func (w *Windows) Write(id, encoded string) error { return w.windows.Write(id, encoded) }

func (w *Windows) Resize(id string, cols, rows int) error { return w.windows.Resize(id, cols, rows) }

func (w *Windows) Content(id string) (core.Document, error) { return w.windows.Content(id) }

func (w *Windows) RenderDiagrams(id string, request uint64) ([]core.RenderedDiagram, error) {
	return w.windows.RenderDiagrams(id, request)
}

func (w *Windows) ReportViewport(id string, report core.ViewportReport) error {
	return w.windows.ReportViewport(id, report)
}

func (w *Windows) Close(id string) error { return w.windows.Close(id) }

func (w *Windows) Surfaces(slug string) core.Surfaces { return w.windows.Surfaces(slug) }

func (w *Windows) FocusAsset(slug, id string, index int) (workbench.AssetSelection, error) {
	return w.windows.FocusAsset(slug, id, index)
}

func (w *Windows) SaveAnswers(id, hash, text string) (string, error) {
	return w.windows.SaveAnswers(id, hash, text)
}

func (w *Windows) SendSpecAnswers(id string) error { return w.windows.SendSpecAnswers(id) }

func (w *Windows) SendAuditTriage(id string) error { return w.windows.SendAuditTriage(id) }

type BugReports struct{ reports *core.BugReports }

func (b *BugReports) Load(slug, requestID string) (core.BugReportPreview, error) {
	return b.reports.Load(slug, requestID)
}

func (b *BugReports) Cancel(slug, requestID string) (workbench.BugReport, error) {
	return b.reports.Cancel(slug, requestID)
}

func (b *BugReports) Confirm(slug, requestID string) (workbench.BugReport, error) {
	return b.reports.Confirm(slug, requestID)
}

type Chrome struct{ chrome *core.Chrome }

func (c *Chrome) AgentTree(slug string) status.AgentTree { return c.chrome.AgentTree(slug) }

func (c *Chrome) Snapshot() status.Fields { return c.chrome.Snapshot() }
