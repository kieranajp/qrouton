package launch

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// swapRunAgent replaces the exec seam with a scripted closure and restores it.
// It also stubs out the quick-reference panel, which would otherwise shell out
// to a zellij session no test has: the panel's own test swaps it deliberately.
func swapRunAgent(t *testing.T, fake func(argv, env []string, dir string, relaunch <-chan os.Signal) (bool, error)) {
	t.Helper()
	original := runAgent
	runAgent = fake
	t.Cleanup(func() { runAgent = original })
	swapShowHelp(t, func(string, mux.Handle, string) {})
}

// swapShowHelp replaces the quick-reference panel's spawn and restores it.
func swapShowHelp(t *testing.T, fake func(dir string, h mux.Handle, warning string)) {
	t.Helper()
	original := showHelp
	showHelp = fake
	t.Cleanup(func() { showHelp = original })
}

func testRunner() Runner {
	return Runner{ID: "claude", Label: "Claude Code", Command: []string{"claude", "--dangerously-skip-permissions"}}
}

func superviseTestDir(t *testing.T, mode session.SessionMode) string {
	t.Helper()
	dir := t.TempDir()
	if err := session.WriteManifest(dir, session.Manifest{Slug: "s", Mode: mode}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSuperviseRelaunchesFreshAfterEscalation(t *testing.T) {
	dir := superviseTestDir(t, session.ModeAssistant)
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		if len(argvs) == 1 {
			// The picker escalates while the assistant runs.
			if err := session.SetMode(dir, session.ModeRPI); err != nil {
				t.Fatal(err)
			}
			return true, nil
		}
		return false, nil
	})

	if err := Supervise(dir, testRunner(), mux.Handle{Kind: "zellij", Session: "s"}, EditorCommand{Argv: []string{"vi"}}, false); err != nil {
		t.Fatal(err)
	}
	if len(argvs) != 2 {
		t.Fatalf("expected the runner relaunched once, got %d launches", len(argvs))
	}
	// Escalation is a handoff: the second launch is fresh, not a continuation.
	for _, arg := range argvs[1] {
		if arg == claudeContinueFlag {
			t.Fatal("escalated relaunch carries --continue; the handoff must start a fresh context")
		}
	}
	// The relaunch stamped from the rewritten manifest.
	assertLinkTargetContains(t, dir+"/CLAUDE.md", "ORCHESTRATOR.md")
}

func TestSuperviseRelaunchesWithContinueAfterDeescalation(t *testing.T) {
	dir := superviseTestDir(t, session.ModeRPI)
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		if len(argvs) == 1 {
			if err := session.SetMode(dir, session.ModeAssistant); err != nil {
				t.Fatal(err)
			}
			return true, nil
		}
		return false, nil
	})

	if err := Supervise(dir, testRunner(), mux.Handle{Kind: "zellij", Session: "s"}, EditorCommand{Argv: []string{"vi"}}, false); err != nil {
		t.Fatal(err)
	}
	if len(argvs) != 2 {
		t.Fatalf("expected the runner relaunched once, got %d launches", len(argvs))
	}
	// De-escalation keeps the conversation.
	if !strings.Contains(strings.Join(argvs[1], " "), claudeContinueFlag) {
		t.Fatalf("de-escalated relaunch lacks %s; the conversation must survive: %v", claudeContinueFlag, argvs[1])
	}
	assertLinkTargetContains(t, dir+"/CLAUDE.md", "ASSISTANT.md")
}

// A second trip through the picker on a session already in RPI adds
// repositories; it is not a handoff, so the conversation must survive it.
// Reading the mode alone cannot tell the two apart — only the change can.
func TestSuperviseKeepsConversationWhenModeIsUnchanged(t *testing.T) {
	dir := superviseTestDir(t, session.ModeRPI)
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		if len(argvs) == 1 {
			// The picker's confirm rewrites the manifest with the same mode.
			if err := session.SetMode(dir, session.ModeRPI); err != nil {
				t.Fatal(err)
			}
			return true, nil
		}
		return false, nil
	})

	if err := Supervise(dir, testRunner(), mux.Handle{Kind: "zellij", Session: "s"}, EditorCommand{Argv: []string{"vi"}}, true); err != nil {
		t.Fatal(err)
	}
	if len(argvs) != 2 {
		t.Fatalf("expected the runner relaunched once, got %d launches", len(argvs))
	}
	if !strings.Contains(strings.Join(argvs[1], " "), claudeContinueFlag) {
		t.Fatalf("adding repos within RPI dropped %s and discarded the conversation: %v", claudeContinueFlag, argvs[1])
	}
}

