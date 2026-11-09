package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

const (
	providerClaude = "claude"
	providerCodex  = "codex"
)

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
	RecordAgentExit(root, providerClaude, 0, "the conversation")

	logged := readAgentLog(t, root)
	if lines := strings.Count(logged, "\n"); lines != 1 {
		t.Fatalf("a clean exit wrote %d lines:\n%s", lines, logged)
	}
	if !strings.Contains(logged, "provider="+providerClaude) || !strings.Contains(logged, "status=0") {
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
	tail := NewRing(AgentTailBytes)
	tail.Write([]byte("\x1b[31mpanic: nothing\x1b[0m\r\nstack\r\n"))

	RecordAgentExit(root, providerCodex, 3, tail.Text(false))

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
	if err := os.WriteFile(path, []byte(strings.Repeat("x", logLimit)), 0o600); err != nil {
		t.Fatal(err)
	}

	RecordAgentExit(root, providerCodex, 0, "")

	logged := readAgentLog(t, root)
	if strings.Contains(logged, "xxx") {
		t.Fatalf("the oversize log was appended to rather than rotated: %d bytes", len(logged))
	}
	previous, err := os.Stat(path + logPreviousSuffix)
	if err != nil {
		t.Fatalf("the previous log was discarded rather than kept: %v", err)
	}
	if previous.Size() != logLimit {
		t.Fatalf("the previous log holds %d bytes, want the file that was rotated", previous.Size())
	}
}

// Cleanup removes a session without waiting for its pump. A directory put back
// by a late log holds a slug that can never be assembled again.
func TestAnExitRecordedAfterTheSessionIsGoneRebuildsNothing(t *testing.T) {
	removed := filepath.Join(t.TempDir(), "octopus")

	RecordAgentExit(removed, providerCodex, 3, "boom")

	if _, err := os.Stat(removed); !os.IsNotExist(err) {
		t.Fatalf("recording an exit recreated the removed session directory: %v", err)
	}
}
