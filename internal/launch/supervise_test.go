package launch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// swapRunAgent replaces the exec seam with a scripted closure and restores it.
func swapRunAgent(t *testing.T, fake func(argv, env []string, dir string, relaunch <-chan os.Signal) (bool, error)) {
	t.Helper()
	original := runAgent
	originalAnnounce := announceRunnerGeneration
	originalGeneration := firstRunnerGeneration
	runAgent = fake
	announceRunnerGeneration = func(workbench.Handle, string, uint64) error { return nil }
	firstRunnerGeneration = func() uint64 { return 1 }
	t.Cleanup(func() {
		runAgent = original
		announceRunnerGeneration = originalAnnounce
		firstRunnerGeneration = originalGeneration
	})
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

func TestSuperviseAnnouncesEverySignalDrivenRunGeneration(t *testing.T) {
	dir := superviseTestDir(t, session.ModeAssistant)
	var announced []uint64
	var hookArgv [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		hookArgv = append(hookArgv, argv)
		return len(hookArgv) == 1, nil
	})
	announceRunnerGeneration = func(_ workbench.Handle, provider string, generation uint64) error {
		if provider != runnerIDClaude {
			t.Fatalf("announced provider = %q, want %q", provider, runnerIDClaude)
		}
		announced = append(announced, generation)
		return nil
	}

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{}, false); err != nil {
		t.Fatal(err)
	}
	if len(announced) != 2 || announced[0] != 1 || announced[1] != 2 {
		t.Fatalf("announced generations = %v, want [1 2]", announced)
	}
	for i, argv := range hookArgv {
		want := generationFlag + " " + strconv.Itoa(i+1)
		if !strings.Contains(strings.Join(argv, " "), want) {
			t.Fatalf("launch %d does not stamp %q into its hooks: %v", i+1, want, argv)
		}
	}
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

func TestSuperviseDeliversARepositoryNoticeOnTheResumedConversationOnce(t *testing.T) {
	dir := superviseTestDir(t, session.ModeAssistant)
	const notice = "qrouton: session repositories changed — added org/svc for editing at src/svc."
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		if len(argvs) == 1 {
			if err := session.QueueAgentNotice(dir, notice); err != nil {
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
		t.Fatalf("expected two launches, got %d", len(argvs))
	}
	if first := strings.Join(argvs[0], " "); strings.Contains(first, notice) {
		t.Fatalf("notice reached the launch before it was queued: %v", argvs[0])
	}
	if second := strings.Join(argvs[1], " "); !strings.Contains(second, claudeContinueFlag) || !strings.Contains(second, notice) {
		t.Fatalf("resumed launch = %v, want continue with notice", argvs[1])
	}
	if _, err := os.Stat(sessionpaths.AgentNotice(dir)); !os.IsNotExist(err) {
		t.Fatalf("consumed agent notice remains on disk: %v", err)
	}
}

// A pending handoff prevents resume when escalation completed before this launch.
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

// A session with no editor is a session, and the supervisor hands the absence to
// the runner the same way it hands over a resolved editor.
func TestSuperviseLaunchesWithNoEditor(t *testing.T) {
	dir := superviseTestDir(t, session.ModeRPI)
	var argvs, envs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs, envs = append(argvs, argv), append(envs, env)
		return false, nil
	})

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{}, false); err != nil {
		t.Fatalf("a session with no editor refused to launch: %v", err)
	}
	if len(argvs) != 1 {
		t.Fatalf("launched %d runners, want 1", len(argvs))
	}
	inherited := ""
	for _, entry := range envs[0] {
		if value, ok := strings.CutPrefix(entry, EditorEnvVar+"="); ok {
			inherited = value
		}
	}
	if inherited == "" {
		t.Fatalf("the child inherits no %s at all", EditorEnvVar)
	}
	editor, err := ParseEditor(inherited)
	if err != nil {
		t.Fatalf("the child inherits %q, which it cannot read: %v", inherited, err)
	}
	if len(editor.Argv) != 0 {
		t.Fatalf("inherited editor = %#v, want none", editor)
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

// signalCatcher starts a child that records SIGUSR1 and exits, answering with
// its pid and a wait on what it recorded. It reports itself ready first, so a
// signal cannot land before the handler that has to survive it is installed.
func signalCatcher(t *testing.T) (int, func(time.Duration) bool) {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "usr1")
	cmd := exec.Command("/bin/sh", "-c",
		`trap 'printf caught > "$0"; exit 0' USR1; printf ready > "$0.ready"; while :; do sleep 0.02; done`,
		marker)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	if !appears(marker+".ready", 10*time.Second) {
		t.Fatal("the child never installed a SIGUSR1 handler")
	}
	return cmd.Process.Pid, func(within time.Duration) bool { return appears(marker, within) }
}

