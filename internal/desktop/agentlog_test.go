package desktop

import (
	"os"
	"path/filepath"
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

func readAgentLog(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(sessionpaths.AgentLog(root))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A supervisor that ended on its own terms has nothing to explain, and a log
// carrying the whole conversation after every clean exit is one nobody reads.
func TestACleanAgentExitIsOneLineWithNoOutput(t *testing.T) {
	root := sessionRoot(t)
	recordAgentExit(root, agentProviderClaude, 0, "the conversation")

	logged := readAgentLog(t, root)
	if lines := strings.Count(logged, "\n"); lines != 1 {
		t.Fatalf("a clean exit wrote %d lines:\n%s", lines, logged)
	}
	if !strings.Contains(logged, "provider="+agentProviderClaude) || !strings.Contains(logged, "status=0") {
		t.Fatalf("the entry does not name the provider and its status:\n%s", logged)
	}
	if strings.Contains(logged, "the conversation") {
		t.Fatalf("a clean exit kept the terminal's output:\n%s", logged)
	}
}

// The tail is the only account of why an agent went away, so it is written as a
// person reads it rather than as the terminal painted it.
func TestAFailedAgentExitKeepsTheTailWithoutItsEscapes(t *testing.T) {
	root := sessionRoot(t)
	state := &sessionState{tail: &ring{limit: agentTailBytes}}
	state.tail.write([]byte("\x1b[31mpanic: nothing\x1b[0m\r\nstack\r\n"))

	recordAgentExit(root, agentProviderCodex, 3, state.tail.text(false))

	logged := readAgentLog(t, root)
	if !strings.Contains(logged, "status=3") {
		t.Fatalf("the entry does not name the failure:\n%s", logged)
	}
	if !strings.Contains(logged, "panic: nothing\nstack") {
		t.Fatalf("the tail is missing from:\n%s", logged)
	}
	if strings.Contains(logged, "\x1b[") {
		t.Fatalf("the tail carries escape sequences:\n%s", logged)
	}
}

// Two files per session at most: a supervisor dying over and over must not grow
// a log until the session's directory is the problem.
func TestAnOversizeAgentLogIsRotatedRatherThanGrown(t *testing.T) {
	root := sessionRoot(t)
	path := sessionpaths.AgentLog(root)
	if err := os.WriteFile(path, []byte(strings.Repeat("x", agentLogLimit)), 0o600); err != nil {
		t.Fatal(err)
	}

	recordAgentExit(root, agentProviderCodex, 0, "")

	logged := readAgentLog(t, root)
	if strings.Contains(logged, "xxx") {
		t.Fatalf("the oversize log was appended to rather than rotated: %d bytes", len(logged))
	}
	previous, err := os.Stat(path + agentLogPreviousSuffix)
	if err != nil {
		t.Fatalf("the previous log was discarded rather than kept: %v", err)
	}
	if previous.Size() != agentLogLimit {
		t.Fatalf("the previous log holds %d bytes, want the file that was rotated", previous.Size())
	}
}

// agent.log is the account of deaths the workbench did not cause. Logging the
// ones it caused itself would rotate the crash that mattered off the disk.
func TestAnExitTheWorkbenchAskedForRecordsNothing(t *testing.T) {
	root := sessionRoot(t)
	state := &sessionState{stopped: true, tail: &ring{limit: agentTailBytes}}
	state.named.Store(&identity{slug: filepath.Base(root), root: root})
	state.tail.write([]byte("still talking"))

	state.recordExit(-1)

	if _, err := os.Stat(sessionpaths.AgentLog(root)); !os.IsNotExist(err) {
		t.Fatalf("a supervisor the workbench stopped recorded an exit: %v", err)
	}
}

// Cleanup removes a session without waiting for its pump. A directory put back
// by a late log holds a slug that can never be assembled again.
func TestAnExitRecordedAfterTheSessionIsGoneRebuildsNothing(t *testing.T) {
	removed := filepath.Join(t.TempDir(), "octopus")

	recordAgentExit(removed, agentProviderCodex, 3, "boom")

	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatalf("recording an exit recreated the removed session directory: %v", err)
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
	state := reg.current()
	term := newTerm(reg, r.Emit)
	if err := term.Start(state.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the supervisor's exit to be recorded", func() bool {
		data, err := os.ReadFile(sessionpaths.AgentLog(state.root()))
		return err == nil && strings.Contains(string(data), "status=3")
	})
	if logged := readAgentLog(t, state.root()); !strings.Contains(logged, "boom") {
		t.Fatalf("the record does not carry what the terminal printed:\n%s", logged)
	}
}
