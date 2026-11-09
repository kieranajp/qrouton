package desktop

import (
	"context"
	"encoding/base64"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// adapterOllama records which core calls reached the client behind it.
type adapterOllama struct {
	probes   atomic.Int32
	pulled   chan struct{}
	stopped  chan struct{}
	released chan struct{}
}

func newAdapterOllama() *adapterOllama {
	return &adapterOllama{pulled: make(chan struct{}, 1), stopped: make(chan struct{}, 1), released: make(chan struct{})}
}

func (a *adapterOllama) Model(context.Context) (vault.Model, error) {
	a.probes.Add(1)
	return vault.Model{}, vault.ErrModelMissing
}

func (a *adapterOllama) Pull(ctx context.Context, _ func(vault.PullProgress)) error {
	a.pulled <- struct{}{}
	select {
	case <-ctx.Done():
		a.stopped <- struct{}{}
	case <-a.released:
	}
	return ctx.Err()
}

func arrives(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal(what)
	}
}

// Catches an adapter method that forwards to the wrong core method, or to none.
// Each bound method gets a row that observes an effect only its own core method has.
func TestEachAdapterMethodReachesItsOwnCoreMethod(t *testing.T) {
	for _, group := range adapterRows() {
		for _, row := range group {
			t.Run(row.method, row.run)
		}
	}
}

func adapterRows() [][]adapterRow {
	return [][]adapterRow{ollamaRows(), sessionsRows(), termRows(), windowsRows(), bugReportRows(), chromeRows()}
}

func ollamaRows() []adapterRow {
	return []adapterRow{
		{"Ollama.Check", func(t *testing.T) {
			client := newAdapterOllama()
			adapter := &Ollama{ollama: core.NewOllama(client, func(string, any) {})}
			if got := adapter.Check(); client.probes.Load() != 1 || got.Model != vault.OllamaModel {
				t.Fatalf("Check answered %+v after %d probes", got, client.probes.Load())
			}
		}},
		{"Ollama.Pull", func(t *testing.T) {
			client := newAdapterOllama()
			defer close(client.released)
			adapter := &Ollama{ollama: core.NewOllama(client, func(string, any) {})}
			adapter.Pull()
			arrives(t, client.pulled, "Pull started no pull")
		}},
		{"Ollama.CancelPull", func(t *testing.T) {
			client := newAdapterOllama()
			defer close(client.released)
			ollama := core.NewOllama(client, func(string, any) {})
			adapter := &Ollama{ollama: ollama}
			ollama.Pull()
			arrives(t, client.pulled, "the pull never started")
			adapter.CancelPull()
			arrives(t, client.stopped, "CancelPull left the pull running")
		}},
	}
}

// adapterWorkbench is a running core opened on one session, with the session's
// own window host and a marker file its commands can write to.
type adapterWorkbench struct {
	kernel *core.Core
	r      *fakeRenderer
	boot   *stubBoot
	opts   Options
	host   workbench.WindowHost
	marker string
}

func newAdapterWorkbench(t *testing.T, argv ...string) *adapterWorkbench {
	t.Helper()
	opts, boot := testOptions(t)
	if len(argv) > 0 {
		boot.argv = argv
	}
	marker := filepath.Join(t.TempDir(), "marker")
	boot.reveal = func(path string) []string {
		return []string{"/bin/sh", "-c", `printf %s "$1" > "$2"`, "sh", path, marker}
	}
	boot.copyPic = func(path string) []string {
		return []string{"/bin/sh", "-c", `printf image > "$1"`, "sh", marker}
	}
	r := newFakeRenderer()
	kernel := testWorkbench(t, r, &opts)
	if err := kernel.Start(opts.Socket); err != nil {
		t.Fatal(err)
	}
	if err := kernel.OpenSession(opts.SessionRoot, false); err != nil {
		t.Fatal(err)
	}
	socket := boot.socketOf(t, opts.SessionRoot)
	host := (workbench.Handle{Socket: socket, SessionRoot: opts.SessionRoot}).WindowHost()
	return &adapterWorkbench{kernel: kernel, r: r, boot: boot, opts: opts, host: host, marker: marker}
}

func (a *adapterWorkbench) slug() string { return filepath.Base(a.opts.SessionRoot) }

// marked waits for a command to write want to the marker.
func (a *adapterWorkbench) marked(t *testing.T, want string) {
	t.Helper()
	markedFile(t, a.marker, want)
}

func markedFile(t *testing.T, path, want string) {
	t.Helper()
	waitFor(t, path+" to read "+want, func() bool {
		got, err := os.ReadFile(path)
		return err == nil && strings.Contains(string(got), want)
	})
}

