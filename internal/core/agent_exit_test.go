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
	state := &SessionState{stopped: true, sessionRoot: root, tail: NewRing(AgentTailBytes)}
	state.tail.Write([]byte("still talking"))

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
	if err := term.Start(state.terminal, 80, 24); err != nil {
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
