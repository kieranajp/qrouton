package desktop

import (
	"context"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func TestActivityRanksTheHookOverTheClock(t *testing.T) {
	a := &activity{}
	if got := a.state(); got != status.ActivityIdle {
		t.Fatalf("a terminal that has never spoken = %q, want idle", got)
	}

	a.wrote()
	if got := a.state(); got != status.ActivityWorking {
		t.Fatalf("fresh output = %q, want working", got)
	}

	// An agent drawing a permission prompt is producing output, so waiting has
	// to beat working rather than merely arrive after it.
	a.hook(status.ActivityWaiting)
	a.wrote()
	if got := a.state(); got != status.ActivityWaiting {
		t.Fatalf("hook says waiting but state = %q", got)
	}

	a.answered()
	if got := a.state(); got != status.ActivityWorking {
		t.Fatalf("after typing = %q, want working", got)
	}

	a.hook(status.ActivityWorking)
	if got := a.state(); got != status.ActivityWorking {
		t.Fatalf("a subagent starting = %q, want working", got)
	}

	a.spoke = time.Now().Add(-activityQuiet - time.Second)
	if got := a.state(); got != status.ActivityIdle {
		t.Fatalf("after silence = %q, want idle", got)
	}
}

func TestTheControlSocketCarriesTheAttentionSignal(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	a := &activity{}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{attention: a.hook})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	handle := workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}
	if err := handle.Attention(context.Background(), status.ActivityWaiting); err != nil {
		t.Fatal(err)
	}
	if got := a.state(); got != status.ActivityWaiting {
		t.Fatalf("state after the hook = %q, want waiting", got)
	}
}