// conversation is the shown session's terminal id, which the chrome carries.
func (a *adapterWorkbench) conversation(t *testing.T) string {
	t.Helper()
	return chromeOf(t, a.kernel, func(f status.Fields) bool { return f.Terminal != "" }).Terminal
}

func (a *adapterWorkbench) open(t *testing.T, opts workbench.WindowOptions) string {
	t.Helper()
	id, err := a.host.Open(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// document opens a session file as a Markdown tab.
func (a *adapterWorkbench) document(t *testing.T, source, text string) string {
	t.Helper()
	path := filepath.Join(a.opts.SessionRoot, source)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return a.open(t, workbench.WindowOptions{Kind: workbench.KindDocument, Label: filepath.Base(source),
		Source: source, Content: text, Format: workbench.FormatMarkdown})
}

// shell opens the user shell the + button does, running script with the marker
// as $1. The page starts it, so nothing runs until Start.
func (a *adapterWorkbench) shell(t *testing.T, script string) string {
	t.Helper()
	a.boot.shell = func(string) []string { return []string{"/bin/sh", "-c", script, "sh", a.marker} }
	id, err := a.kernel.Windows.OpenShell()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// terminal opens a tab running script, which may write to the marker as $1.
func (a *adapterWorkbench) terminal(t *testing.T, script string) string {
	t.Helper()
	return a.open(t, workbench.WindowOptions{Kind: workbench.KindTerminal, Label: "tab", Cwd: a.opts.SessionRoot,
		Command: []string{"/bin/sh", "-c", script, "sh", a.marker}})
}

func (a *adapterWorkbench) tabs(slug string) []string {
	var ids []string
	for _, tab := range a.kernel.Windows.Surfaces(slug).Tabs {
		ids = append(ids, tab.ID)
	}
	return ids
}

func (b *stubBoot) socketOf(t *testing.T, sessionRoot string) string {
	t.Helper()
	var socket string
	waitFor(t, "the session's own control socket", func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		socket = b.sockets[sessionRoot]
		return socket != "" && workbench.Answered(socket)
	})
	return socket
}

func encoded(text string) string { return base64.StdEncoding.EncodeToString([]byte(text)) }

const (
	catToMarker  = `cat > "$1"`
	sizeToMarker = `while :; do stty size > "$1"; sleep 0.05; done`
)

func sessionsRows() []adapterRow {
	return []adapterRow{
		{"Sessions.Show", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			sessionDir(t, a.opts.Root, "kraken")
			if err := (&Sessions{sessions: a.kernel.Sessions}).Show("kraken"); err != nil {
				t.Fatal(err)
			}
			if got := a.kernel.Sessions.Current().Slug(); got != "kraken" {
				t.Fatalf("Show left %q on screen", got)
			}
		}},
		{"Sessions.Reload", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			before, _ := a.boot.counts()
			if err := (&Sessions{sessions: a.kernel.Sessions}).Reload(a.slug()); err != nil {
				t.Fatal(err)
			}
			if after, _ := a.boot.counts(); after != before+1 || !a.boot.resumed(a.opts.SessionRoot) {
				t.Fatalf("Reload of a live session asked for %d supervisors, resumed %v", after-before, a.boot.resumed(a.opts.SessionRoot))
			}
		}},
		{"Sessions.Unload", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			if err := (&Sessions{sessions: a.kernel.Sessions}).Unload(a.slug()); err != nil {
				t.Fatal(err)
			}
			if a.kernel.Sessions.BySlug(a.slug()) != nil {
				t.Fatal("Unload left the session registered")
			}
			if _, err := os.Stat(a.opts.SessionRoot); err != nil {
				t.Fatalf("Unload took the session off disk: %v", err)
			}
		}},
		{"Sessions.Reorder", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			sessionDir(t, a.opts.Root, "kraken")
			rail := func(f status.Fields) []string {
				var slugs []string
				for _, row := range f.Sessions {
					slugs = append(slugs, row.Slug)
				}
				return slugs
			}
			first := rail(chromeOf(t, a.kernel, func(f status.Fields) bool { return len(f.Sessions) == 2 }))
			if err := (&Sessions{sessions: a.kernel.Sessions}).Reorder(first[1], first[0]); err != nil {
				t.Fatal(err)
			}
			a.kernel.Sessions.Touch()
			chromeOf(t, a.kernel, func(f status.Fields) bool {
				got := rail(f)
				return len(got) == 2 && got[0] == first[1] && got[1] == first[0]
			})
		}},
		{"Sessions.CycleSticker", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			sticker, err := (&Sessions{sessions: a.kernel.Sessions}).CycleSticker(a.slug())
			if err != nil {
				t.Fatal(err)
			}
			if m, err := session.Load(a.opts.SessionRoot); err != nil || m.Sticker != sticker {
				t.Fatalf("CycleSticker answered %q, manifest holds %q (%v)", sticker, m.Sticker, err)
			}
		}},
		{"Sessions.Reveal", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			if err := (&Sessions{sessions: a.kernel.Sessions}).Reveal(a.slug()); err != nil {
				t.Fatal(err)
			}
			a.marked(t, a.opts.SessionRoot)
		}},
		{"Sessions.RevealPath", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			inside := filepath.Join(a.opts.SessionRoot, "thoughts")
			if err := os.MkdirAll(inside, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := (&Sessions{sessions: a.kernel.Sessions}).RevealPath(a.slug(), inside); err != nil {
				t.Fatal(err)
			}
			a.marked(t, inside)
		}},
		{"Sessions.CopyAsset", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			shot := filepath.Join(a.opts.SessionRoot, "shot.png")
			if err := os.WriteFile(shot, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := (&Sessions{sessions: a.kernel.Sessions}).CopyAsset(a.slug(), shot); err != nil {
				t.Fatal(err)
			}
			a.marked(t, "image")
		}},
		{"Sessions.Uncommitted", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			dirty, err := (&Sessions{sessions: a.kernel.Sessions}).Uncommitted(a.slug())
			if err != nil || len(dirty) != 0 {
				t.Fatalf("a session holding no repositories reported %v, %v", dirty, err)
			}
			if _, err := (&Sessions{sessions: a.kernel.Sessions}).Uncommitted("kraken"); err == nil {
				t.Fatal("Uncommitted answered for a session that does not exist")
			}
		}},
		{"Sessions.Cleanup", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			if err := (&Sessions{sessions: a.kernel.Sessions}).Cleanup(a.slug()); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(a.opts.SessionRoot); !os.IsNotExist(err) {
				t.Fatalf("Cleanup left the session on disk: %v", err)
			}
		}},
	}
}