// The escalation can land while no supervisor is watching the transition — a
// workspace restart between the picker's confirm and the next launch, or a
// signal that never arrived. The launcher then passes resume, the manifest
// already reads "rpi", and there is no change left to observe: the handoff used
// to silently resume the assistant's conversation into the fresh orchestrator.
func TestSuperviseStartsFreshWhenEscalationPrecedesTheLaunch(t *testing.T) {
	dir := superviseTestDir(t, session.ModeAssistant)
	if err := session.SetMode(dir, session.ModeRPI); err != nil {
		t.Fatal(err)
	}
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		return false, nil
	})

	// resume: true, as the launcher's resume path passes on a restart.
	if err := Supervise(dir, testRunner(), mux.Handle{Kind: "zellij", Session: "s"}, EditorCommand{Argv: []string{"vi"}}, true); err != nil {
		t.Fatal(err)
	}
	if len(argvs) != 1 {
		t.Fatalf("expected one launch, got %d", len(argvs))
	}
	for _, arg := range argvs[0] {
		if arg == claudeContinueFlag {
			t.Fatalf("handoff resumed the assistant's conversation: %v", argvs[0])
		}
	}
	if _, err := os.Stat(sessionpaths.HandoffPending(dir)); !os.IsNotExist(err) {
		t.Fatal("handoff marker outlived the launch that used it; a later restart would clear the context again")
	}
}

// The panel greets the session, not every runner the supervisor launches: an
// escalation relaunch must not float it over the fresh orchestrator.
func TestSuperviseFloatsTheHelpPanelOnceAtStartup(t *testing.T) {
	dir := superviseTestDir(t, session.ModeAssistant)
	var argvs int
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs++
		if argvs == 1 {
			if err := session.SetMode(dir, session.ModeRPI); err != nil {
				t.Fatal(err)
			}
			return true, nil
		}
		return false, nil
	})
	shown := make(chan string, 4)
	swapShowHelp(t, func(d string, h mux.Handle, warning string) { shown <- d + "|" + warning })

	if err := Supervise(dir, testRunner(), mux.Handle{Kind: "zellij", Session: "s"}, EditorCommand{Argv: []string{"vi"}}, false); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-shown:
		// Claude, not Codex: no depth warning to pass along as help.sh's $1.
		if got != dir+"|" {
			t.Fatalf("panel spawned with %q, want the session root and no warning", got)
		}
	case <-time.After(time.Second):
		t.Fatal("the session came up without its quick-reference panel")
	}
	select {
	case got := <-shown:
		t.Fatalf("panel floated a second time (%q); it greets the session, not each relaunch", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSuperviseLeavesThePaneOnUnsignalledExit(t *testing.T) {
	dir := superviseTestDir(t, session.ModeAssistant)
	calls := 0
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		calls++
		// The pid file exists while the supervisor runs, for the signalling side.
		if _, err := os.Stat(sessionpaths.AgentPID(dir)); err != nil {
			t.Fatalf("agent pid file missing while supervised: %v", err)
		}
		return false, nil
	})

	if err := Supervise(dir, testRunner(), mux.Handle{Kind: "zellij", Session: "s"}, EditorCommand{Argv: []string{"vi"}}, false); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("a real exit must end the loop; runner launched %d times", calls)
	}
	if _, err := os.Stat(sessionpaths.AgentPID(dir)); !os.IsNotExist(err) {
		t.Fatal("agent pid file not removed after the supervisor exits")
	}
}

// recordingHost is a PaneHost that answers Attached from a script of replies,
// so a test can decide whether a client showed up before or after the panel
// floated.
type recordingHost struct {
	attachAfter  int // report attached from this Attached call onward
	attachCalls  int
	spawned      []mux.SpawnOptions
	repositioned []mux.Geometry
}

