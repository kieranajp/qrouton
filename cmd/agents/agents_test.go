package agents

// The middle of the attention chain: the subcommand parses one of Claude's hook
// payloads, maps it to an activity, and dials a real control socket. The socket
// here speaks the line protocol itself rather than running the desktop server,
// which links WebKit and would need a display.

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

const (
	notificationPayload = `{"session_id":"4f3a1e19","transcript_path":"/home/t/.claude/projects/-work-webhook/4f3a1e19.jsonl",` +
		`"cwd":"/work/webhook","hook_event_name":"Notification","message":"Claude needs your permission to use Bash"}`
	subagentStartPayload = `{"session_id":"4f3a1e19","transcript_path":"/home/t/.claude/projects/-work-webhook/4f3a1e19.jsonl",` +
		`"cwd":"/work/webhook","hook_event_name":"SubagentStart","agent_id":"agent_017c","agent_type":"Explore"}`
	subagentStopPayload = `{"session_id":"4f3a1e19","transcript_path":"/home/t/.claude/projects/-work-webhook/4f3a1e19.jsonl",` +
		`"cwd":"/work/webhook","hook_event_name":"SubagentStop","agent_id":"agent_017c","agent_type":"Explore"}`
	preToolUsePayload = `{"session_id":"4f3a1e19","transcript_path":"/home/t/.claude/projects/-work-webhook/4f3a1e19.jsonl",` +
		`"cwd":"/work/webhook","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`
)

func TestNotificationHookAsksTheWorkbenchForAttention(t *testing.T) {
	root := sessionRoot(t)
	socket, requests := controlSocket(t)
	if err := runEvent(t, root, handleFor(socket, root), notificationPayload); err != nil {
		t.Fatal(err)
	}
	req := await(t, requests)
	if req.Op != workbench.OpAttention || req.Activity != status.ActivityWaiting || req.Generation != 7 {
		t.Fatalf("request = %#v, want generation-scoped op %q and activity %q", req, workbench.OpAttention, status.ActivityWaiting)
	}
}

func TestSubagentHooksSendLifecycleAndStartClearsAttentionToWorking(t *testing.T) {
	root := sessionRoot(t)
	socket, requests := controlSocket(t)
	if err := runEvent(t, root, handleFor(socket, root), subagentStartPayload); err != nil {
		t.Fatal(err)
	}
	start := await(t, requests)
	if start.Op != workbench.OpDelegatedLifecycle || start.Lifecycle == nil ||
		start.Lifecycle.Kind != workbench.LifecycleStart || start.Lifecycle.ID != "agent_017c" ||
		start.Lifecycle.Type != "Explore" || start.Lifecycle.Generation != 7 || start.Lifecycle.Timestamp.IsZero() {
		t.Fatalf("start request = %#v", start)
	}
	attention := await(t, requests)
	if attention.Op != workbench.OpAttention || attention.Activity != status.ActivityWorking || attention.Generation != 7 {
		t.Fatalf("request = %#v, want generation-scoped op %q and activity %q", attention, workbench.OpAttention, status.ActivityWorking)
	}
	if err := runEvent(t, root, handleFor(socket, root), subagentStopPayload); err != nil {
		t.Fatal(err)
	}
	stop := await(t, requests)
	if stop.Op != workbench.OpDelegatedLifecycle || stop.Lifecycle == nil || stop.Lifecycle.Kind != workbench.LifecycleStop {
		t.Fatalf("stop request = %#v", stop)
	}
}

// A hook outside the map says nothing about attention, so it must not dial at
// all: the desktop reads any activity that is not "waiting" as an answer, so an
// empty one would clear a header the user has not looked at yet.
func TestUnmappedHookSendsNothing(t *testing.T) {
	root := sessionRoot(t)
	socket, requests := controlSocket(t)
	handle := handleFor(socket, root)
	if err := runEvent(t, root, handle, preToolUsePayload); err != nil {
		t.Fatal(err)
	}
	// The mapped hook that follows is the barrier: its request proves the socket
	// was reachable all along, so an empty queue before it means nothing was sent.
	if err := runEvent(t, root, handle, notificationPayload); err != nil {
		t.Fatal(err)
	}
	if got := len(requests); got != 1 {
		t.Fatalf("socket received %d requests, want only the Notification's", got)
	}
	if req := await(t, requests); req.Activity != status.ActivityWaiting {
		t.Fatalf("request = %#v, want the Notification's", req)
	}
}

