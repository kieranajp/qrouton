package desktop

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/kieranajp/qrouton/internal/api"
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/api/pagejson"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// emitter puts one payload in front of every page.
type emitter func(event string, payload any)

// translator turns the core's event stream back into the page's named events.
// It owns those names, and remembers the last event so a resubscribe resumes.
type translator struct {
	events    qroutonv1.EventServiceClient
	windows   qroutonv1.WindowServiceClient
	terminals *terminals
	emit      emitter
	after     uint64
}

type eventStream = grpc.ServerStreamingClient[qroutonv1.SubscribeResponse]

// subscribe returns once the core has registered the stream, so nothing
// published after it is missed.
func (t *translator) subscribe(ctx context.Context) (eventStream, error) {
	stream, err := t.events.Subscribe(ctx, &qroutonv1.SubscribeRequest{AfterSeq: t.after})
	if err != nil {
		return nil, err
	}
	header, err := stream.Header()
	if err != nil {
		return nil, err
	}
	if t.after == 0 {
		if seq := header.Get(api.SeqHeader); len(seq) == 1 {
			t.after, _ = strconv.ParseUint(seq[0], 10, 64)
		}
	}
	return stream, nil
}

// run delivers stream until it ends, then resubscribes until ctx is done.
func (t *translator) run(ctx context.Context, stream eventStream) {
	for {
		for {
			resp, err := stream.Recv()
			if err != nil {
				break
			}
			t.after = resp.GetEvent().GetSeq()
			t.deliver(resp.GetEvent())
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(resubscribeDelay):
			}
			next, err := t.subscribe(ctx)
			if err == nil {
				stream = next
				t.reattach(ctx)
				break
			}
		}
	}
}

// reattach replays the whole ring of every started terminal the page has
// mounted, since output and exits during the break reached no stream. Only a
// pending mount passes a size.
func (t *translator) reattach(ctx context.Context) {
	listed, err := t.windows.ListTerminals(ctx, &qroutonv1.ListTerminalsRequest{})
	if err != nil {
		if ctx.Err() == nil {
			log.Printf(reattachLogFormat, err)
		}
		return
	}
	pending := t.terminals.takePending()
	mounted := t.terminals.mountedOf(listed.GetTerminals())
	for _, terminal := range listed.GetTerminals() {
		id := terminal.GetTerminalId()
		size := pending[id]
		if terminal.GetSize() == nil || !mounted[id] {
			continue
		}
		_, err := t.terminals.Attach(ctx, &qroutonv1.AttachRequest{TerminalId: id, Size: size})
		switch status.Code(err) {
		case codes.OK:
		case codes.NotFound:
			t.emit(terminalEvent(terminal.GetKind(), ptyExitEvent, windowExitEvent)+id, -1)
		default:
			if size != nil {
				t.terminals.keepPending(id, size)
			}
		}
	}
}

func (t *translator) deliver(event *qroutonv1.Event) {
	switch payload := event.GetPayload().(type) {
	case *qroutonv1.Event_Ollama:
		t.emit(ollamaEvent, pagejson.Marshal(payload.Ollama))
	case *qroutonv1.Event_Dashboard:
		t.emit(dashboardEvent, pagejson.Marshal(payload.Dashboard))
	case *qroutonv1.Event_Chrome:
		t.emit(chromeEvent, pagejson.Marshal(payload.Chrome))
	case *qroutonv1.Event_AgentTree:
		t.emit(agentTreeEvent+payload.AgentTree.GetSlug(), pagejson.Marshal(payload.AgentTree.GetTree()))
	case *qroutonv1.Event_WindowsOpened:
		t.emit(windowsEvent, pagejson.Marshal(payload.WindowsOpened))
	case *qroutonv1.Event_WindowContent:
		t.emit(windowContentEvent+payload.WindowContent.GetId(), pagejson.Marshal(payload.WindowContent.GetDocument()))
	case *qroutonv1.Event_WindowDiagram:
		t.emit(windowDiagramEvent+payload.WindowDiagram.GetId(), pagejson.Marshal(payload.WindowDiagram.GetDiagram()))
	case *qroutonv1.Event_TerminalOutput:
		output := payload.TerminalOutput
		t.emit(terminalEvent(output.GetKind(), ptyDataEvent, windowDataEvent)+output.GetTerminalId(), pagejson.Marshal(output))
	case *qroutonv1.Event_TerminalExit:
		exit := payload.TerminalExit
		t.emit(terminalEvent(exit.GetKind(), ptyExitEvent, windowExitEvent)+exit.GetTerminalId(), int(exit.GetCode()))
	case *qroutonv1.Event_RepositoryRefresh:
		t.emit(reposRefreshEvent, pagejson.Marshal(payload.RepositoryRefresh))
	case *qroutonv1.Event_AssemblyProgress:
		t.emit(assemblyProgressEvent, pagejson.Marshal(payload.AssemblyProgress))
	case *qroutonv1.Event_AssemblyRequested:
		t.emit(assemblyRequestedEvent, payload.AssemblyRequested.GetTicket())
	case *qroutonv1.Event_OrgsChanged:
		t.emit(orgsChangedEvent, payload.OrgsChanged.GetOrgs())
	case *qroutonv1.Event_UiScale:
		t.emit(uiScaleEvent, int(payload.UiScale.GetScale()))
	case *qroutonv1.Event_Resync:
		log.Printf(resyncLogFormat, event.GetSeq())
	}
}

func terminalEvent(kind qroutonv1.TerminalKind, conversation, tab string) string {
	if kind == qroutonv1.TerminalKind_TERMINAL_KIND_CONVERSATION {
		return conversation
	}
	return tab
}
