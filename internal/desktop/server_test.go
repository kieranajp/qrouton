package desktop

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// adoption is what the adopt hook was told: the session chosen, and whether the
// workbench is the one to boot its agent.
type adoption struct {
	root string
	boot bool
}

// The control socket is the one place the port's wire format is agreed, and the
// two halves are compiled separately — so this drives the real server through
// the real client rather than either side's idea of the other.
func TestTheControlSocketServesTheWorkbenchPort(t *testing.T) {
	windows, r := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	queued := make(chan workbench.PickerRequest, 1)
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{
		picker: func(req workbench.PickerRequest) error { queued <- req; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	host, err := (workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	id, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec := <-r.opened; spec.Name != id {
		t.Fatalf("the server opened %q for id %q", spec.Name, id)
	}

	document, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ diff", Content: "one change",
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened

	if live, err := host.Exists(ctx, id); err != nil || !live {
		t.Fatalf("Exists = %v, %v for a window that is open", live, err)
	}
	ids, err := host.List(ctx)
	if err != nil || len(ids) != 2 {
		t.Fatalf("List = %v, %v, want both windows", ids, err)
	}
	text, err := host.Read(ctx, document, false)
	if err != nil || text != "one change" {
		t.Fatalf("Read = %q, %v", text, err)
	}
	deadline := time.Now().Add(time.Minute)
	if err := host.Picker(ctx, workbench.PickerRequest{SessionRoot: "/sessions/octopus",
		Name: "Webhook retry", Prefix: "fix", Deadline: deadline}); err != nil {
		t.Fatal(err)
	}
	if got := <-queued; got.SessionRoot != "/sessions/octopus" || got.Name != "Webhook retry" ||
		got.Prefix != "fix" || !got.Deadline.Equal(deadline) {
		t.Fatalf("queued %+v, want the request the caller sent", got)
	}

	if err := host.Close(ctx, id); err != nil {
		t.Fatal(err)
	}
	if live, err := host.Exists(ctx, id); err != nil || live {
		t.Fatalf("Exists = %v, %v after Close", live, err)
	}
	if !r.wasClosed(id) {
		t.Fatal("the closed window never left the screen")
	}
}

// A refusal is the desktop process's answer, not a transport failure, so the
// caller must read the reason rather than a dial error.
func TestTheControlSocketAnswersBadRequestsWithTheirReason(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	host, _ := (workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}).WindowHost()
	ctx := context.Background()

	if _, err := host.Read(ctx, "window-99", false); err == nil {
		t.Fatal("read of an unknown window succeeded")
	}
	if err := host.Close(ctx, "window-99"); err == nil {
		t.Fatal("close of an unknown window succeeded")
	}
	if err := host.Picker(ctx, workbench.PickerRequest{}); err == nil {
		t.Fatal("a picker request with no session root succeeded")
	}
	if _, err := host.Open(ctx, workbench.WindowOptions{Kind: workbench.KindTerminal, Label: "x"}); err == nil {
		t.Fatal("a terminal window opened with no command")
	}
}

// A process that died without unlinking its socket would otherwise leave the
// next run unable to bind.
func TestServeControlReplacesAStaleSocket(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the socket outlived the process that served it")
	}
}