func (h *recordingHost) Attached(context.Context) (bool, error) {
	h.attachCalls++
	return h.attachCalls >= h.attachAfter, nil
}

func (h *recordingHost) Spawn(_ context.Context, opts mux.SpawnOptions) (string, error) {
	h.spawned = append(h.spawned, opts)
	return "terminal_9", nil
}

func (h *recordingHost) Reposition(_ context.Context, _ string, geom mux.Geometry) error {
	h.repositioned = append(h.repositioned, geom)
	return nil
}

func (h *recordingHost) Close(context.Context, string) error { return nil }
func (h *recordingHost) Capture(context.Context, string, bool) (string, error) {
	return "", nil
}
func (h *recordingHost) Exists(context.Context, string) (bool, error) { return true, nil }

func swapHelpPaneHost(t *testing.T, host mux.PaneHost) {
	t.Helper()
	original := helpPaneHost
	helpPaneHost = func(mux.Handle) (mux.PaneHost, error) { return host, nil }
	t.Cleanup(func() { helpPaneHost = original })
}

// shortHelpWaits keeps the panel's client waits from dominating the test run.
func shortHelpWaits(t *testing.T) {
	t.Helper()
	interval, wait, late := clientPollInterval, clientWaitTimeout, lateClientTimeout
	clientPollInterval, clientWaitTimeout, lateClientTimeout = time.Millisecond, 10*time.Millisecond, time.Second
	t.Cleanup(func() {
		clientPollInterval, clientWaitTimeout, lateClientTimeout = interval, wait, late
	})
}

// A client already looking means the panel's percentages resolve against a real
// terminal at spawn time. Repositioning then would be a visible snap for
// nothing.
func TestHelpPanelIsNotRepositionedWhenAClientWasAlreadyThere(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shortHelpWaits(t)
	host := &recordingHost{attachAfter: 1}
	swapHelpPaneHost(t, host)

	spawnHelp(t.TempDir(), mux.Handle{Kind: "zellij", Session: "s"}, "")

	if len(host.spawned) != 1 {
		t.Fatalf("panel spawned %d times, want 1", len(host.spawned))
	}
	if len(host.repositioned) != 0 {
		t.Fatalf("panel was repositioned despite being sized correctly at birth: %v", host.repositioned)
	}
}

// Nobody looking by the deadline: the panel floats anyway (absent is worse than
// squished) and its geometry is re-applied once someone does attach. That repair
// is what the old "squished beats absent" compromise had no way to do.
func TestHelpPanelGeometryIsRepairedAfterALateAttach(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shortHelpWaits(t)
	// Never attached during the first wait; present by the time the repair polls.
	host := &recordingHost{attachAfter: 40}
	swapHelpPaneHost(t, host)
	dir := t.TempDir()

	spawnHelp(dir, mux.Handle{Kind: "zellij", Session: "s"}, "")

	if len(host.spawned) != 1 {
		t.Fatalf("panel spawned %d times, want 1", len(host.spawned))
	}
	if len(host.repositioned) != 1 {
		t.Fatalf("late-attached panel was not repaired: %v", host.repositioned)
	}
	if want := HelpSpawn(dir, "").Geometry; host.repositioned[0] != want {
		t.Fatalf("repaired to %+v, want the panel's own geometry %+v", host.repositioned[0], want)
	}
}

// A session nobody ever attaches to must not leave the supervisor's greeting
// goroutine polling forever.
func TestHelpPanelStopsWaitingWhenNoClientEverArrives(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shortHelpWaits(t)
	lateClientTimeout = 20 * time.Millisecond
	host := &recordingHost{attachAfter: 1 << 30} // never attached
	swapHelpPaneHost(t, host)

	done := make(chan struct{})
	go func() {
		spawnHelp(t.TempDir(), mux.Handle{Kind: "zellij", Session: "s"}, "")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("spawnHelp never gave up waiting for a client")
	}
	if len(host.repositioned) != 0 {
		t.Fatalf("repositioned without a client to size against: %v", host.repositioned)
	}
}
