package workbench

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleRoundTripsAcrossExecBoundary(t *testing.T) {
	h := Handle{Socket: "/tmp/qrouton/501/abc123.sock", SessionRoot: "/sessions/my-session"}
	got, err := ParseHandle(h.Marshal())
	if err != nil {
		t.Fatal(err)
	}
	if got != h {
		t.Fatalf("round-trip = %#v, want %#v", got, h)
	}
}

func TestParseHandleRejectsGarbageAndMissingIdentity(t *testing.T) {
	if _, err := ParseHandle("not json"); err == nil {
		t.Fatal("accepted malformed handle")
	}
	if _, err := ParseHandle(`{"socket":"/tmp/s.sock"}`); !errors.Is(err, ErrHandleIncomplete) {
		t.Fatal("accepted handle without a session root")
	}
	if _, err := ParseHandle(`{"session_root":"/s"}`); !errors.Is(err, ErrHandleIncomplete) {
		t.Fatal("accepted handle without a socket")
	}
}

// macOS caps a unix socket path at 104 bytes, which is why the address is not
// derived from the session directory.
func TestNewSocketPathStaysWellInsideTheUnixPathLimit(t *testing.T) {
	path, err := NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if len(path) > 104 {
		t.Fatalf("socket path is %d bytes: %q", len(path), path)
	}
	if !strings.HasSuffix(path, socketSuffix) {
		t.Fatalf("socket path %q is not a socket", path)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal("socket directory was not created:", err)
	}
	if info.Mode().Perm() != socketDirMode {
		t.Fatalf("socket directory mode = %v, want %v", info.Mode().Perm(), os.FileMode(socketDirMode))
	}
	second, err := NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if second == path {
		t.Fatal("two desktop processes were handed the same address")
	}
}

// The client is the only encoder of the wire format, so its request shape and
// its reading of a reply are pinned against a listener rather than a mock.
func TestClientSendsOneRequestPerConnectionAndReadsItsAnswer(t *testing.T) {
	socket, requests := echoServer(t, func(req Request) Response {
		switch req.Op {
		case OpOpen:
			return Response{ID: "window-1"}
		case OpRead:
			return Response{Text: "listening on :3000"}
		case OpExists:
			return Response{Exists: true}
		case OpList:
			return Response{IDs: []string{"window-1", "window-2"}}
		}
		return Response{}
	})
	host := newClient(socket)
	ctx := context.Background()

	id, err := host.Open(ctx, WindowOptions{Kind: KindTerminal, Label: "dev", Command: []string{"npm", "run", "dev"}})
	if err != nil {
		t.Fatal(err)
	}
	if id != "window-1" {
		t.Fatalf("Open returned %q", id)
	}
	text, err := host.Read(ctx, id, true)
	if err != nil || text != "listening on :3000" {
		t.Fatalf("Read = %q, %v", text, err)
	}
	live, err := host.Exists(ctx, id)
	if err != nil || !live {
		t.Fatalf("Exists = %v, %v", live, err)
	}
	ids, err := host.List(ctx)
	if err != nil || len(ids) != 2 {
		t.Fatalf("List = %v, %v", ids, err)
	}
	if err := host.Adopt(ctx, "/sessions/octopus"); err != nil {
		t.Fatal(err)
	}
	if err := host.Close(ctx, id); err != nil {
		t.Fatal(err)
	}

	got := <-requests
	if got.Op != OpOpen || got.Options == nil || got.Options.Kind != KindTerminal || got.Options.Label != "dev" {
		t.Fatalf("open request = %+v", got)
	}
	for _, want := range []struct {
		op    string
		check func(Request) bool
	}{
		{OpRead, func(r Request) bool { return r.ID == "window-1" && r.Full }},
		{OpExists, func(r Request) bool { return r.ID == "window-1" }},
		{OpList, func(r Request) bool { return true }},
		{OpAdopt, func(r Request) bool { return r.Root == "/sessions/octopus" }},
		{OpClose, func(r Request) bool { return r.ID == "window-1" }},
	} {
		got := <-requests
		if got.Op != want.op || !want.check(got) {
			t.Fatalf("%s request = %+v", want.op, got)
		}
	}
}

// An answer carrying an error is the desktop process refusing, not the
// transport failing, so the caller must see the reason rather than a dial error.
func TestClientSurfacesTheAnswersError(t *testing.T) {
	socket, _ := echoServer(t, func(Request) Response {
		return Response{Error: "no window with id \"window-9\""}
	})
	_, err := newClient(socket).Read(context.Background(), "window-9", false)
	if err == nil || !strings.Contains(err.Error(), "window-9") {
		t.Fatalf("Read error = %v, want the desktop process's own reason", err)
	}
}

// A window id the desktop process answered without is an orphan the caller
// could never address again.
func TestClientRejectsAnOpenWithNoWindowID(t *testing.T) {
	socket, _ := echoServer(t, func(Request) Response { return Response{} })
	if _, err := newClient(socket).Open(context.Background(), WindowOptions{}); !errors.Is(err, ErrWindowIDUnavailable) {
		t.Fatalf("Open error = %v, want ErrWindowIDUnavailable", err)
	}
}

func TestClientReportsAnAbsentWorkbench(t *testing.T) {
	host := newClient("/tmp/qrouton/does-not-exist.sock")
	if _, err := host.List(context.Background()); !errors.Is(err, ErrWorkbenchUnreachable) {
		t.Fatalf("List error = %v, want ErrWorkbenchUnreachable", err)
	}
}

// echoServer answers each request with reply and records what it was asked.
func echoServer(t *testing.T, reply func(Request) Response) (socket string, requests chan Request) {
	t.Helper()
	socket, err := NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen(socketNetwork, socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(socket)
	})
	requests = make(chan Request, 16)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadBytes('\n')
				if err != nil {
					return
				}
				var req Request
				if err := json.Unmarshal(line, &req); err != nil {
					return
				}
				requests <- req
				answer, _ := json.Marshal(reply(req))
				_, _ = conn.Write(append(answer, '\n'))
			}()
		}
	}()
	return socket, requests
}
