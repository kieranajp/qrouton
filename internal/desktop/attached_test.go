package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/api"
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stall stops the page taking events, as a page too busy to keep up does, so
// the translator stops reading its stream. The returned func lets it go.
func stall(t *testing.T, r *fakeRenderer) func() {
	t.Helper()
	r.mu.Lock()
	release := sync.OnceFunc(r.mu.Unlock)
	t.Cleanup(release)
	return release
}

// overflow ends a stalled translator's stream by filling its queue.
func overflow(a *adapterWorkbench) {
	for range 8 * 4096 {
		a.opts.hub.AssemblyRequested("flood")
	}
}

// painted waits for the page's latest data event under name to hold want, as a
// replay when replay is set, and returns it.
func painted(t *testing.T, a *adapterWorkbench, name, want string, replay bool) map[string]any {
	t.Helper()
	var got map[string]any
	waitFor(t, name+" to paint "+want, func() bool {
		got = nil
		_ = json.Unmarshal(emitted(t, a.r, name), &got)
		data, _ := base64.StdEncoding.DecodeString(fmt.Sprint(got["encoded"]))
		return got["replay"] == replay && strings.Contains(string(data), want)
	})
	return got
}

// held waits until the core's own copy of a tab holds want.
func held(t *testing.T, a *adapterWorkbench, id, want string) {
	t.Helper()
	waitFor(t, id+" to hold "+want, func() bool {
		read, err := a.host.Read(context.Background(), id, true)
		return err == nil && strings.Contains(read.Text, want)
	})
}

// Catches output lost in a stream break: a tab flooding a stalled page breaks
// its stream, and what it wrote after that must still reach the page, followed
// by live output. The hub flood makes the break certain where the transport
// would absorb the tab's.
func TestAFloodedTabReplaysItsRingAfterTheResubscribe(t *testing.T) {
	a := newAdapterWorkbench(t)
	windows := &Windows{windows: a.kernel.Windows, terminals: a.in.terminals}
	id := a.shell(t, `read go; seq 1 200000; read go; echo flood-done; exec cat`)
	if err := windows.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	release := stall(t, a.r)
	if err := windows.Write(id, encoded("go\n")); err != nil {
		t.Fatal(err)
	}
	held(t, a, id, "200000")
	overflow(a)
	if err := windows.Write(id, encoded("go\n")); err != nil {
		t.Fatal(err)
	}
	held(t, a, id, "flood-done")
	release()

	painted(t, a, windowDataEvent+id, "flood-done", true)
	if err := windows.Write(id, encoded("live-after\n")); err != nil {
		t.Fatal(err)
	}
	painted(t, a, windowDataEvent+id, "live-after", false)
}

// Catches a tab stuck open: its exit during a stream break must still end it
// on the page once the stream is back.
func TestATabThatExitsInAStreamBreakEndsAfterTheResubscribe(t *testing.T) {
	a := newAdapterWorkbench(t)
	windows := &Windows{windows: a.kernel.Windows, terminals: a.in.terminals}
	id := a.shell(t, `read go; echo exiting; exit 3`)
	if err := windows.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	release := stall(t, a.r)
	overflow(a)
	if err := windows.Write(id, encoded("go\n")); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the tab to exit", func() bool {
		read, err := a.host.Read(context.Background(), id, false)
		return err == nil && read.Terminal != nil && read.Terminal.ExitCode != nil
	})
	release()

	if got := painted(t, a, windowDataEvent+id, "exiting", true); got["exit_code"] != float64(3) {
		t.Fatalf("the replay carried exit_code %v, want 3", got["exit_code"])
	}
}

// Catches a silent drop: a replay to a client with no live stream reaches
// nobody, so the attach must say so rather than succeed.
func TestAttachWithNoLiveSubscriberFailsFailedPrecondition(t *testing.T) {
	a := newAdapterWorkbench(t)
	id := a.conversation(t)
	if err := (&Term{terminals: a.in.terminals}).Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	resp, err := qroutonv1.NewWorkbenchServiceClient(a.in.Conn).Hello(context.Background(),
		&qroutonv1.HelloRequest{Protocol: api.ProtocolVersion, ClientName: "unsubscribed"})
	if err != nil {
		t.Fatal(err)
	}
	unsubscribed := qroutonv1.NewTerminalServiceClient(api.Identified(a.in.Conn, resp.GetClientId()))
	_, err = unsubscribed.Attach(context.Background(), &qroutonv1.AttachRequest{TerminalId: id})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("an attach with no live stream = %v, want FailedPrecondition", err)
	}
}

// Catches a page mount during a stream break that fails, or that never
// paints: the mount must succeed and the terminal paint once the stream is back.
func TestATermStartInAStreamBreakPaintsAfterTheResubscribe(t *testing.T) {
	a := newAdapterWorkbench(t, "/bin/sh", "-c", `printf 'scrollback-marker\n'; exec cat`)
	id := a.conversation(t)
	term := &Term{terminals: a.in.terminals}
	if err := term.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	painted(t, a, ptyDataEvent+id, "scrollback-marker", false)
	release := stall(t, a.r)
	overflow(a)
	if err := term.Start(id, 100, 30); err != nil {
		t.Fatalf("a Start during a stream break = %v, want nil", err)
	}
	release()

	painted(t, a, ptyDataEvent+id, "scrollback-marker", true)
}

