package evalharness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeClaudeAndCodexEvents(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		stream   string
		session  string
		final    string
	}{
		{
			name:     "claude",
			provider: "claude",
			stream:   "{\"type\":\"system\",\"session_id\":\"session-1\"}\n{\"type\":\"result\",\"result\":\"done\"}\n",
			session:  "session-1",
			final:    "done",
		},
		{
			name:     "codex",
			provider: "codex",
			stream:   "{\"type\":\"thread.started\",\"thread_id\":\"thread-1\"}\n{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"done\"}}\n",
			session:  "thread-1",
			final:    "done",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events, final, session, err := Normalize(test.provider, []byte(test.stream), 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 2 || final != test.final || session != test.session {
				t.Fatalf("events=%d final=%q session=%q", len(events), final, session)
			}
		})
	}
}

func TestNormalizeRejectsMalformedOutput(t *testing.T) {
	_, _, _, err := Normalize("claude", []byte("not-json\n"), 1)
	if err == nil {
		t.Fatal("expected malformed event error")
	}
}

func TestNormalizeRemovesHiddenReasoning(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"item.completed","item":{"type":"reasoning","text":"hidden"}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"visible"}}`,
	}, "\n")
	events, final, _, err := Normalize("codex", []byte(stream), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if final != "visible" || strings.Contains(string(events[0].Arguments), "hidden") {
		t.Fatalf("hidden reasoning reached normalized trace: %#v", events)
	}
}

func TestAdapterContinuesSession(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	script := filepath.Join(dir, "fake-claude")
	writeTestFile(t, script, `#!/bin/sh
printf '%s\n' "$*" >> "`+logPath+`"
printf '%s\n' '{"type":"system","session_id":"session-42"}'
printf '%s\n' '{"type":"result","result":"done"}'
`)
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}

	adapter := Adapter{Name: "claude", Bin: script, SelfPath: script}
	_, _, session, err := adapter.RunTurn(context.Background(), dir, filepath.Join(dir, "mcp.log"), "first", "", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := adapter.RunTurn(context.Background(), dir, filepath.Join(dir, "mcp.log"), "second", session, 2); err != nil {
		t.Fatal(err)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "--resume session-42") {
		t.Fatalf("continuation args missing from log:\n%s", log)
	}
}

func TestCodexContinuationArguments(t *testing.T) {
	adapter := Adapter{Name: "codex", Bin: "codex", SelfPath: "/tmp/qrouton-eval"}
	args, err := adapter.args("/tmp/workspace", "/tmp/mcp.log", "continue", "thread-42")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.HasPrefix(joined, "exec resume ") {
		t.Fatalf("resume subcommand is misplaced: %s", joined)
	}
	if !strings.Contains(joined, "thread-42 continue") {
		t.Fatalf("thread ID or prompt is missing: %s", joined)
	}
}

func TestAdapterHonorsContextTimeout(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "slow-claude")
	writeTestFile(t, script, "#!/bin/sh\nsleep 2\n")
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	adapter := Adapter{Name: "claude", Bin: script, SelfPath: script}
	_, _, _, err := adapter.RunTurn(ctx, dir, filepath.Join(dir, "mcp.log"), "prompt", "", 1)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("context error = %v", ctx.Err())
	}
}
