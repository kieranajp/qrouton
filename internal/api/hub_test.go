package api

import (
	"context"
	"strconv"
	"strings"
	"testing"

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/status"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
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
		if grpcstatus.Code(err) != codes.ResourceExhausted {
			t.Fatalf("the stream ended with %v, want ResourceExhausted", err)
		}
		return
	}
}

// Catches an event holding invalid UTF-8 in any string, which proto refuses to
// marshal: it would end the stream, and the resume would hit it again.
func TestAnEventWithInvalidUTF8CrossesTheStream(t *testing.T) {
	p, hub := served(t)
	sub := subscribeAfter(t, p, 0)
	hub.Chrome(status.Fields{Documents: []status.Document{{Name: "bad\xffname"}}, Branch: "main\xfe"})
	hub.AssemblyRequested("after")
	chrome := sub.next().GetChrome()
	if chrome.GetBranch() != "main\uFFFD" || chrome.GetDocuments()[0].GetName() != "bad\uFFFDname" {
		t.Fatalf("the chrome arrived as %q and %q", chrome.GetBranch(), chrome.GetDocuments()[0].GetName())
	}
	if got := sub.next().GetAssemblyRequested().GetTicket(); got != "after" {
		t.Fatalf("the stream carried %q after it", got)
	}
}

// Catches a backlog bounded only by count, which a run of large documents
// would grow by every one of them.
func TestTheBacklogStaysWithinItsByteBudget(t *testing.T) {
	hub := NewHub()
	big := strings.Repeat("x", backlogBytes/4)
	for range 8 {
		hub.WindowContent("window-1", core.Document{Text: big})
	}
	if hub.held > backlogBytes || len(hub.backlog) > 4 {
		t.Fatalf("the backlog holds %d events and %d bytes", len(hub.backlog), hub.held)
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
	queue <- output("window-2", 6, "zz")
	queue <- output("window-1", 8, "gh")

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
	exited := make(chan *qroutonv1.Event, 2)
	exit := output("window-1", 2, "cd")
	code := int32(0)
	exit.GetTerminalOutput().ExitCode = &code
	exited <- exit
	if _, held := coalesce(output("window-1", 0, "ab"), exited); held == nil {
		t.Fatal("output carrying an exit code was merged")
	}
	full := make(chan *qroutonv1.Event, 2)
	full <- output("window-1", coalesceLimit, "y")
	if _, held := coalesce(output("window-1", 0, strings.Repeat("x", coalesceLimit)), full); held == nil {
		t.Fatal("a merge grew past the size cap")
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

// Catches a stream that does not say where it starts, which leaves a client
// whose first stream ends before any event resubscribing from now and losing
// what came between.
func TestSubscribeNamesTheSeqItResumesAfter(t *testing.T) {
	p, hub := served(t)
	watching := subscribeAfter(t, p, 0)
	hub.AssemblyRequested("two")
	var two uint64
	for two == 0 {
		if event := watching.next(); event.GetAssemblyRequested().GetTicket() == "two" {
			two = event.GetSeq()
		}
	}
	stream, err := qroutonv1.NewEventServiceClient(hello(t, p)).Subscribe(context.Background(), &qroutonv1.SubscribeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	header, err := stream.Header()
	if err != nil {
		t.Fatal(err)
	}
	got := header.Get(SeqHeader)
	if len(got) != 1 {
		t.Fatalf("the stream's header named seq %v", got)
	}
	if seq, err := strconv.ParseUint(got[0], 10, 64); err != nil || seq < two {
		t.Fatalf("the stream's header named seq %v, before the event at %d it cannot carry", got, two)
	}
}
