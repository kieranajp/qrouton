package evalharness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
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
printf 'args: %s\n' "$*" >> "`+logPath+`"
printf 'stdin: %s\n' "$(cat)" >> "`+logPath+`"
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
	if !strings.Contains(string(log), "stdin: first") || !strings.Contains(string(log), "stdin: second") {
		t.Fatalf("prompts did not arrive over stdin:\n%s", log)
	}
	for _, line := range strings.Split(string(log), "\n") {
		if strings.HasPrefix(line, "args: ") && (strings.Contains(line, "first") || strings.Contains(line, "second")) {
			t.Fatalf("prompt leaked into argv: %s", line)
		}
	}
}

func TestCodexContinuationArguments(t *testing.T) {
	adapter := Adapter{Name: "codex", Bin: "codex", SelfPath: "/tmp/qrouton-eval"}
	args, err := adapter.args("/tmp/workspace", "/tmp/mcp.log", "thread-42")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.HasPrefix(joined, "exec resume ") {
		t.Fatalf("resume subcommand is misplaced: %s", joined)
	}
	if !strings.HasSuffix(joined, "thread-42 -") {
		t.Fatalf("thread ID or stdin prompt marker is missing: %s", joined)
	}
}

func TestPromptStaysOutOfArgv(t *testing.T) {
	// Judge prompts embed candidate artifacts and diffs; a single argv element
	// caps out at ~128KiB on Linux, so the prompt must never appear in args.
	claude := Adapter{Name: "claude", Bin: "claude", SelfPath: "/tmp/qrouton-eval"}
	args, err := claude.args("/tmp/workspace", "/tmp/mcp.log", "")
	if err != nil {
		t.Fatal(err)
	}
	if last := args[len(args)-1]; strings.HasPrefix(last, "--mcp-config") {
		t.Fatalf("variadic --mcp-config left dangling at end of argv: %q", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--setting-sources project") || !strings.Contains(joined, "--strict-mcp-config") {
		t.Fatalf("Claude user configuration is not isolated: %s", joined)
	}

	codex := Adapter{Name: "codex", Bin: "codex", SelfPath: "/tmp/qrouton-eval"}
	args, err = codex.args("/tmp/workspace", "/tmp/mcp.log", "")
	if err != nil {
		t.Fatal(err)
	}
	if args[len(args)-1] != "-" {
		t.Fatalf("codex argv does not end with the stdin marker: %q", args)
	}
}

func TestCodexIgnoresUserConfig(t *testing.T) {
	adapter := Adapter{Name: "codex", Bin: "codex", SelfPath: "/tmp/qrouton-eval"}
	args, err := adapter.args("/tmp/workspace", "/tmp/mcp.log", "")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--ignore-user-config") {
		t.Fatalf("Codex user configuration is not isolated: %s", strings.Join(args, " "))
	}
	if !strings.Contains(joined, "--enable multi_agent") {
		t.Fatalf("Codex multi-agent support is not enabled: %s", joined)
	}
	if !strings.Contains(joined, "--skip-git-repo-check") {
		t.Fatalf("Codex cannot run in an isolated judge directory: %s", joined)
	}
}

// A graded run has to reach the qrouton tool surface the way a launched agent
// does. Wiring the mock server by hand here is how the eval ends up grading a
// differently configured agent than the one that ships.
func TestAdapterWiresMCPTheWayLaunchDoes(t *testing.T) {
	const self = "/tmp/qrouton-eval"
	workspace, mcpLog := "/tmp/workspace", "/tmp/mcp.log"
	for _, runner := range []string{runnerClaude, runnerCodex} {
		adapter := Adapter{Name: runner, Bin: runner, SelfPath: self}
		args, err := adapter.args(workspace, mcpLog, "")
		if err != nil {
			t.Fatal(err)
		}
		wiring, err := launch.RunnerMCPWiring(runner, self, mockMCPArgs(mcpLog, workspace))
		if err != nil {
			t.Fatal(err)
		}
		if len(wiring.Args) == 0 {
			t.Fatalf("%s has no MCP arguments to carry", runner)
		}
		if !strings.Contains(strings.Join(args, "\x00"), strings.Join(wiring.Args, "\x00")) {
			t.Errorf("%s argv %q does not carry the launch wiring %q", runner, args, wiring.Args)
		}
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
