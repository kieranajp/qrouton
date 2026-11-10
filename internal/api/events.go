package api

import (
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type eventService struct {
	qroutonv1.UnimplementedEventServiceServer
	hub *Hub
}

// Subscribe sends its header once the queue is registered, so a client that
// waits for the header misses nothing published after.
func (e *eventService) Subscribe(req *qroutonv1.SubscribeRequest, stream grpc.ServerStreamingServer[qroutonv1.SubscribeResponse]) error {
	sub, missed, resync := e.hub.subscribe(req.GetAfterSeq())
	defer e.hub.unsubscribe(sub)
	if err := stream.SendHeader(metadata.MD{}); err != nil {
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
	for {
		select {
		case event := <-sub.queue:
			if err := stream.Send(&qroutonv1.SubscribeResponse{Event: event}); err != nil {
				return err
			}
		case <-sub.overflow:
			return status.Error(codes.ResourceExhausted, ErrSubscriberBehind.Error())
		case <-stream.Context().Done():
			return nil
		}
	}
}