func termRows() []adapterRow {
	return []adapterRow{
		{"Term.Start", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.conversation(t)
			if err := a.kernel.Term.Write(id, encoded("x")); !errors.Is(err, core.ErrTerminalNotStarted) {
				t.Fatalf("a write before Start = %v", err)
			}
			if err := (&Term{term: a.kernel.Term}).Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			if err := a.kernel.Term.Write(id, encoded("x")); err != nil {
				t.Fatalf("a write after Start = %v", err)
			}
		}},
		{"Term.Write", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "conversation")
			b := newAdapterWorkbench(t, "/bin/sh", "-c", catToMarker, "sh", marker)
			id := b.conversation(t)
			if err := b.kernel.Term.Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			if err := (&Term{term: b.kernel.Term}).Write(id, encoded("hello\n")); err != nil {
				t.Fatal(err)
			}
			markedFile(t, marker, "hello")
		}},
		{"Term.Resize", func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "conversation")
			b := newAdapterWorkbench(t, "/bin/sh", "-c", sizeToMarker, "sh", marker)
			id := b.conversation(t)
			if err := b.kernel.Term.Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			if err := (&Term{term: b.kernel.Term}).Resize(id, 132, 50); err != nil {
				t.Fatal(err)
			}
			markedFile(t, marker, "50 132")
		}},
	}
}

