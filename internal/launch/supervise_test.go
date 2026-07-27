package launch

import (
	"os"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// swapRunAgent replaces the exec seam with a scripted closure and restores it.
func swapRunAgent(t *testing.T, fake func(argv, env []string, dir string, relaunch <-chan os.Signal) (bool, error)) {
	t.Helper()
	original := runAgent
	runAgent = fake
	t.Cleanup(func() { runAgent = original })
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
