package desktop

import (
	"context"
	"errors"
	"github.com/kieranajp/qrouton/internal/assembly"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestProcessSocketOpensCanonicalTicketsAndFocusesTheWindow(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	var offered, offeredPrompt string
	var focused int
	conflict := false
	server, err := serveControl(socket, windows, nil, controlHooks{
		openTicket: func(ticket, prompt string) (string, error) {
			offered = ticket
			offeredPrompt = prompt
			if conflict {
				return "", assembly.ErrDraftConflict
			}
			return assembly.OutcomeQueued, nil
		},
		focus: func() { focused++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	outcome, err := workbench.OpenTicket(context.Background(), socket,
		"https://linear.app/lifesum/issue/lif-2841/title", "Linear's task")
	if err != nil || outcome != assembly.OutcomeQueued {
		t.Fatalf("OpenTicket = %q, %v", outcome, err)
	}
	if offered != "https://linear.app/issue/LIF-2841" || offeredPrompt != "Linear's task" || focused != 1 {
		t.Fatalf("offered %q, prompt %q, and focused %d times", offered, offeredPrompt, focused)
	}

	conflict = true
	if _, err := workbench.OpenTicket(context.Background(), socket, "LIF-2842", ""); err == nil {
		t.Fatal("draft conflict succeeded")
	}
	if focused != 2 {
		t.Fatalf("draft conflict focused %d times, want twice in all", focused)
	}
	before := offered
	if _, err := workbench.OpenTicket(context.Background(), socket, "not-an-issue", ""); err == nil {
		t.Fatal("invalid Linear issue reached the process")
	}
	if offered != before || focused != 2 {
		t.Fatalf("invalid issue was offered as %q or focused %d times", offered, focused)
	}
}

func TestOpenTicketIsRefusedOnSessionSocketsAndWithoutItsPayload(t *testing.T) {
	windows, _ := testWindows(t)
	for _, tc := range []struct {
		name  string
		owner *sessionState
		req   workbench.Request
	}{
		{name: "session socket", owner: windows.shown(), req: workbench.Request{
			Op:     workbench.OpOpenTicket,
			Ticket: &workbench.TicketRequest{URL: "https://linear.app/issue/LIF-2841"},
		}},
		{name: "missing payload", req: workbench.Request{Op: workbench.OpOpenTicket}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control := &control{windows: windows, owner: tc.owner, hooks: controlHooks{
				openTicket: func(string, string) (string, error) { return assembly.OutcomeQueued, nil },
			}}
			if res := control.dispatch(tc.req); res.Error == "" {
				t.Fatalf("dispatch(%+v) = %+v, want a refusal", tc.req, res)
			}
		})
	}
}

// The table is the only place an operation becomes servable, so a constant the
// port declares without an entry answers "unknown workbench operation".
func TestEveryDeclaredOperationHasAHandler(t *testing.T) {
	declared := declaredOperations(t)
	if len(declared) == 0 {
		t.Fatal("no operations found; the port's constants moved")
	}
	for op, name := range declared {
		if _, ok := handlers[op]; !ok {
			t.Errorf("workbench.%s (%q) has no handler", name, op)
		}
	}
	for op := range handlers {
		if _, ok := declared[op]; !ok {
			t.Errorf("handler %q is not an operation the port declares", op)
		}
	}
}

// declaredOperations answers the wire value of every Op constant the workbench
// port declares, against the identifier that named it.
func declaredOperations(t *testing.T) map[string]string {
	t.Helper()
	const dir = "../workbench"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	ops := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, ident := range value.Names {
					if !strings.HasPrefix(ident.Name, "Op") || i >= len(value.Values) {
						continue
					}
					literal, ok := value.Values[i].(*ast.BasicLit)
					if !ok || literal.Kind != token.STRING {
						continue
					}
					op, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatal(err)
					}
					ops[op] = ident.Name
				}
			}
		}
	}
	return ops
}

// Each operation's guards are declared beside it, so the refusal a malformed or
// misdirected request gets has to stay the one that names what it lacks.
func TestDispatchRefusesEachOperationWithItsOwnSentinel(t *testing.T) {
	windows, _ := testWindows(t)
	owner := windows.shown()
	options := &workbench.WindowOptions{Kind: workbench.KindDocument, Label: "◆ plan", Content: "one"}

	for _, tc := range []struct {
		name  string
		owner *sessionState
		req   workbench.Request
		want  error
	}{
		{name: "open without options", owner: owner,
			req: workbench.Request{Op: workbench.OpOpen}, want: ErrNoWindowOptions},
		{name: "open on a session-less socket",
			req: workbench.Request{Op: workbench.OpOpen, Options: options}, want: ErrNoSession},
		{name: "viewport on a session-less socket",
			req: workbench.Request{Op: workbench.OpViewport, ID: "window-1"}, want: ErrNoSession},
		{name: "picker without a root", owner: owner,
			req: workbench.Request{Op: workbench.OpPicker, Picker: &workbench.PickerRequest{}}, want: ErrNoSessionRoot},
		{name: "runner generation on a session-less socket",
			req: workbench.Request{Op: workbench.OpRunnerGeneration,
				RunnerGeneration: &workbench.RunnerGenerationRequest{Generation: 3}}, want: ErrNoSession},
		{name: "runner generation without a payload", owner: owner,
			req: workbench.Request{Op: workbench.OpRunnerGeneration}, want: ErrNoRunnerGeneration},
		{name: "runner generation at generation zero", owner: owner,
			req: workbench.Request{Op: workbench.OpRunnerGeneration,
				RunnerGeneration: &workbench.RunnerGenerationRequest{Provider: agentProviderClaude}},
			want: ErrNoRunnerGeneration},
		{name: "delegated lifecycle on a session-less socket",
			req: workbench.Request{Op: workbench.OpDelegatedLifecycle,
				Lifecycle: &workbench.DelegatedLifecycleRequest{Kind: workbench.LifecycleStart}}, want: ErrNoSession},
		{name: "delegated lifecycle without a payload", owner: owner,
			req: workbench.Request{Op: workbench.OpDelegatedLifecycle}, want: ErrNoDelegatedLifecycle},
		{name: "an operation nothing serves", owner: owner,
			req: workbench.Request{Op: "teleport"}, want: unknownOperation("teleport")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control := &control{windows: windows, owner: tc.owner}
			if res := control.dispatch(tc.req); res.Error != tc.want.Error() {
				t.Fatalf("dispatch(%+v) = %q, want %q", tc.req, res.Error, tc.want)
			}
		})
	}
}
