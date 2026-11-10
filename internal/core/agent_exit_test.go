package core

import (
	"os"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// sessionRoot is a session as it exists on disk: every assembled one has its
// private directory, and the log is only ever written beside what is already there.
func sessionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(sessionpaths.Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// agent.log is the account of deaths the workbench did not cause. Logging the
// ones it caused itself would rotate the crash that mattered off the disk.
func TestAnExitTheWorkbenchAskedForRecordsNothing(t *testing.T) {
	root := sessionRoot(t)
	state := &SessionState{stopped: true, sessionRoot: root, output: newOutputStream(WindowScrollback)}
	state.output.ring.Write([]byte("still talking"))

	state.recordExit(-1)

	if _, err := os.Stat(sessionpaths.AgentLog(root)); !os.IsNotExist(err) {
		t.Fatalf("a supervisor the workbench stopped recorded an exit: %v", err)
	}
}

// The log is written from the pump rather than the workbench's exit wiring, so
// a session records its own death whatever else is listening.
func TestASupervisorFailingRecordsItsExitAgainstTheSession(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/sh", "-c", "printf 'boom\\n'; exit 3")
	reg, _, r := testSessions(t, root, boot)
	if err := os.MkdirAll(sessionpaths.Dir(sessionDir(t, root, "octopus")), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	state := reg.Current()
	term := newTerm(reg, emitFunc(r.Emit))
	if err := startTerm(term, state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the supervisor's exit to be recorded", func() bool {
		data, err := os.ReadFile(sessionpaths.AgentLog(state.Root()))
		return err == nil && strings.Contains(string(data), "status=3")
	})
	if logged, _ := os.ReadFile(sessionpaths.AgentLog(state.Root())); !strings.Contains(string(logged), "boom") {
		t.Fatalf("the record does not carry what the terminal printed:\n%s", logged)
	}
}

// Catches the exit log keeping less than the last screen, which a ring shorter
// than one screen of long lines would.
func TestACrashedConversationsExitLogHoldsItsLastScreen(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/sh", "-c",
		"i=1; while [ $i -le 80 ]; do printf 'line-%03d-%0180d\\n' $i 0; i=$((i+1)); done; printf boom; exit 3")
	reg, _, r := testSessions(t, root, boot)
	if err := os.MkdirAll(sessionpaths.Dir(sessionDir(t, root, "octopus")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	state := reg.Current()
	if err := startTerm(newTerm(reg, emitFunc(r.Emit)), state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	var logged string
	waitFor(t, "the crash to be recorded", func() bool {
		data, err := os.ReadFile(sessionpaths.AgentLog(state.Root()))
		logged = string(data)
		return err == nil && strings.Contains(logged, "status=3")
	})
	if !strings.Contains(logged, "line-032-") || !strings.Contains(logged, "boom") {
		t.Fatalf("the record lost part of the last screen:\n%s", logged)
	}
	if strings.Contains(logged, "line-031-") {
		t.Fatalf("the record holds more than the last screen:\n%s", logged)
	}
}
