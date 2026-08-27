package launch

import (
	"os"
	"strings"
	"testing"

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

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, false); err != nil {
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

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, false); err != nil {
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

func TestSuperviseConsumesTheExternalPromptOnTheFirstLaunchOnly(t *testing.T) {
	dir := superviseTestDir(t, session.ModeAssistant)
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.InitialPrompt(dir), []byte("Fix the login regression."), 0o600); err != nil {
		t.Fatal(err)
	}
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		return len(argvs) == 1, nil
	})

	// Even a restart asking to resume must start fresh when the prompt is still
	// pending: otherwise the external request would never enter the conversation.
	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, true); err != nil {
		t.Fatal(err)
	}
	if len(argvs) != 2 {
		t.Fatalf("expected two launches, got %d", len(argvs))
	}
	if first := strings.Join(argvs[0], " "); !strings.Contains(first, openingMessageAssistant) ||
		!strings.Contains(first, linearRequestSeparator+"Fix the login regression.") ||
		strings.Contains(first, claudeContinueFlag) {
		t.Fatalf("first launch did not layer the external prompt into a fresh opening: %v", argvs[0])
	}
	if second := strings.Join(argvs[1], " "); !strings.Contains(second, claudeContinueFlag) ||
		strings.Contains(second, "Fix the login regression") {
		t.Fatalf("relaunch repeated the external prompt or lost the conversation: %v", argvs[1])
	}
	if _, err := os.Stat(sessionpaths.InitialPrompt(dir)); !os.IsNotExist(err) {
		t.Fatalf("consumed initial prompt remains on disk: %v", err)
	}
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

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, true); err != nil {
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
	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, true); err != nil {
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

func TestSuperviseEndsTheTerminalOnUnsignalledExit(t *testing.T) {
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

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, false); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("a real exit must end the loop; runner launched %d times", calls)
	}
	if _, err := os.Stat(sessionpaths.AgentPID(dir)); !os.IsNotExist(err) {
		t.Fatal("agent pid file not removed after the supervisor exits")
	}
}
