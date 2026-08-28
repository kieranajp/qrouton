package desktop

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// adoption is what the adopt hook was told: the session chosen, and whether the
// workbench is the one to boot its agent.
type adoption struct {
	root string
	boot bool
}

// The control socket is the one place the port's wire format is agreed, and the
// two halves are compiled separately — so this drives the real server through
// the real client rather than either side's idea of the other.
func TestTheControlSocketServesTheWorkbenchPort(t *testing.T) {
	windows, r := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	queued := make(chan workbench.PickerRequest, 1)
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{
		picker: func(req workbench.PickerRequest) error { queued <- req; return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	host := (workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}).WindowHost()
	ctx := context.Background()

	id, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tabs := windows.surfaces(windows.shown()).Tabs; len(tabs) != 1 || tabs[0].ID != id {
		t.Fatalf("socket-opened tabs = %+v, want terminal %q", tabs, id)
	}

	document, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ plan", Content: "one change",
		Format: workbench.FormatMarkdown, Source: "thoughts/shared/plans/P1.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tabs := windows.surfaces(windows.shown()).Tabs; len(tabs) != 2 || tabs[1].ID != document {
		t.Fatalf("socket-opened tabs = %+v, want document %q second", tabs, document)
	}
	select {
	case spec := <-r.opened:
		t.Fatalf("socket-opened tab reached the renderer: %+v", spec)
	default:
	}

	if live, err := host.Exists(ctx, id); err != nil || !live {
		t.Fatalf("Exists = %v, %v for a window that is open", live, err)
	}
	ids, err := host.List(ctx)
	if err != nil || len(ids) != 2 {
		t.Fatalf("List = %v, %v, want both windows", ids, err)
	}
	text, err := host.Read(ctx, document, false)
	if err != nil || text != "one change" {
		t.Fatalf("Read = %q, %v", text, err)
	}
	const diff = `diff --git a/app.txt b/app.txt
index 6dad4ad..84db3de 100644
--- a/app.txt
+++ b/app.txt
@@ -1,4 +1,4 @@
 alpha
-beta
+bravo
 gamma
 delta
@@ -10,3 +10,4 @@ footer
 ten
 eleven
+eleven and a half
 twelve
`
	diffDocument, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ diff", Content: diff, Format: workbench.FormatDiff,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tabs := windows.surfaces(windows.shown()).Tabs; len(tabs) != 3 || tabs[2].ID != diffDocument {
		t.Fatalf("socket-opened tabs = %+v, want diff document %q third", tabs, diffDocument)
	}
	diffPage, err := windows.Content(diffDocument)
	if err != nil || diffPage.Text != diff || diffPage.Format != string(workbench.FormatDiff) {
		t.Fatalf("diff Content = %+v, %v", diffPage, err)
	}
	diffText, err := host.Read(ctx, diffDocument, true)
	if err != nil || diffText != diff {
		t.Fatalf("diff Read = %q, %v", diffText, err)
	}
	if viewport, err := host.Viewport(ctx, diffDocument); err != nil || viewport != nil {
		t.Fatalf("diff Viewport = %+v, %v", viewport, err)
	}
	if viewport, err := host.Viewport(ctx, id); err != nil || viewport != nil {
		t.Fatalf("terminal Viewport = %+v, %v", viewport, err)
	}
	viewport, err := host.Viewport(ctx, document)
	if err != nil || viewport == nil || viewport.Available || viewport.Intervals == nil {
		t.Fatalf("initial Markdown Viewport = %+v, %v", viewport, err)
	}
	page, err := windows.Content(document)
	if err != nil || page.ViewportEpoch == 0 {
		t.Fatalf("Content viewport epoch = %d, %v", page.ViewportEpoch, err)
	}
	if err := windows.ReportViewport(document, ViewportReport{
		Epoch: page.ViewportEpoch, Seq: 1, Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 4, To: 6}, {Line: 12, To: 12}},
	}); err != nil {
		t.Fatal(err)
	}
	viewport, err = host.Viewport(ctx, document)
	if err != nil || viewport == nil || viewport.Source != "thoughts/shared/plans/P1.md" ||
		len(viewport.Intervals) != 2 || viewport.Intervals[0] != (workbench.LineInterval{Line: 4, To: 6}) {
		t.Fatalf("measured Markdown Viewport = %+v, %v", viewport, err)
	}
	deadline := time.Now().Add(time.Minute)
	if err := host.Picker(ctx, workbench.PickerRequest{SessionRoot: "/sessions/octopus",
		Name: "Webhook retry", Prefix: "fix", Deadline: deadline}); err != nil {
		t.Fatal(err)
	}
	if got := <-queued; got.SessionRoot != "/sessions/octopus" || got.Name != "Webhook retry" ||
		got.Prefix != "fix" || !got.Deadline.Equal(deadline) {
		t.Fatalf("queued %+v, want the request the caller sent", got)
	}

	if err := host.Close(ctx, id); err != nil {
		t.Fatal(err)
	}
	if live, err := host.Exists(ctx, id); err != nil || live {
		t.Fatalf("Exists = %v, %v after Close", live, err)
	}
	if tabs := windows.surfaces(windows.shown()).Tabs; len(tabs) != 2 || tabs[0].ID != document || tabs[1].ID != diffDocument {
		t.Fatalf("tabs after close = %+v, want documents %q and %q", tabs, document, diffDocument)
	}
}

