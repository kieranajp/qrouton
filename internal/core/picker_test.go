package core

import (
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// An escalation from a background session changes nothing on screen: the request
// waits on the session it names until the user arrives there.
func TestAQueuedPickerOnABackgroundSessionOpensNothing(t *testing.T) {
	reg, shown, background := pickerWorkbench(t)
	deadline := time.Now().Add(time.Minute)

	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: background, Deadline: deadline}); err != nil {
		t.Fatal(err)
	}
	if reg.Current().Root() != shown {
		t.Fatalf("queueing a picker moved the screen to %q", reg.Current().Root())
	}
	if reg.Current().PendingPicker() != nil {
		t.Fatal("a background session's escalation reached the session on screen")
	}
	if reg.BySlug("background").PendingPicker() == nil {
		t.Fatal("the escalation did not wait on the session it names")
	}
}

// Arrival is what draws it, and the chrome poll is how arrival reaches the page.
func TestAPendingPickerReachesTheChromeOfTheSessionItNames(t *testing.T) {
	reg, _, background := pickerWorkbench(t)
	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: background, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}

	if fields := chromeOf(t, reg); fields.Picker {
		t.Fatal("another session's escalation drew a picker over the one on screen")
	}
	reg.reveal(reg.BySlug("background"))
	if fields := chromeOf(t, reg); !fields.Picker {
		t.Fatal("arriving at the session did not draw its picker")
	}
}

// The workbench never learns that awaitEscalation gave up, so an expired request
// is ignored rather than drawn for an answer nobody is polling for.
func TestAnExpiredPickerIsIgnoredOnArrival(t *testing.T) {
	reg, _, background := pickerWorkbench(t)
	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: background, Deadline: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	reg.reveal(reg.BySlug("background"))
	if fields := chromeOf(t, reg); fields.Picker {
		t.Fatal("an escalation whose poll has timed out was still drawn")
	}
}

// Two escalations for one session is the second replacing the first; both
// pollers then read the one stanza the confirm writes.
func TestASecondPickerRequestReplacesTheFirst(t *testing.T) {
	reg, shown, _ := pickerWorkbench(t)
	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if reg.Current().PendingPicker() == nil {
		t.Fatal("a live request did not replace the expired one before it")
	}
}

func TestAPickerForASessionThisWorkbenchIsNotRunningIsRefused(t *testing.T) {
	reg, _, _ := pickerWorkbench(t)
	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: "/sessions/kraken", Deadline: time.Now().Add(time.Minute)}); err == nil {
		t.Fatal("a picker for an unknown session was queued")
	}
}

func TestOpPickerRefusesAnEmptyRootAsAnAnswer(t *testing.T) {
	reg, _, windows := testKernel(t, func(string, any) {})
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, reg.Current(),
		controlHooks{picker: func(workbench.PickerRequest) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	host := (workbench.Handle{Socket: socket, SessionRoot: "/sessions/x"}).WindowHost()
	if err := host.Picker(t.Context(), workbench.PickerRequest{Deadline: time.Now()}); err == nil {
		t.Fatal("a picker with no session root succeeded")
	} else if !strings.Contains(err.Error(), ErrNoSessionRoot.Error()) {
		t.Fatalf("picker refusal = %v", err)
	}
}

// pickerWorkbench is a registry holding two booted sessions, one on screen.
func pickerWorkbench(t *testing.T) (*Sessions, string, string) {
	t.Helper()
	root := t.TempDir()
	shown, background := sessionDir(t, root, "shown"), sessionDir(t, root, "background")
	reg := newSessions()
	t.Cleanup(reg.stopAll)
	reg.reveal(reg.add(shown, []string{"/bin/cat"}, nil))
	reg.add(background, []string{"/bin/cat"}, nil)
	return reg, shown, background
}

func chromeOf(t *testing.T, reg *Sessions) status.Fields {
	t.Helper()
	var fields status.Fields
	pushChrome(reg, "", nil, map[string][]status.RepoStat{}, nil, map[string]int{},
		func(event string, payload any) {
			if event == chromeEvent {
				fields = payload.(status.Fields)
			}
		})
	return fields
}
