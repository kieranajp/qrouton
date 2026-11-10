package core

import "github.com/kieranajp/qrouton/internal/status"

// Events is everything the core tells its clients, one method per event, so a
// client that misses one does not compile. Each call returns without waiting
// for delivery, which lets a caller hold a lock that orders its events.
type Events interface {
	Ollama(OllamaStatus)
	Chrome(status.Fields)
	AgentTree(slug string, tree status.AgentTree)
	WindowsOpened(Surfaces)
	WindowContent(id string, doc Document)
	WindowDiagram(id string, diagram RenderedDiagram)
	TerminalOutput(kind TerminalKind, id string, chunk Chunk)
	TerminalExit(kind TerminalKind, id string, code int)
	RepositoryRefresh(RefreshEvent)
	AssemblyProgress(ProgressEvent)
	AssemblyRequested(ticket string)
	OrgsChanged(orgs []string)
	UIScale(scale int)
}

// TerminalKind says which pane a terminal's output is drawn in.
type TerminalKind int

const (
	TerminalConversation TerminalKind = iota + 1
	TerminalTab
)