// A refusal is the desktop process's answer, not a transport failure, so the
// caller must read the reason rather than a dial error.
func TestTheControlSocketAnswersBadRequestsWithTheirReason(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	host := (workbench.Handle{Socket: socket, SessionRoot: t.TempDir()}).WindowHost()
	ctx := context.Background()

	if _, err := host.Read(ctx, "window-99", false); err == nil {
		t.Fatal("read of an unknown window succeeded")
	}
	if err := host.Close(ctx, "window-99"); err == nil {
		t.Fatal("close of an unknown window succeeded")
	}
	if err := host.Picker(ctx, workbench.PickerRequest{}); err == nil {
		t.Fatal("a picker request with no session root succeeded")
	}
	if _, err := host.Open(ctx, workbench.WindowOptions{Kind: workbench.KindTerminal, Label: "x"}); err == nil {
		t.Fatal("a terminal window opened with no command")
	}
}

// A process that died without unlinking its socket would otherwise leave the
// next run unable to bind.
func TestServeControlReplacesAStaleSocket(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(socket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the socket outlived the process that served it")
	}
}

func TestProcessSocketOpensCanonicalLinearIssuesAndFocusesTheWindow(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	var offered, offeredPrompt string
	var focused int
	conflict := false
	server, err := serveControl(socket, windows, nil, controlHooks{
		linearIssue: func(ticket, prompt string) (string, error) {
			offered = ticket
			offeredPrompt = prompt
			if conflict {
				return "", ErrAssemblyDraftConflict
			}
			return assemblyOutcomeQueued, nil
		},
		focus: func() { focused++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	outcome, err := workbench.OpenLinearIssue(context.Background(), socket,
		"https://linear.app/lifesum/issue/lif-2841/title", "Linear's task")
	if err != nil || outcome != assemblyOutcomeQueued {
		t.Fatalf("OpenLinearIssue = %q, %v", outcome, err)
	}
	if offered != "https://linear.app/issue/LIF-2841" || offeredPrompt != "Linear's task" || focused != 1 {
		t.Fatalf("offered %q, prompt %q, and focused %d times", offered, offeredPrompt, focused)
	}

	conflict = true
	if _, err := workbench.OpenLinearIssue(context.Background(), socket, "LIF-2842", ""); err == nil {
		t.Fatal("draft conflict succeeded")
	}
	if focused != 2 {
		t.Fatalf("draft conflict focused %d times, want twice in all", focused)
	}
	before := offered
	if _, err := workbench.OpenLinearIssue(context.Background(), socket, "not-an-issue", ""); err == nil {
		t.Fatal("invalid Linear issue reached the process")
	}
	if offered != before || focused != 2 {
		t.Fatalf("invalid issue was offered as %q or focused %d times", offered, focused)
	}
}

func TestLinearIssueOpIsRefusedOnSessionSocketsAndWithoutItsPayload(t *testing.T) {
	windows, _ := testWindows(t)
	for _, tc := range []struct {
		name  string
		owner *sessionState
		req   workbench.Request
	}{
		{name: "session socket", owner: windows.shown(), req: workbench.Request{
			Op:          workbench.OpOpenLinearIssue,
			LinearIssue: &workbench.LinearIssueRequest{Ticket: "https://linear.app/issue/LIF-2841"},
		}},
		{name: "missing payload", req: workbench.Request{Op: workbench.OpOpenLinearIssue}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control := &control{windows: windows, owner: tc.owner, hooks: controlHooks{
				linearIssue: func(string, string) (string, error) { return assemblyOutcomeQueued, nil },
			}}
			if res := control.dispatch(tc.req); res.Error == "" {
				t.Fatalf("dispatch(%+v) = %+v, want a refusal", tc.req, res)
			}
		})
	}
}
