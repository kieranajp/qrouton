package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/kieranajp/qrouton/internal/api"
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/api/pagejson"
	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/placement"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

type Ollama struct{ ollama qroutonv1.OllamaServiceClient }

func (o *Ollama) Check() (json.RawMessage, error) {
	resp, err := o.ollama.Check(context.Background(), &qroutonv1.CheckRequest{})
	if err != nil {
		return nil, api.FromStatus(err)
	}
	return pagejson.Marshal(resp.GetStatus()), nil
}

func (o *Ollama) Pull() error {
	_, err := o.ollama.Pull(context.Background(), &qroutonv1.PullRequest{})
	return api.FromStatus(err)
}

func (o *Ollama) CancelPull() error {
	_, err := o.ollama.CancelPull(context.Background(), &qroutonv1.CancelPullRequest{})
	return api.FromStatus(err)
}

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

type Term struct {
	terminals qroutonv1.TerminalServiceClient
}

func (t *Term) Start(id string, cols, rows int) error { return attach(t.terminals, id, cols, rows) }

func (t *Term) Write(id, encoded string) error { return write(t.terminals, id, encoded) }

func (t *Term) Resize(id string, cols, rows int) error { return resize(t.terminals, id, cols, rows) }

// attach is the page mounting a terminal: it starts one nothing has, and
// otherwise replays it from the start, since the page keeps no offset.
func attach(terminals qroutonv1.TerminalServiceClient, id string, cols, rows int) error {
	_, err := terminals.Attach(context.Background(), &qroutonv1.AttachRequest{TerminalId: id, Size: terminalSize(cols, rows)})
	return api.FromStatus(err)
}

func write(terminals qroutonv1.TerminalServiceClient, id, encoded string) error {
	data, err := decodeInput(encoded)
	if err != nil {
		return err
	}
	_, err = terminals.Write(context.Background(), &qroutonv1.WriteRequest{TerminalId: id, Data: data})
	return api.FromStatus(err)
}

func resize(terminals qroutonv1.TerminalServiceClient, id string, cols, rows int) error {
	_, err := terminals.Resize(context.Background(), &qroutonv1.ResizeRequest{TerminalId: id, Size: terminalSize(cols, rows)})
	return api.FromStatus(err)
}

func terminalSize(cols, rows int) *qroutonv1.TerminalSize {
	return &qroutonv1.TerminalSize{Cols: int32(cols), Rows: int32(rows)}
}

// decodeInput is base64 because the page's keystrokes are bytes, and a
// bound string argument must be valid UTF-8.
func decodeInput(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

type Windows struct {
	windows   *core.Windows
	terminals qroutonv1.TerminalServiceClient
}

func (w *Windows) OpenShell() (string, error) { return w.windows.OpenShell() }

func (w *Windows) OpenDocument(name string) (string, error) { return w.windows.OpenDocument(name) }

func (w *Windows) Select(slug, id string) error { return w.windows.Select(slug, id) }

func (w *Windows) Reorder(slug, id string, to int) error { return w.windows.Reorder(slug, id, to) }

func (w *Windows) Start(id string, cols, rows int) error { return attach(w.terminals, id, cols, rows) }

func (w *Windows) Write(id, encoded string) error { return write(w.terminals, id, encoded) }

func (w *Windows) Resize(id string, cols, rows int) error { return resize(w.terminals, id, cols, rows) }

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

type Assembly struct{ assembly *core.Assembly }

func (a *Assembly) Prefixes() []string { return a.assembly.Prefixes() }

func (a *Assembly) Runners() ([]assembly.Runner, error) { return a.assembly.Runners() }

func (a *Assembly) Check(in core.DraftInput) []assembly.Problem { return a.assembly.Check(in) }

func (a *Assembly) CheckSlug(in core.DraftInput) []assembly.Problem { return a.assembly.CheckSlug(in) }

func (a *Assembly) Preview(in core.DraftInput) string { return a.assembly.Preview(in) }

func (a *Assembly) Fetch(url string) (core.TicketFields, error) { return a.assembly.Fetch(url) }

func (a *Assembly) Pending() string { return a.assembly.Pending() }

func (a *Assembly) Begin() core.AssemblySeed { return a.assembly.Begin() }

func (a *Assembly) End(generation uint64) { a.assembly.End(generation) }

func (a *Assembly) Create(in core.DraftInput) error { return a.assembly.Create(in) }

type Repositories struct{ repos *core.Repositories }

func (r *Repositories) Cached() []github.Repo { return r.repos.Cached() }

func (r *Repositories) Select(picks []core.RepoPick) []session.RepoSelection {
	return r.repos.Select(picks)
}

func (r *Repositories) Branches(id string) core.BranchList { return r.repos.Branches(id) }

func (r *Repositories) Refresh() int { return r.repos.Refresh() }

type Orgs struct{ orgs *core.Orgs }

func (o *Orgs) List() []string { return o.orgs.List() }

type Picker struct{ picker *core.Picker }

func (p *Picker) Escalate(slug string) error { return p.picker.Escalate(slug) }

func (p *Picker) Load(slug string) (core.PickerFields, error) { return p.picker.Load(slug) }

func (p *Picker) Confirm(slug string, in core.PickerInput) error { return p.picker.Confirm(slug, in) }

func (p *Picker) Cancel(slug string) error { return p.picker.Cancel(slug) }

type Settings struct{ settings *core.Settings }

func (s *Settings) Load() core.SettingsView { return s.settings.Load() }

func (s *Settings) Save(in core.SettingsInput) (core.SaveResult, error) { return s.settings.Save(in) }

func (s *Settings) AdjustUIScale(action string) (int, error) { return s.settings.AdjustUIScale(action) }

func (s *Settings) GitHubStatus() core.GitHubStatus { return s.settings.GitHubStatus() }

func (s *Settings) Relaunch() error { return s.settings.Relaunch() }

func (s *Settings) OpenConfig() error { return s.settings.OpenConfig() }

func (s *Settings) InstallLinear() (core.LinearView, error) { return s.settings.InstallLinear() }

type FirstRun struct{ firstRun *core.FirstRun }

func (f *FirstRun) Login() string { return f.firstRun.Login() }

func (f *FirstRun) ChooseRoot() (string, error) { return f.firstRun.ChooseRoot() }

func (f *FirstRun) ThoughtsDefault(root string) string { return f.firstRun.ThoughtsDefault(root) }

func (f *FirstRun) Save(in core.FirstRunInput) (core.FirstRunResult, error) {
	return f.firstRun.Save(in)
}

type Placement struct{ placement *core.Placement }

func (p *Placement) Check() error { return p.placement.Check() }

func (p *Placement) Report() core.PlacementView { return p.placement.Report() }

func (p *Placement) Move(slugs []string) []placement.Outcome { return p.placement.Move(slugs) }

func (p *Placement) Close() error { return p.placement.Close() }