// Catches a re-attach that resizes: after a stream break, a terminal resized
// by another client since the page mounted it keeps that size.
func TestAReattachNeverResizesARunningTerminal(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "size")
	a := newAdapterWorkbench(t, "/bin/sh", "-c", `printf 'resize-marker\n'; `+sizeToMarker, "sh", marker)
	id := a.conversation(t)
	term := &Term{terminals: a.in.terminals}
	if err := term.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := a.kernel.Term.Resize(id, 132, 50); err != nil {
		t.Fatal(err)
	}
	markedFile(t, marker, "50 132")
	release := stall(t, a.r)
	overflow(a)
	release()

	painted(t, a, ptyDataEvent+id, "resize-marker", true)
	time.Sleep(200 * time.Millisecond)
	markedFile(t, marker, "50 132")
}

// Catches a re-attach that undoes the page: a terminal mounted and then resized
// during a stream break keeps the later size.
func TestAReattachKeepsAResizeFromTheStreamBreak(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "size")
	a := newAdapterWorkbench(t, "/bin/sh", "-c", `printf 'resize-marker\n'; `+sizeToMarker, "sh", marker)
	id := a.conversation(t)
	term := &Term{terminals: a.in.terminals}
	if err := term.Start(id, 80, 24); err != nil {
		t.Fatal(err)
	}
	release := stall(t, a.r)
	overflow(a)
	if err := term.Start(id, 100, 30); err != nil {
		t.Fatal(err)
	}
	if err := term.Resize(id, 132, 50); err != nil {
		t.Fatal(err)
	}
	release()

	painted(t, a, ptyDataEvent+id, "resize-marker", true)
	time.Sleep(200 * time.Millisecond)
	markedFile(t, marker, "50 132")
}

// firstMount waits for the page's replay of an agent tab the page mounts after
// a stream break, and reports whether it came marked first.
func firstMount(t *testing.T, a *adapterWorkbench, id string) bool {
	t.Helper()
	return painted(t, a, windowDataEvent+id, "agent-tab", true)["first"] == true
}

// Catches a re-attach that consumes an unmounted tab's first mount: the page
// would then swallow the terminal queries in that tab's replay, and a program
// waiting on a reply would hang.
func TestAReattachLeavesATabThePageNeverMountedFirst(t *testing.T) {
	a := newAdapterWorkbench(t, "/bin/sh", "-c", `printf 'reattached\n'; exec cat`)
	windows := &Windows{windows: a.kernel.Windows, terminals: a.in.terminals}
	conversation := a.conversation(t)
	if err := (&Term{terminals: a.in.terminals}).Start(conversation, 80, 24); err != nil {
		t.Fatal(err)
	}
	painted(t, a, ptyDataEvent+conversation, "reattached", false)
	tab := a.terminal(t, `printf 'agent-tab\n'; exec cat`)
	held(t, a, tab, "agent-tab")
	release := stall(t, a.r)
	overflow(a)
	release()
	painted(t, a, ptyDataEvent+conversation, "reattached", true)

	if err := windows.Start(tab, 80, 24); err != nil {
		t.Fatal(err)
	}
	if !firstMount(t, a, tab) {
		t.Fatal("the page's first mount of the tab replayed without first")
	}
}

// Catches a replay that reached no stream counting as the tab's first mount:
// the mount during the break must still paint as first once the stream is back.
func TestAMountInAStreamBreakStaysFirstAfterTheResubscribe(t *testing.T) {
	a := newAdapterWorkbench(t)
	windows := &Windows{windows: a.kernel.Windows, terminals: a.in.terminals}
	tab := a.terminal(t, `printf 'agent-tab\n'; exec cat`)
	held(t, a, tab, "agent-tab")
	release := stall(t, a.r)
	overflow(a)
	if err := windows.Start(tab, 80, 24); err != nil {
		t.Fatal(err)
	}
	release()

	if !firstMount(t, a, tab) {
		t.Fatal("the re-attach replayed the tab without first")
	}
}

type listedTerminals struct {
	qroutonv1.WindowServiceClient
	terminals []*qroutonv1.SessionTerminal
}

func (l listedTerminals) ListTerminals(context.Context, *qroutonv1.ListTerminalsRequest, ...grpc.CallOption) (*qroutonv1.ListTerminalsResponse, error) {
	return &qroutonv1.ListTerminalsResponse{Terminals: l.terminals}, nil
}

type goneTerminals struct {
	qroutonv1.TerminalServiceClient
}

func (goneTerminals) Attach(context.Context, *qroutonv1.AttachRequest, ...grpc.CallOption) (*qroutonv1.AttachResponse, error) {
	return nil, status.Error(codes.NotFound, "gone")
}

// Catches a terminal that ended between the listing and its re-attach staying
// open on the page.
func TestAReattachThatFindsNoTerminalEndsIt(t *testing.T) {
	size := &qroutonv1.TerminalSize{Cols: 80, Rows: 24}
	var mu sync.Mutex
	got := map[string]any{}
	tr := &translator{
		windows: listedTerminals{terminals: []*qroutonv1.SessionTerminal{
			{TerminalId: "term-1", Kind: qroutonv1.TerminalKind_TERMINAL_KIND_CONVERSATION, Size: size},
			{TerminalId: "window-2", Kind: qroutonv1.TerminalKind_TERMINAL_KIND_TAB, Size: size},
		}},
		terminals: newTerminals(goneTerminals{}),
		emit: func(event string, payload any) {
			mu.Lock()
			defer mu.Unlock()
			got[event] = payload
		},
	}
	tr.terminals.mount("term-1")
	tr.terminals.mount("window-2")
	tr.reattach(context.Background())
	for _, name := range []string{ptyExitEvent + "term-1", windowExitEvent + "window-2"} {
		if got[name] != -1 {
			t.Fatalf("%s = %v, want -1 in %v", name, got[name], got)
		}
	}
}