func appears(path string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func writeAgentPID(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.AgentPID(dir), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// exitedPID is the pid of a child that has already been reaped, which is what a
// pid file outliving its supervisor names.
func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	return pid
}

// The escalation reaches a running supervisor by this signal alone: the manifest
// is already rewritten, and only SIGUSR1 at the recorded pid relaunches the
// runner against it.
func TestSignalSupervisorSignalsTheRecordedSupervisor(t *testing.T) {
	dir := t.TempDir()
	pid, caught := signalCatcher(t)
	writeAgentPID(t, dir, strconv.Itoa(pid))

	SignalSupervisor(dir)

	if !caught(10 * time.Second) {
		t.Fatal("no SIGUSR1 reached the recorded pid, so an escalation would leave the old runner running")
	}
}

// The supervisor writes the pid file itself, and a signaller reading it has to
// find the process it named.
func TestSignalSupervisorReachesASupervisorThroughTheFileItWrote(t *testing.T) {
	dir := t.TempDir()
	if err := writePID(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(sessionpaths.AgentPID(dir))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != strconv.Itoa(os.Getpid()) {
		t.Fatalf("recorded pid = %q, want this process (%d)", got, os.Getpid())
	}
}

// An unusable pid file is not an error: the mode change is already on disk and
// takes effect at the next launch. What it must never do is signal something
// else — a pid of 0 or -1 handed to kill reaches a whole process group.
func TestSignalSupervisorSignalsNobodyWithoutALiveSupervisor(t *testing.T) {
	dead := strconv.Itoa(exitedPID(t))
	for _, tc := range []struct {
		name    string
		pidFile string
		write   bool
	}{
		{name: "no pid file"},
		{name: "empty", write: true},
		{name: "not a number", pidFile: "supervisor", write: true},
		{name: "zero", pidFile: "0", write: true},
		{name: "whole process group", pidFile: "-1", write: true},
		{name: "supervisor already exited", pidFile: dead, write: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bystander, caught := signalCatcher(t)
			if tc.write {
				writeAgentPID(t, dir, tc.pidFile)
			}

			SignalSupervisor(dir)

			if caught(200 * time.Millisecond) {
				t.Fatalf("a %s pid file signalled process %d", tc.name, bystander)
			}
		})
	}
}

// A missing transcript gets a fresh launch instead of ending the terminal.
func TestSuperviseStartsFreshWhenThereIsNoConversationToResume(t *testing.T) {
	dir := superviseTestDir(t, session.ModeRPI)
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		if len(argvs) == 1 {
			return false, errors.New("exit status 1")
		}
		return false, nil
	})

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, true); err != nil {
		t.Fatalf("a resume with nothing to resume must not end the terminal: %v", err)
	}
	if len(argvs) != 2 {
		t.Fatalf("expected the failed resume to be followed by a fresh launch, got %d launches", len(argvs))
	}
	if first := strings.Join(argvs[0], " "); !strings.Contains(first, claudeContinueFlag) {
		t.Fatalf("first launch = %v, want a resume", argvs[0])
	}
	if second := strings.Join(argvs[1], " "); strings.Contains(second, claudeContinueFlag) {
		t.Fatalf("fallback launch = %v, want a fresh conversation", argvs[1])
	}
}

// The fallback is spent once. A runner failing for its own reasons — a bad flag,
// a missing binary — must still end the terminal rather than relaunch forever.
func TestSuperviseEndsTheTerminalWhenTheFreshStartFailsToo(t *testing.T) {
	dir := superviseTestDir(t, session.ModeRPI)
	calls := 0
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		calls++
		if calls > 8 {
			t.Fatal("the supervisor is relaunching a failing runner without end")
		}
		return false, errors.New("exit status 1")
	})

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, true); err == nil {
		t.Fatal("a runner that fails on a fresh conversation must surface its error")
	}
	if calls != 2 {
		t.Fatalf("expected the resume and one fresh start, got %d launches", calls)
	}
}

// Nothing to fall back to: a launch that never asked to resume has no lost
// conversation to explain its failure, so its error is the runner's own.
func TestSuperviseEndsTheTerminalWhenAFreshLaunchFails(t *testing.T) {
	dir := superviseTestDir(t, session.ModeRPI)
	calls := 0
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		calls++
		return false, errors.New("exit status 1")
	})

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, false); err == nil {
		t.Fatal("a failing fresh launch must surface its error")
	}
	if calls != 1 {
		t.Fatalf("a fresh launch has nothing to fall back to; runner launched %d times", calls)
	}
}

// The notice is consumed by the launch that fails, so the fallback has to carry
// it or a repository change is announced to nobody.
func TestSuperviseCarriesTheNoticeOntoTheFreshStart(t *testing.T) {
	dir := superviseTestDir(t, session.ModeRPI)
	const notice = "qrouton: session repositories changed — added org/svc for editing at src/svc."
	if err := session.QueueAgentNotice(dir, notice); err != nil {
		t.Fatal(err)
	}
	var argvs [][]string
	swapRunAgent(t, func(argv, env []string, d string, relaunch <-chan os.Signal) (bool, error) {
		argvs = append(argvs, argv)
		if len(argvs) == 1 {
			return false, errors.New("exit status 1")
		}
		return false, nil
	})

	if err := Supervise(dir, testRunner(), testHandle(), EditorCommand{Argv: []string{"vi"}}, true); err != nil {
		t.Fatal(err)
	}
	if len(argvs) != 2 {
		t.Fatalf("expected a fresh launch after the failed resume, got %d", len(argvs))
	}
	if second := strings.Join(argvs[1], " "); !strings.Contains(second, notice) {
		t.Fatalf("fallback launch dropped the notice: %v", argvs[1])
	}
}
