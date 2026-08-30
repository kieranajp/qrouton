package agentevent

// The middle of the attention chain: the subcommand parses one runner hook
// payload, maps it to an activity, and dials a real control socket. The socket
// here speaks the line protocol itself rather than running the desktop server,
// which links WebKit and would need a display.

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	codexStartPayload = `{"session_id":"lead_012","transcript_path":"/work/.codex/rollout.jsonl",` +
		`"cwd":"/work/webhook","hook_event_name":"SubagentStart","agent_id":"agent_017c","agent_type":"code-reviewer","model":"gpt-5.6-sol"}`
	preToolUsePayload = `{"session_id":"4f3a1e19","transcript_path":"/home/t/.claude/projects/-work-webhook/4f3a1e19.jsonl",` +
		`"cwd":"/work/webhook","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`
)

func TestNotificationHookAsksTheWorkbenchForAttention(t *testing.T) {
	root := t.TempDir()
	socket, requests := controlSocket(t)
	if err := runEvent(t, "claude", handleFor(socket, root), notificationPayload); err != nil {
		t.Fatal(err)
	}
	req := await(t, requests)
	if req.Op != workbench.OpAttention || req.Activity != status.ActivityWaiting || req.Generation != 7 {
		t.Fatalf("request = %#v, want generation-scoped op %q and activity %q", req, workbench.OpAttention, status.ActivityWaiting)
	}
}

func TestSubagentHooksSendLifecycleAndStartClearsAttentionToWorking(t *testing.T) {
	root := t.TempDir()
	socket, requests := controlSocket(t)
	if err := runEvent(t, "claude", handleFor(socket, root), subagentStartPayload); err != nil {
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
	if err := runEvent(t, "claude", handleFor(socket, root), subagentStopPayload); err != nil {
		t.Fatal(err)
	}
	stop := await(t, requests)
	if stop.Op != workbench.OpDelegatedLifecycle || stop.Lifecycle == nil || stop.Lifecycle.Kind != workbench.LifecycleStop {
		t.Fatalf("stop request = %#v", stop)
	}
}

func TestCodexHookSendsProviderAndParentLifecycle(t *testing.T) {
	root := t.TempDir()
	socket, requests := controlSocket(t)
	if err := runEvent(t, "codex", handleFor(socket, root), codexStartPayload); err != nil {
		t.Fatal(err)
	}
	start := await(t, requests)
	if start.Lifecycle == nil || start.Lifecycle.Provider != "codex" || start.Lifecycle.Kind != workbench.LifecycleStart ||
		start.Lifecycle.ID != "agent_017c" || start.Lifecycle.ParentID != "lead_012" {
		t.Fatalf("Codex lifecycle = %+v", start)
	}
	if attention := await(t, requests); attention.Op != workbench.OpAttention || attention.Activity != status.ActivityWorking {
		t.Fatalf("Codex attention = %+v", attention)
	}
}

// A hook outside the map says nothing about attention, so it must not dial at
// all: the desktop reads any activity that is not "waiting" as an answer, so an
// empty one would clear a header the user has not looked at yet.
func TestUnmappedHookSendsNothing(t *testing.T) {
	root := t.TempDir()
	socket, requests := controlSocket(t)
	handle := handleFor(socket, root)
	if err := runEvent(t, "claude", handle, preToolUsePayload); err != nil {
		t.Fatal(err)
	}
	// The mapped hook that follows is the barrier: its request proves the socket
	// was reachable all along, so an empty queue before it means nothing was sent.
	if err := runEvent(t, "claude", handle, notificationPayload); err != nil {
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
// must not fail the command.
func TestHookSurvivesAnUnreachableWorkbench(t *testing.T) {
	gone := workbench.Handle{Socket: filepath.Join(t.TempDir(), "gone.sock"), SessionRoot: t.TempDir()}.Marshal()
	if err := runEvent(t, "claude", gone, subagentStartPayload); err != nil {
		t.Fatalf("unreachable workbench failed the hook: %v", err)
	}
	if err := runEvent(t, "claude", "", subagentStartPayload); err != nil {
		t.Fatalf("absent workbench handle failed the hook: %v", err)
	}
}

func runEvent(t *testing.T, provider, handle, payload string) error {
	t.Helper()
	feedStdin(t, payload)
	args := []string{"qrouton", eventCommandName, "--" + generationFlag, "7", "--" + providerFlag, provider}
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