// A hook that fails is noise in the runner's own output, so an absent workbench
// must not fail the command — and the event still belongs in the session log.
func TestHookSurvivesAnUnreachableWorkbench(t *testing.T) {
	root := sessionRoot(t)
	gone := workbench.Handle{Socket: filepath.Join(t.TempDir(), "gone.sock"), SessionRoot: root}.Marshal()
	if err := runEvent(t, root, gone, subagentStartPayload); err != nil {
		t.Fatalf("unreachable workbench failed the hook: %v", err)
	}
	if err := runEvent(t, root, "", subagentStartPayload); err != nil {
		t.Fatalf("absent workbench handle failed the hook: %v", err)
	}
	logged, err := os.ReadFile(sessionpaths.ClaudeAgentLog(root))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(logged)), "\n")
	if len(lines) != 2 {
		t.Fatalf("agent log holds %d events, want 2:\n%s", len(lines), logged)
	}
	for _, line := range lines {
		if !strings.Contains(line, `"agent_id":"agent_017c"`) {
			t.Fatalf("event lost its agent: %s", line)
		}
	}
}

func runEvent(t *testing.T, root, handle, payload string) error {
	t.Helper()
	feedStdin(t, payload)
	args := []string{"qrouton", eventCommandName, "--" + sessionRootFlag, root}
	args = append(args, "--"+generationFlag, "7")
	if handle != "" {
		args = append(args, "--"+workbenchJSONFlag, handle)
	}
	return (&cli.App{Commands: []*cli.Command{EventCommand}}).Run(args)
}

// The action reads os.Stdin directly, as the hook runner hands it over.
func feedStdin(t *testing.T, payload string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	previous := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = previous
		r.Close()
	})
}

func sessionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(sessionpaths.Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func handleFor(socket, root string) string {
	return workbench.Handle{Socket: socket, SessionRoot: root}.Marshal()
}

func controlSocket(t *testing.T) (string, chan workbench.Request) {
	t.Helper()
	// Not t.TempDir(): its path carries the test's name, and macOS caps a unix
	// socket path at 104 bytes.
	dir, err := os.MkdirTemp("", "qrctl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "c.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	requests := make(chan workbench.Request, 4)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			line, err := bufio.NewReader(conn).ReadBytes('\n')
			if err == nil {
				var req workbench.Request
				if json.Unmarshal(line, &req) == nil {
					requests <- req
				}
				_, _ = conn.Write([]byte("{}\n"))
			}
			conn.Close()
		}
	}()
	return socket, requests
}

func await(t *testing.T, requests chan workbench.Request) workbench.Request {
	t.Helper()
	select {
	case req := <-requests:
		return req
	case <-time.After(5 * time.Second):
		t.Fatal("no request reached the control socket")
		return workbench.Request{}
	}
}

// The log is written only for subagent events, so this payload is the one that
// reaches the file — and a root without .qrouton is the failure the launcher
// normally prevents.
func TestAFailedLogStillSignalsTheWorkbench(t *testing.T) {
	root := t.TempDir()
	socket, requests := controlSocket(t)
	if err := runEvent(t, root, handleFor(socket, root), subagentStartPayload); err == nil {
		t.Fatal("an unwritable agent log reported success")
	}
	if req := await(t, requests); req.Op != workbench.OpDelegatedLifecycle || req.Lifecycle == nil ||
		req.Lifecycle.Kind != workbench.LifecycleStart || req.Lifecycle.ID != "agent_017c" {
		t.Fatalf("request = %#v, want the valid lifecycle event despite the log failure", req)
	}
	if req := await(t, requests); req.Op != workbench.OpAttention || req.Activity != status.ActivityWorking {
		t.Fatalf("request = %#v, want op %q and activity %q", req, workbench.OpAttention, status.ActivityWorking)
	}
}
