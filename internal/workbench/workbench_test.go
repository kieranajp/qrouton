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
	"time"
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

func TestAnsweredSeesOnlyALiveSocket(t *testing.T) {
	socket, _ := echoServer(t, func(Request) Response { return Response{} })
	if !Answered(socket) {
		t.Fatalf("Answered(%q) = false while a listener is serving it", socket)
	}
	absent := socket + socketSuffix
	if Answered(absent) {
		t.Fatalf("Answered(%q) = true with nothing listening", absent)
	}
}

func TestRunningFindsALiveSocket(t *testing.T) {
	dir := t.TempDir()
	listenAt(t, filepath.Join(dir, "live"+socketSuffix))
	if !running(dir) {
		t.Fatal("running = false with a workbench serving its socket, so a second one would launch over it")
	}
}

// A stale socket file left there makes every later launch think a workbench is
// already up; a process log beside it is the only record of why one died.
func TestRunningSweepsStaleSockets(t *testing.T) {
	dir := t.TempDir()
	if running(filepath.Join(dir, "absent")) {
		t.Fatal("running = true for a directory that does not exist")
	}
	dead := filepath.Join(dir, "dead"+socketSuffix)
	log := ProcessLog(dead)
	if err := os.WriteFile(dead, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if running(dir) {
		t.Fatal("running = true with only a socket nothing answers on")
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatalf("stale socket survived: %v", err)
	}
	if _, err := os.Stat(log); err != nil {
		t.Fatalf("process log was swept along with the stale socket: %v", err)
	}
}

// listenAt serves a socket at path until the test ends.
func listenAt(t *testing.T, path string) {
	t.Helper()
	listener, err := net.Listen(socketNetwork, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
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

// The deadline travels with the request: the workbench never learns that the
// escalating agent gave up, so a request without it would draw a picker whose
// answer nothing is polling for.
func TestPickerCarriesItsSessionAndDeadlineOverTheSocket(t *testing.T) {
	socket, requests := echoServer(t, func(Request) Response { return Response{} })
	deadline := time.Now().Add(30 * time.Minute).Round(time.Millisecond)
	req := PickerRequest{SessionRoot: "/sessions/octopus", Name: "Webhook retry",
		Prefix: "fix", Deadline: deadline}
	if err := newClient(socket).Picker(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := <-requests
	if got.Op != OpPicker || got.Root != "/sessions/octopus" || got.Picker == nil {
		t.Fatalf("picker request = %+v", got)
	}
	// A round-tripped time answers to a different location than the one that
	// made it, so the instant is what carries, not the struct.
	if got.Picker.SessionRoot != req.SessionRoot || got.Picker.Name != req.Name ||
		got.Picker.Prefix != req.Prefix || !got.Picker.Deadline.Equal(req.Deadline) {
		t.Fatalf("picker request = %+v, want %+v", *got.Picker, req)
	}
}

// A refusal is the workbench answering, not the transport failing.
func TestPickerSurfacesARefusalRatherThanADialError(t *testing.T) {
	socket, _ := echoServer(t, func(Request) Response {
		return Response{Error: "picker request carries no session root"}
	})
	err := newClient(socket).Picker(context.Background(), PickerRequest{Deadline: time.Now()})
	if err == nil {
		t.Fatal("a refused picker request succeeded")
	}
	if errors.Is(err, ErrWorkbenchUnreachable) {
		t.Fatalf("a refusal reported as a transport failure: %v", err)
	}
}