func windowsRows() []adapterRow {
	return []adapterRow{
		{"Windows.OpenShell", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			a.boot.shell = func(string) []string { return []string{"/bin/cat"} }
			id, err := (&Windows{windows: a.kernel.Windows}).OpenShell()
			if err != nil || !slices.Contains(a.tabs(a.slug()), id) {
				t.Fatalf("OpenShell = %q, %v; tabs %v", id, err, a.tabs(a.slug()))
			}
		}},
		{"Windows.OpenDocument", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			_, err := (&Windows{windows: a.kernel.Windows}).OpenDocument("thoughts/notes.md")
			if !errors.Is(err, core.ErrNoEditorCommand) {
				t.Fatalf("OpenDocument with no editor = %v", err)
			}
		}},
		{"Windows.Select", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			first := a.document(t, "one.md", "# one")
			a.document(t, "two.md", "# two")
			if err := (&Windows{windows: a.kernel.Windows}).Select(a.slug(), first); err != nil {
				t.Fatal(err)
			}
			if got := a.kernel.Windows.Surfaces(a.slug()).Selected; got != first {
				t.Fatalf("Select left %q selected", got)
			}
		}},
		{"Windows.Reorder", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			a.document(t, "one.md", "# one")
			last := a.document(t, "two.md", "# two")
			if err := (&Windows{windows: a.kernel.Windows}).Reorder(a.slug(), last, 1); err != nil {
				t.Fatal(err)
			}
			if tabs := a.tabs(a.slug()); len(tabs) != 3 || tabs[1] != last {
				t.Fatalf("Reorder left the tabs %v", tabs)
			}
		}},
		{"Windows.Start", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.shell(t, `printf started > "$1"; exec cat`)
			if _, err := os.Stat(a.marker); !os.IsNotExist(err) {
				t.Fatalf("the shell ran before the page started it: %v", err)
			}
			if err := (&Windows{windows: a.kernel.Windows}).Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			a.marked(t, "started")
		}},
		{"Windows.Write", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.terminal(t, catToMarker)
			if err := a.kernel.Windows.Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			if err := (&Windows{windows: a.kernel.Windows}).Write(id, encoded("typed\n")); err != nil {
				t.Fatal(err)
			}
			a.marked(t, "typed")
		}},
		{"Windows.Resize", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.shell(t, `while :; do stty size >> "$1"; sleep 0.05; done`)
			adapter := &Windows{windows: a.kernel.Windows}
			if err := adapter.Resize(id, 100, 40); err != nil {
				t.Fatalf("resizing a shell the page has not started = %v", err)
			}
			// A started shell writes its size within milliseconds; this only bounds how
			// long a wrongly started one gets to show itself.
			time.Sleep(200 * time.Millisecond)
			if _, err := os.Stat(a.marker); !os.IsNotExist(err) {
				t.Fatalf("a resize started the shell: %v", err)
			}
			if err := a.kernel.Windows.Start(id, 80, 24); err != nil {
				t.Fatal(err)
			}
			a.marked(t, "24 80")
			if err := adapter.Resize(id, 132, 50); err != nil {
				t.Fatal(err)
			}
			a.marked(t, "50 132")
		}},
		{"Windows.Content", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.document(t, "one.md", "# only here")
			page, err := (&Windows{windows: a.kernel.Windows}).Content(id)
			if err != nil || !strings.Contains(page.Text, "only here") {
				t.Fatalf("Content = %+v, %v", page, err)
			}
		}},
		{"Windows.RenderDiagrams", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.document(t, "one.md", "# no diagrams")
			drawn, err := (&Windows{windows: a.kernel.Windows}).RenderDiagrams(id, 1)
			if err != nil || len(drawn) != 0 {
				t.Fatalf("RenderDiagrams of a page with none = %v, %v", drawn, err)
			}
		}},
		{"Windows.ReportViewport", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.terminal(t, "exec cat")
			if err := (&Windows{windows: a.kernel.Windows}).ReportViewport(id, core.ViewportReport{}); !errors.Is(err, core.ErrNoViewport) {
				t.Fatalf("a terminal's viewport report = %v, want ErrNoViewport", err)
			}
		}},
		{"Windows.Close", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.document(t, "one.md", "# one")
			if err := (&Windows{windows: a.kernel.Windows}).Close(id); err != nil {
				t.Fatal(err)
			}
			if slices.Contains(a.tabs(a.slug()), id) {
				t.Fatal("Close left the tab open")
			}
		}},
		{"Windows.Surfaces", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.document(t, "one.md", "# one")
			got := (&Windows{windows: a.kernel.Windows}).Surfaces(a.slug())
			if got.Session != a.slug() || len(got.Tabs) != 2 || got.Tabs[1].ID != id {
				t.Fatalf("Surfaces = %+v", got)
			}
		}},
		{"Windows.FocusAsset", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			for _, name := range []string{"a.png", "b.png"} {
				if err := os.WriteFile(filepath.Join(a.opts.SessionRoot, name), []byte(name), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			id := a.open(t, workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatAssets,
				Assets: []workbench.AssetRef{{Source: "a.png"}, {Source: "b.png"}}})
			got, err := (&Windows{windows: a.kernel.Windows}).FocusAsset(a.slug(), id, 2)
			if err != nil || got.CurrentIndex != 2 {
				t.Fatalf("FocusAsset = %+v, %v", got, err)
			}
		}},
		{"Windows.SaveAnswers", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.document(t, "thoughts/shared/specs/S1-x.md", "# spec")
			if _, err := (&Windows{windows: a.kernel.Windows}).SaveAnswers(id, "stale", "# answered"); !errors.Is(err, core.ErrDocumentChanged) {
				t.Fatalf("a save against a stale hash = %v, want ErrDocumentChanged", err)
			}
		}},
		{"Windows.SendSpecAnswers", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.document(t, "thoughts/shared/audits/A1-x.md", "# audit")
			if err := (&Windows{windows: a.kernel.Windows}).SendSpecAnswers(id); !errors.Is(err, core.ErrWrongSend) {
				t.Fatalf("sending spec answers from an audit = %v, want ErrWrongSend", err)
			}
		}},
		{"Windows.SendAuditTriage", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			id := a.document(t, "thoughts/shared/specs/S1-x.md", "# spec")
			if err := (&Windows{windows: a.kernel.Windows}).SendAuditTriage(id); !errors.Is(err, core.ErrWrongSend) {
				t.Fatalf("sending audit triage from a spec = %v, want ErrWrongSend", err)
			}
		}},
	}
}

