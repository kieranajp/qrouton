package api

import (
	"sync"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/status"
)

const (
	backlogSize   = 1024
	queueSize     = 4096
	coalesceLimit = 64 << 10
)

// Hub numbers every core event and fans it out to each subscriber's queue.
// Terminal events stay out of the backlog: a terminal resumes by offset.
type Hub struct {
	mu      sync.Mutex
	seq     uint64
	backlog []*qroutonv1.Event
	// evicted is the newest sequence number the backlog no longer holds.
	evicted uint64
	subs    map[*subscriber]struct{}
}

type subscriber struct {
	client   string
	queue    chan *qroutonv1.Event
	overflow chan struct{}
}

var _ core.Events = (*Hub)(nil)

func NewHub() *Hub {
	return &Hub{subs: map[*subscriber]struct{}{}}
}

func (h *Hub) publish(event *qroutonv1.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	event.Seq = h.seq
	if event.GetTerminalOutput() == nil && event.GetTerminalExit() == nil {
		if len(h.backlog) == backlogSize {
			h.evicted = h.backlog[0].Seq
			h.backlog = h.backlog[1:]
		}
		h.backlog = append(h.backlog, event)
	}
	for sub := range h.subs {
		h.enqueueLocked(sub, event)
	}
}

// deliver numbers event and queues it for one client's subscribers alone.
func (h *Hub) deliver(client string, event *qroutonv1.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seq++
	event.Seq = h.seq
	for sub := range h.subs {
		if sub.client == client {
			h.enqueueLocked(sub, event)
		}
	}
}

func (h *Hub) enqueueLocked(sub *subscriber, event *qroutonv1.Event) {
	select {
	case sub.queue <- event:
	default:
		delete(h.subs, sub)
		close(sub.overflow)
	}
}

// subscribe registers a queue and, under the same lock, takes what it missed
// after after, so nothing falls between the backlog and the queue.
func (h *Hub) subscribe(client string, after uint64) (sub *subscriber, missed []*qroutonv1.Event, resync *qroutonv1.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sub = &subscriber{client: client, queue: make(chan *qroutonv1.Event, queueSize), overflow: make(chan struct{})}
	h.subs[sub] = struct{}{}
	switch {
	case after == 0 || after == h.seq:
	case after > h.seq || after < h.evicted:
		resync = &qroutonv1.Event{Seq: h.seq, Payload: &qroutonv1.Event_Resync{Resync: &qroutonv1.Resync{}}}
	default:
		for _, event := range h.backlog {
			if event.Seq > after {
				missed = append(missed, event)
			}
		}
	}
	return sub, missed, resync
}

func (h *Hub) unsubscribe(sub *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subs, sub)
}

func (h *Hub) Ollama(s core.OllamaStatus) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_Ollama{Ollama: ollamaStatus(s)}})
}

func (h *Hub) Chrome(f status.Fields) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_Chrome{Chrome: chromeFields(f)}})
}

func (h *Hub) AgentTree(slug string, tree status.AgentTree) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_AgentTree{AgentTree: &qroutonv1.AgentTreeChanged{Slug: slug, Tree: agentTree(tree)}}})
}

func (h *Hub) WindowsOpened(s core.Surfaces) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_WindowsOpened{WindowsOpened: surfaces(s)}})
}

func (h *Hub) WindowContent(id string, doc core.Document) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_WindowContent{WindowContent: &qroutonv1.WindowContent{Id: id, Document: document(doc)}}})
}

func (h *Hub) WindowDiagram(id string, diagram core.RenderedDiagram) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_WindowDiagram{WindowDiagram: &qroutonv1.WindowDiagram{Id: id, Diagram: renderedDiagram(diagram)}}})
}

func (h *Hub) TerminalOutput(kind core.TerminalKind, id string, out core.Output) {
	h.publish(terminalOutput(kind, id, out))
}

func (h *Hub) TerminalExit(kind core.TerminalKind, id string, code int) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_TerminalExit{TerminalExit: &qroutonv1.TerminalExit{TerminalId: id, Kind: terminalKind(kind), Code: int32(code)}}})
}

func (h *Hub) RepositoryRefresh(e core.RefreshEvent) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_RepositoryRefresh{RepositoryRefresh: repositoryRefresh(e)}})
}

func (h *Hub) AssemblyProgress(e core.ProgressEvent) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_AssemblyProgress{AssemblyProgress: assemblyProgress(e)}})
}

func (h *Hub) AssemblyRequested(ticket string) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_AssemblyRequested{AssemblyRequested: &qroutonv1.AssemblyRequested{Ticket: ticket}}})
}

func (h *Hub) OrgsChanged(orgs []string) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_OrgsChanged{OrgsChanged: &qroutonv1.OrgsChanged{Orgs: orgs}}})
}

func (h *Hub) UIScale(scale int) {
	h.publish(&qroutonv1.Event{Payload: &qroutonv1.Event_UiScale{UiScale: &qroutonv1.UIScaleChanged{Scale: int32(scale)}}})
}
