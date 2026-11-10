package api

import (
	"strconv"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type eventService struct {
	qroutonv1.UnimplementedEventServiceServer
	hub *Hub
}

// Subscribe sends its header, naming the seq it resumes after, once the queue
// is registered, so a client that waits for the header misses nothing after it.
func (e *eventService) Subscribe(req *qroutonv1.SubscribeRequest, stream grpc.ServerStreamingServer[qroutonv1.SubscribeResponse]) error {
	sub, missed, resync, now := e.hub.subscribe(clientID(stream.Context()), req.GetAfterSeq())
	defer e.hub.unsubscribe(sub)
	if err := stream.SendHeader(metadata.Pairs(SeqHeader, strconv.FormatUint(now, 10))); err != nil {
		return err
	}
	if resync != nil {
		missed = []*qroutonv1.Event{resync}
	}
	for _, event := range missed {
		if err := stream.Send(&qroutonv1.SubscribeResponse{Event: event}); err != nil {
			return err
		}
	}
	var held *qroutonv1.Event
	for {
		event := held
		if event == nil {
			select {
			case event = <-sub.queue:
			case <-sub.overflow:
				return status.Error(codes.ResourceExhausted, ErrSubscriberBehind.Error())
			case <-stream.Context().Done():
				return nil
			}
		}
		event, held = coalesce(event, sub.queue)
		if err := stream.Send(&qroutonv1.SubscribeResponse{Event: event}); err != nil {
			return err
		}
	}
}

// coalesce merges the live output already queued behind first for the same
// terminal, up to the size cap, and hands back the event that stopped it.
// Events are shared between subscribers, so a merge builds a new one.
func coalesce(first *qroutonv1.Event, queue <-chan *qroutonv1.Event) (event, held *qroutonv1.Event) {
	event = first
	var merged *qroutonv1.TerminalOutput
	for mergeable(event.GetTerminalOutput()) {
		var next *qroutonv1.Event
		select {
		case next = <-queue:
		default:
			return event, nil
		}
		a, b := event.GetTerminalOutput(), next.GetTerminalOutput()
		if !mergeable(b) || b.GetTerminalId() != a.GetTerminalId() || b.GetOffset() != a.GetOffset()+uint64(len(a.GetData())) ||
			len(a.GetData())+len(b.GetData()) > coalesceLimit {
			return event, next
		}
		if merged == nil {
			merged = proto.Clone(a).(*qroutonv1.TerminalOutput)
		}
		merged.Data = append(merged.Data, b.GetData()...)
		event = &qroutonv1.Event{Seq: next.GetSeq(), Payload: &qroutonv1.Event_TerminalOutput{TerminalOutput: merged}}
	}
	return event, nil
}

// mergeable is live output: a replay or an exit marks a boundary the page draws.
func mergeable(out *qroutonv1.TerminalOutput) bool {
	return out != nil && !out.GetReplay() && !out.GetReset_() && out.ExitCode == nil
}