// queueBugReport asks for a report over the session's socket, as the agent does.
func (a *adapterWorkbench) queueBugReport(t *testing.T, id string) {
	t.Helper()
	reports := a.host.(workbench.BugReportHost)
	if _, err := reports.QueueBugReport(context.Background(), workbench.BugReportRequest{
		ID: id, Title: "title " + id, Body: "body", Deadline: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
}

func bugReportRows() []adapterRow {
	return []adapterRow{
		{"BugReports.Load", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			a.queueBugReport(t, "one")
			preview, err := (&BugReports{reports: a.kernel.BugReports}).Load(a.slug(), "one")
			if err != nil || preview.Title != "title one" {
				t.Fatalf("Load = %+v, %v", preview, err)
			}
		}},
		{"BugReports.Cancel", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			a.queueBugReport(t, "one")
			report, err := (&BugReports{reports: a.kernel.BugReports}).Cancel(a.slug(), "one")
			if err != nil || report.Status != workbench.BugReportCancelled {
				t.Fatalf("Cancel = %+v, %v", report, err)
			}
		}},
		{"BugReports.Confirm", func(t *testing.T) {
			// A report superseded by a later one is still cancellable but no
			// longer confirmable, which is what tells the two forwards apart.
			a := newAdapterWorkbench(t)
			a.queueBugReport(t, "one")
			if _, err := a.kernel.BugReports.Cancel(a.slug(), "one"); err != nil {
				t.Fatal(err)
			}
			a.queueBugReport(t, "two")
			if _, err := (&BugReports{reports: a.kernel.BugReports}).Confirm(a.slug(), "one"); !errors.Is(err, core.ErrBugReportStale) {
				t.Fatalf("confirming a superseded report = %v, want ErrBugReportStale", err)
			}
		}},
	}
}

func chromeRows() []adapterRow {
	return []adapterRow{
		{"Chrome.AgentTree", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			kraken := sessionDir(t, a.opts.Root, "kraken")
			if err := session.WriteManifest(kraken, session.Manifest{Slug: "kraken", Mode: session.ModeAssistant, Runner: "codex"}); err != nil {
				t.Fatal(err)
			}
			if got := (&Chrome{chrome: a.kernel.Chrome}).AgentTree("kraken"); got.Provider != "codex" {
				t.Fatalf("AgentTree of an unbooted session = %+v, want its manifest's runner", got)
			}
		}},
		{"Chrome.Snapshot", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			want := chromeOf(t, a.kernel, func(f status.Fields) bool { return f.Slug == a.slug() })
			if got := (&Chrome{chrome: a.kernel.Chrome}).Snapshot(); got.Slug != want.Slug {
				t.Fatalf("Snapshot = %q, want the published %q", got.Slug, want.Slug)
			}
		}},
	}
}

type adapterRow struct {
	method string
	run    func(t *testing.T)
}

// Catches an adapter method added to services.go without a row above, which
// would leave its forward unchecked.
func TestEveryAdapterMethodHasARow(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "services.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]bool{}
	for _, group := range adapterRows() {
		for _, row := range group {
			rows[row.method] = true
		}
	}
	found := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || !fn.Name.IsExported() {
			continue
		}
		receiver := fn.Recv.List[0].Type
		if star, ok := receiver.(*ast.StarExpr); ok {
			receiver = star.X
		}
		name := receiver.(*ast.Ident).Name + "." + fn.Name.Name
		found++
		if !rows[name] {
			t.Errorf("%s has no row in TestEachAdapterMethodReachesItsOwnCoreMethod", name)
		}
	}
	if found != len(rows) {
		t.Errorf("services.go declares %d adapter methods and there are %d rows", found, len(rows))
	}
}
