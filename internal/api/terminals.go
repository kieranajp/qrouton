package api

import (
	"context"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// terminalService routes an id to the conversation it names, or else to a tab.
type terminalService struct {
	qroutonv1.UnimplementedTerminalServiceServer
	core *core.Core
	hub  *Hub
}

func (t *terminalService) conversation(id string) bool {
	_, ok := t.core.Sessions.ByTerminal(id)
	return ok
}

// Attach hands the replay to the caller's own queues alone.
func (t *terminalService) Attach(ctx context.Context, req *qroutonv1.AttachRequest) (*qroutonv1.AttachResponse, error) {
	id := req.GetTerminalId()
	var size *core.Size
	if req.GetSize() != nil {
		size = &core.Size{Cols: int(req.GetSize().GetCols()), Rows: int(req.GetSize().GetRows())}
	}
	var from *uint64
	if req.FromOffset != nil {
		offset := req.GetFromOffset()
		from = &offset
	}
	client := clientID(ctx)
	if t.conversation(id) {
		return &qroutonv1.AttachResponse{}, t.core.Term.Attach(id, size, from, func(out core.Output) {
			t.hub.deliver(client, terminalOutput(core.TerminalConversation, id, out))
		})
	}
	return &qroutonv1.AttachResponse{}, t.core.Windows.Attach(id, size, from, func(out core.Output) {
		t.hub.deliver(client, terminalOutput(core.TerminalTab, id, out))
	})
}

func (t *terminalService) Write(_ context.Context, req *qroutonv1.WriteRequest) (*qroutonv1.WriteResponse, error) {
	id := req.GetTerminalId()
	if t.conversation(id) {
		return &qroutonv1.WriteResponse{}, t.core.Term.Write(id, req.GetData())
	}
	return &qroutonv1.WriteResponse{}, t.core.Windows.Write(id, req.GetData())
}

func (t *terminalService) Resize(_ context.Context, req *qroutonv1.ResizeRequest) (*qroutonv1.ResizeResponse, error) {
	if req.GetSize() == nil {
		return nil, status.Error(codes.InvalidArgument, ErrNoSize.Error())
	}
	id, cols, rows := req.GetTerminalId(), int(req.GetSize().GetCols()), int(req.GetSize().GetRows())
	if t.conversation(id) {
		return &qroutonv1.ResizeResponse{}, t.core.Term.Resize(id, cols, rows)
	}
	return &qroutonv1.ResizeResponse{}, t.core.Windows.Resize(id, cols, rows)
}

func (t *terminalService) Session(_ context.Context, req *qroutonv1.SessionRequest) (*qroutonv1.SessionResponse, error) {
	return &qroutonv1.SessionResponse{Slug: t.core.TerminalSession(req.GetTerminalId())}, nil
}

func terminalOutput(kind core.TerminalKind, id string, out core.Output) *qroutonv1.Event {
	output := &qroutonv1.TerminalOutput{
		TerminalId: id,
		Kind:       terminalKind(kind),
		Data:       out.Data,
		Replay:     out.Replay,
		First:      out.First,
		Offset:     out.Offset,
		Reset_:     out.Reset,
	}
	if out.ExitCode != nil {
		code := int32(*out.ExitCode)
		output.ExitCode = &code
	}
	return &qroutonv1.Event{Payload: &qroutonv1.Event_TerminalOutput{TerminalOutput: output}}
}

func terminalKind(kind core.TerminalKind) qroutonv1.TerminalKind {
	switch kind {
	case core.TerminalConversation:
		return qroutonv1.TerminalKind_TERMINAL_KIND_CONVERSATION
	case core.TerminalTab:
		return qroutonv1.TerminalKind_TERMINAL_KIND_TAB
	}
	return qroutonv1.TerminalKind_TERMINAL_KIND_UNSPECIFIED
}
