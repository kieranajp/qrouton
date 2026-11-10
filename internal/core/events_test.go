package core

import "github.com/kieranajp/qrouton/internal/status"

const (
	ollamaName            = "ollama:status"
	ptyDataName           = "pty:data:"
	ptyExitName           = "pty:exit:"
	chromeName            = "chrome:update"
	agentTreeName         = "agents:tree:"
	windowDataName        = "window:data:"
	windowExitName        = "window:exit:"
	windowDiagramName     = "window:diagram:"
	windowContentName     = "window:content:"
	windowsName           = "window:open"
	reposRefreshName      = "repos:refresh"
	assemblyProgressName  = "assembly:progress"
	assemblyRequestedName = "assembly:requested"
	orgsChangedName       = "orgs:changed"
	uiScaleName           = "ui:scale"
)

// emitFunc flattens each event back into a name and payload, so a test can
// record every kind of event through one func.
type emitFunc func(event string, payload any)

func (e emitFunc) Ollama(s OllamaStatus)  { e(ollamaName, s) }
func (e emitFunc) Chrome(f status.Fields) { e(chromeName, f) }
func (e emitFunc) AgentTree(slug string, tree status.AgentTree) {
	e(agentTreeName+slug, tree)
}
func (e emitFunc) WindowsOpened(s Surfaces) { e(windowsName, s) }
func (e emitFunc) WindowContent(id string, doc Document) {
	e(windowContentName+id, doc)
}
func (e emitFunc) WindowDiagram(id string, d RenderedDiagram) {
	e(windowDiagramName+id, d)
}
func (e emitFunc) TerminalOutput(kind TerminalKind, id string, out Output) {
	if kind == TerminalConversation {
		e(ptyDataName+id, out)
		return
	}
	e(windowDataName+id, out)
}
func (e emitFunc) TerminalExit(kind TerminalKind, id string, code int) {
	if kind == TerminalConversation {
		e(ptyExitName+id, code)
		return
	}
	e(windowExitName+id, code)
}
func (e emitFunc) RepositoryRefresh(r RefreshEvent) { e(reposRefreshName, r) }
func (e emitFunc) AssemblyProgress(p ProgressEvent) { e(assemblyProgressName, p) }
func (e emitFunc) AssemblyRequested(ticket string)  { e(assemblyRequestedName, ticket) }
func (e emitFunc) OrgsChanged(orgs []string)        { e(orgsChangedName, orgs) }
func (e emitFunc) UIScale(scale int)                { e(uiScaleName, scale) }

// startTerm is a page's first attach, which starts the conversation at its size.
func startTerm(term *Term, id string, cols, rows int) error {
	return term.Attach(id, &Size{Cols: cols, Rows: rows}, nil, func(Output) {})
}

// attachTab is a page attaching to a tab at its size. The replay lands in r
// under the tab's data event, as the old replay did.
func attachTab(w *Windows, r *fakeClient, id string, cols, rows int) error {
	return w.Attach(id, &Size{Cols: cols, Rows: rows}, nil, func(out Output) {
		if r != nil {
			r.Emit(windowDataName+id, out)
		}
	})
}
