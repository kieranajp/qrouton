package api

import (
	"context"
	"testing"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type subscription struct {
	t      *testing.T
	stream interface {
		Recv() (*qroutonv1.SubscribeResponse, error)
	}
}

func subscribeAfter(t *testing.T, p *InProcess, after uint64) subscription {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stream, err := qroutonv1.NewEventServiceClient(hello(t, p)).Subscribe(ctx, &qroutonv1.SubscribeRequest{AfterSeq: after})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Header(); err != nil {
		t.Fatal(err)
	}
	return subscription{t: t, stream: stream}
}

func (s subscription) next() *qroutonv1.Event {
	s.t.Helper()
	resp, err := s.stream.Recv()
	if err != nil {
		s.t.Fatal(err)
	}
	return resp.GetEvent()
}

func served(t *testing.T) (*InProcess, *Hub) {
	t.Helper()
	p := serve(t, newFakeOllama())
	return p, p.hub
}

// Catches a hub that reorders events or numbers them out of sequence, which
// would let a later chrome state be overwritten by an earlier one.
func TestTheHubDeliversEventsInOrderWithRisingSeq(t *testing.T) {
	p, hub := served(t)
	sub := subscribeAfter(t, p, 0)
	for scale := 80; scale <= 120; scale += 10 {
		hub.UIScale(scale)
	}
	var last uint64
	for want := 80; want <= 120; want += 10 {
		event := sub.next()
		if got := int(event.GetUiScale().GetScale()); got != want || event.GetSeq() <= last {
			t.Fatalf("event %d carried scale %d after seq %d, want scale %d", event.GetSeq(), got, last, want)
		}
		last = event.GetSeq()
	}
}

// Catches a resubscribe that loses or repeats events still in the backlog,
// and one that replays terminal output, which resumes by offset instead.
func TestAResubscribeResumesFromTheBacklog(t *testing.T) {
	p, hub := served(t)
	first := subscribeAfter(t, p, 0)
	hub.AssemblyRequested("one")
	seen := first.next().GetSeq()
	hub.AssemblyRequested("two")
	hub.TerminalOutput(core.TerminalTab, "window-1", core.Output{Data: []byte("x")})
	hub.AssemblyRequested("three")

	resumed := subscribeAfter(t, p, seen)
	for _, want := range []string{"two", "three"} {
		if got := resumed.next().GetAssemblyRequested().GetTicket(); got != want {
			t.Fatalf("resumed with %q, want %q", got, want)
		}
	}
	hub.AssemblyRequested("four")
	if got := resumed.next().GetAssemblyRequested().GetTicket(); got != "four" {
		t.Fatalf("after the backlog came %q, want the live event", got)
	}
}

// Catches a resume from past the backlog that carries on as if nothing were
// lost, leaving the client with a stale picture and no reason to snapshot.
func TestAResubscribePastTheBacklogResyncsFirst(t *testing.T) {
	p, hub := served(t)
	for range backlogSize + 2 {
		hub.AssemblyRequested("old")
	}
	sub := subscribeAfter(t, p, 1)
	if sub.next().GetResync() == nil {
		t.Fatal("a resume from an evicted seq did not start with Resync")
	}
	hub.AssemblyRequested("live")
	if got := sub.next().GetAssemblyRequested().GetTicket(); got != "live" {
		t.Fatalf("after Resync came %q, want the live event", got)
	}
}

// Catches a subscriber that falls behind and either blocks the core or
// silently loses events instead of being told to resubscribe.
func TestAnOverflowingSubscriberEndsWithResourceExhausted(t *testing.T) {
	p, hub := served(t)
	sub := subscribeAfter(t, p, 0)
	// The transport buffers some of the flood before the queue starts filling.
	for range 8 * queueSize {
		hub.AssemblyRequested("flood")
	}
	for {
		_, err := sub.stream.Recv()
		if err == nil {
			continue
		}
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("the stream ended with %v, want ResourceExhausted", err)
		}
		return
	}
}

func output(id string, offset uint64, data string) *qroutonv1.Event {
	return terminalOutput(core.TerminalTab, id, core.Output{Data: []byte(data), Offset: offset})
}

// Catches a sender that merges two terminals' output, merges across a gap or a
// replay, or changes the events other subscribers share.
func TestCoalesceMergesQueuedOutputForOneTerminalOnly(t *testing.T) {
	first := output("window-1", 0, "ab")
	queue := make(chan *qroutonv1.Event, 8)
	queue <- output("window-1", 2, "cd")
	queue <- output("window-1", 4, "ef")
	queue <- output("window-2", 0, "zz")
	queue <- output("window-1", 6, "gh")

	merged, held := coalesce(first, queue)
	if got := string(merged.GetTerminalOutput().GetData()); got != "abcdef" || merged.GetTerminalOutput().GetOffset() != 0 {
		t.Fatalf("merged %q at %d, want abcdef at 0", got, merged.GetTerminalOutput().GetOffset())
	}
	if held.GetTerminalOutput().GetTerminalId() != "window-2" {
		t.Fatalf("held %v, want the other terminal's output", held)
	}
	if string(first.GetTerminalOutput().GetData()) != "ab" {
		t.Fatal("the merge changed an event other subscribers share")
	}
	again, _ := coalesce(held, queue)
	if string(again.GetTerminalOutput().GetData()) != "zz" {
		t.Fatalf("window-2 merged into %q", again.GetTerminalOutput().GetData())
	}

	gap := make(chan *qroutonv1.Event, 2)
	gap <- output("window-1", 9, "late")
	if _, held := coalesce(output("window-1", 0, "ab"), gap); held == nil {
		t.Fatal("output across a gap in offsets was merged")
	}
	replay := make(chan *qroutonv1.Event, 2)
	replayed := output("window-1", 2, "cd")
	replayed.GetTerminalOutput().Replay = true
	replay <- replayed
	if _, held := coalesce(output("window-1", 0, "ab"), replay); held == nil {
		t.Fatal("a replay was merged into live output")
	}
}

// Catches a replay broadcast to every client, which would repaint a terminal a
// second client already shows.
func TestAReplayReachesOnlyTheClientThatAttached(t *testing.T) {
	p, hub := served(t)
	attaching := hello(t, p)
	other := subscribeAfter(t, p, 0)
	ctx := context.Background()
	stream, err := qroutonv1.NewEventServiceClient(attaching).Subscribe(ctx, &qroutonv1.SubscribeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Header(); err != nil {
		t.Fatal(err)
	}
	id := clientOf(t, attaching)
	hub.deliver(id, output("window-1", 0, "replayed"))
	hub.AssemblyRequested("after")
	resp, err := stream.Recv()
	if err != nil || string(resp.GetEvent().GetTerminalOutput().GetData()) != "replayed" {
		t.Fatalf("the attaching client got %v, %v", resp, err)
	}
	if got := other.next(); got.GetAssemblyRequested().GetTicket() != "after" {
		t.Fatalf("another client got %v before the broadcast", got)
	}
}
