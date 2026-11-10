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
func (e emitFunc) TerminalOutput(kind TerminalKind, id string, chunk Chunk) {
	if kind == TerminalConversation {
		e(ptyDataName+id, chunk)
		return
	}
	e(windowDataName+id, chunk)
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
