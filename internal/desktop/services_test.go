package desktop

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/lineartools"
	"github.com/kieranajp/qrouton/internal/placement"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/ticket"
	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
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
	return [][]adapterRow{ollamaRows(), sessionsRows(), termRows(), windowsRows(), bugReportRows(), chromeRows(),
		assemblyRows(), repositoriesRows(), orgsRows(), pickerRows(), settingsRows(), firstRunRows(), placementRows()}
}

func ollamaRows() []adapterRow {
	return []adapterRow{
		{"Ollama.Check", func(t *testing.T) {
			rec, conn := recorded(t)
			got, err := (&Ollama{ollama: qroutonv1.NewOllamaServiceClient(conn)}).Check()
			rec.reached(t, "Ollama.Check")
			if err != nil || !strings.Contains(string(got), `"model":"recorded"`) {
				t.Fatalf("Check answered %s, %v", got, err)
			}
		}},
		{"Ollama.Pull", func(t *testing.T) {
			rec, conn := recorded(t)
			if err := (&Ollama{ollama: qroutonv1.NewOllamaServiceClient(conn)}).Pull(); err != nil {
				t.Fatal(err)
			}
			rec.reached(t, "Ollama.Pull")
		}},
		{"Ollama.CancelPull", func(t *testing.T) {
			rec, conn := recorded(t)
			if err := (&Ollama{ollama: qroutonv1.NewOllamaServiceClient(conn)}).CancelPull(); err != nil {
				t.Fatal(err)
			}
			rec.reached(t, "Ollama.CancelPull")
		}},
	}
}

// Catches an adapter that hands the page the status wrapper, or drops the
// error, rather than the text the core refused with.
func TestAnOllamaErrorReachesThePageWithItsOwnText(t *testing.T) {
	rec, conn := recorded(t)
	rec.err = grpcstatus.Error(codes.Unimplemented, core.ErrNoRevealCommand.Error())
	adapter := &Ollama{ollama: qroutonv1.NewOllamaServiceClient(conn)}
	_, checkErr := adapter.Check()
	for name, err := range map[string]error{"Check": checkErr, "Pull": adapter.Pull(), "CancelPull": adapter.CancelPull()} {
		if err == nil || err.Error() != core.ErrNoRevealCommand.Error() || !errors.Is(err, core.ErrUnavailable) {
			t.Fatalf("%s returned %v, want the core's own refusal", name, err)
		}
	}
}

// Catches a desktop that serves the core but never says Hello, or drops the
// client id, which would refuse every call the page makes.
func TestTheOllamaAdapterReachesTheCoreAfterHello(t *testing.T) {
	opts, _ := testOptions(t)
	opts.Config = &config.Config{Root: opts.Root}
	options := coreOptions(opts, func() {})
	ollama := newAdapterOllama()
	defer close(ollama.released)
	options.Ollama = ollama
	in, err := startInProcess(core.New(options, newLocalClient(newFakeRenderer(), opts.Launcher, nil)), opts.Socket, "")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	got, err := (&Ollama{ollama: in.ollama}).Check()
	if err != nil || ollama.probes.Load() != 1 || !strings.Contains(string(got), vault.OllamaModel) {
		t.Fatalf("Check answered %s, %v after %d probes", got, err, ollama.probes.Load())
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

// assemblyCore is a core over an empty sessions root, with no launcher.
func assemblyCore(t *testing.T) (*core.Core, string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	return testWorkbench(t, newFakeRenderer(), &Options{Root: root, Config: &config.Config{Root: root}}), root
}

const offeredTicket = "https://linear.app/issue/LIF-2841"

// unslugged is a draft with a name and no repositories, which Check refuses
// for its repositories and CheckSlug passes unless its slug is taken.
var unslugged = core.DraftInput{Name: "Cleanup", Entropy: "4f3a", Prefix: "feat"}

func refusedFor(err error, field assembly.Field) bool {
	return err != nil && strings.HasPrefix(err.Error(), string(field)+":")
}

func assemblyRows() []adapterRow {
	return []adapterRow{
		{"Assembly.Prefixes", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			if got := (&Assembly{assembly: kernel.Assembly}).Prefixes(); len(got) == 0 || !slices.Equal(got, assembly.Prefixes()) {
				t.Fatalf("Prefixes = %v", got)
			}
		}},
		{"Assembly.Runners", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			if _, err := (&Assembly{assembly: kernel.Assembly}).Runners(); !errors.Is(err, core.ErrNoAgentCommand) {
				t.Fatalf("Runners with no launcher = %v, want ErrNoAgentCommand", err)
			}
		}},
		{"Assembly.Check", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			problems := (&Assembly{assembly: kernel.Assembly}).Check(unslugged)
			if len(problems) != 1 || problems[0].Field != assembly.FieldRepos {
				t.Fatalf("Check of a draft with no repositories = %+v", problems)
			}
		}},
		{"Assembly.CheckSlug", func(t *testing.T) {
			kernel, root := assemblyCore(t)
			if err := os.MkdirAll(filepath.Join(root, "cleanup-4f3a"), 0o755); err != nil {
				t.Fatal(err)
			}
			problems := (&Assembly{assembly: kernel.Assembly}).CheckSlug(unslugged)
			if len(problems) != 1 || problems[0].Field != assembly.FieldName {
				t.Fatalf("CheckSlug of a taken slug = %+v", problems)
			}
		}},
		{"Assembly.Preview", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			if got := (&Assembly{assembly: kernel.Assembly}).Preview(unslugged); got != "feat/cleanup-4f3a" {
				t.Fatalf("Preview = %q", got)
			}
		}},
		{"Assembly.Fetch", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			if _, err := (&Assembly{assembly: kernel.Assembly}).Fetch("not a ticket"); !errors.Is(err, ticket.ErrUnsupportedProvider) {
				t.Fatalf("Fetch of no ticket = %v, want ErrUnsupportedProvider", err)
			}
		}},
		{"Assembly.Pending", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			if _, err := kernel.Offer("lif-2841", ""); err != nil {
				t.Fatal(err)
			}
			if got := (&Assembly{assembly: kernel.Assembly}).Pending(); got != offeredTicket {
				t.Fatalf("Pending = %q", got)
			}
		}},
		{"Assembly.Begin", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			if _, err := kernel.Offer("lif-2841", ""); err != nil {
				t.Fatal(err)
			}
			seed := (&Assembly{assembly: kernel.Assembly}).Begin()
			if seed.Ticket != offeredTicket || seed.Generation == 0 || kernel.Assembly.Pending() != "" {
				t.Fatalf("Begin = %+v, pending %q", seed, kernel.Assembly.Pending())
			}
		}},
		{"Assembly.End", func(t *testing.T) {
			kernel, _ := assemblyCore(t)
			seed := kernel.Assembly.Begin()
			(&Assembly{assembly: kernel.Assembly}).End(seed.Generation)
			if outcome, err := kernel.Offer("lif-2841", ""); err != nil || outcome != assembly.OutcomeQueued {
				t.Fatalf("an offer after End = %q, %v", outcome, err)
			}
		}},
		{"Assembly.Create", func(t *testing.T) {
			kernel, root := assemblyCore(t)
			if err := (&Assembly{assembly: kernel.Assembly}).Create(unslugged); !refusedFor(err, assembly.FieldRepos) {
				t.Fatalf("Create with no editing repository = %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "cleanup-4f3a")); !os.IsNotExist(err) {
				t.Fatalf("a refused Create left a session directory: %v", err)
			}
		}},
	}
}

// repositoriesCore is a core whose cached list holds acme/api, on a machine
// with no GitHub credential, so nothing reaches the network.
func repositoriesCore(t *testing.T) (*core.Core, *fakeRenderer) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	cfg := &config.Config{Root: t.TempDir(), Orgs: []string{"acme"}}
	if err := github.WriteRepoCache(cfg.Orgs, []github.Repo{{Org: "acme", Name: "api", DefaultBranch: "main"}}); err != nil {
		t.Fatal(err)
	}
	r := newFakeRenderer()
	return testWorkbench(t, r, &Options{Root: cfg.Root, Config: cfg}), r
}

func repositoriesRows() []adapterRow {
	return []adapterRow{
		{"Repositories.Cached", func(t *testing.T) {
			kernel, _ := repositoriesCore(t)
			if got := (&Repositories{repos: kernel.Repositories}).Cached(); len(got) != 1 || got[0].ID() != "acme/api" {
				t.Fatalf("Cached = %+v", got)
			}
		}},
		{"Repositories.Select", func(t *testing.T) {
			kernel, _ := repositoriesCore(t)
			got := (&Repositories{repos: kernel.Repositories}).Select([]core.RepoPick{
				{ID: "acme/api", Role: "editing", Base: "main"}, {ID: "acme/gone", Role: "editing"}})
			if len(got) != 1 || got[0].Repo.ID() != "acme/api" || got[0].Base != "" {
				t.Fatalf("Select = %+v", got)
			}
		}},
		{"Repositories.Branches", func(t *testing.T) {
			kernel, _ := repositoriesCore(t)
			got := (&Repositories{repos: kernel.Repositories}).Branches("acme/api")
			if got.Default != "main" || !slices.Equal(got.Branches, []string{"main"}) || got.Error == "" {
				t.Fatalf("Branches with no credential = %+v", got)
			}
		}},
		{"Repositories.Refresh", func(t *testing.T) {
			kernel, r := repositoriesCore(t)
			if gen := (&Repositories{repos: kernel.Repositories}).Refresh(); gen != 1 {
				t.Fatalf("Refresh answered generation %d", gen)
			}
			waitFor(t, "the refresh to complete", func() bool {
				r.mu.Lock()
				payload := r.events["repos:refresh"]
				r.mu.Unlock()
				encoded, _ := json.Marshal(payload)
				return strings.Contains(string(encoded), `"state":"complete"`)
			})
		}},
	}
}

func orgsRows() []adapterRow {
	return []adapterRow{
		{"Orgs.List", func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			cfg := &config.Config{Root: t.TempDir(), Orgs: []string{"acme", "beta"}}
			kernel := testWorkbench(t, newFakeRenderer(), &Options{Root: cfg.Root, Config: cfg})
			if got := (&Orgs{orgs: kernel.Orgs}).List(); !slices.Equal(got, cfg.Orgs) {
				t.Fatalf("List = %v", got)
			}
		}},
	}
}

func pickerRows() []adapterRow {
	return []adapterRow{
		{"Picker.Escalate", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			if err := (&Picker{picker: a.kernel.Picker}).Escalate(a.slug()); err != nil {
				t.Fatal(err)
			}
			if a.kernel.Sessions.Current().PendingPicker() == nil {
				t.Fatal("Escalate queued no picker")
			}
		}},
		{"Picker.Load", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			if err := session.WriteManifest(a.opts.SessionRoot, session.Manifest{Slug: a.slug(), Mode: session.ModeAssistant,
				Repos: []session.ManifestRepo{{Org: "acme", Name: "x", Role: session.RepoRoleEditing, Branch: "feat/x"}}}); err != nil {
				t.Fatal(err)
			}
			fields, err := (&Picker{picker: a.kernel.Picker}).Load(a.slug())
			if err != nil || fields.Branch != "feat/x" || len(fields.Repos) != 1 ||
				fields.Repos[0].ID != "acme/x" || !fields.Repos[0].Locked {
				t.Fatalf("Load of a session holding acme/x = %+v, %v", fields, err)
			}
		}},
		{"Picker.Confirm", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			if err := (&Picker{picker: a.kernel.Picker}).Confirm(a.slug(), core.PickerInput{}); !refusedFor(err, assembly.FieldRepos) {
				t.Fatalf("confirming no editing repository = %v", err)
			}
		}},
		{"Picker.Cancel", func(t *testing.T) {
			a := newAdapterWorkbench(t)
			if err := a.kernel.Picker.Escalate(a.slug()); err != nil {
				t.Fatal(err)
			}
			if err := (&Picker{picker: a.kernel.Picker}).Cancel(a.slug()); err != nil {
				t.Fatal(err)
			}
			if a.kernel.Sessions.Current().PendingPicker() != nil {
				t.Fatal("Cancel left the picker pending")
			}
		}},
	}
}

// settingsCore is a core over a config.json of its own, on a machine whose gh is
// signed out and appends to the returned marker each time it runs.
func settingsCore(t *testing.T, opts Options) (*core.Core, *config.Config, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "")
	bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "gh")
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/sh\necho ran >> "+marker+"\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	cfg := &config.Config{Root: t.TempDir(), Orgs: []string{"acme"}, Editor: []string{"code", "--wait"}}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	opts.Root, opts.Config = cfg.Root, cfg
	if opts.Launcher == nil {
		opts.Launcher = newStubBoot()
	}
	return testWorkbench(t, newFakeRenderer(), &opts), cfg, marker
}

func settingsRows() []adapterRow {
	return []adapterRow{
		{"Settings.Load", func(t *testing.T) {
			kernel, _, _ := settingsCore(t, Options{})
			if got := (&Settings{settings: kernel.Settings}).Load(); got.Editor != "code --wait" || len(got.UIScaleSteps) == 0 {
				t.Fatalf("Load = %+v", got)
			}
		}},
		{"Settings.Save", func(t *testing.T) {
			kernel, cfg, _ := settingsCore(t, Options{})
			_, err := (&Settings{settings: kernel.Settings}).Save(core.SettingsInput{Orgs: []string{"acme"}, Root: cfg.Root, Editor: "vim"})
			file, readErr := config.ReadFile()
			if err != nil || readErr != nil || !slices.Equal(file.Editor, []string{"vim"}) {
				t.Fatalf("Save = %v, config.json editor %q (%v)", err, file.Editor, readErr)
			}
		}},
		{"Settings.AdjustUIScale", func(t *testing.T) {
			kernel, cfg, _ := settingsCore(t, Options{})
			if got, err := (&Settings{settings: kernel.Settings}).AdjustUIScale("in"); err != nil || got != 110 || cfg.EffectiveUIScale() != 110 {
				t.Fatalf("AdjustUIScale(in) = %d, %v; live scale %d", got, err, cfg.EffectiveUIScale())
			}
		}},
		{"Settings.GitHubStatus", func(t *testing.T) {
			kernel, _, marker := settingsCore(t, Options{})
			got := (&Settings{settings: kernel.Settings}).GitHubStatus()
			if got.State != github.CredentialSignedOut || got.Variable != github.TokenEnvVar() {
				t.Fatalf("GitHubStatus with gh signed out = %+v", got)
			}
			markedFile(t, marker, "ran")
		}},
		{"Settings.Relaunch", func(t *testing.T) {
			relauncher := &recordingRelauncher{}
			kernel, _, _ := settingsCore(t, Options{Relauncher: relauncher})
			if err := (&Settings{settings: kernel.Settings}).Relaunch(); err != nil || relauncher.calls != 1 {
				t.Fatalf("Relaunch = %v after %d relaunches", err, relauncher.calls)
			}
		}},
		{"Settings.OpenConfig", func(t *testing.T) {
			boot := newStubBoot()
			opened := filepath.Join(t.TempDir(), "opened")
			boot.openText = func(path string) []string {
				return []string{"/bin/sh", "-c", `printf %s "$1" > "$2"`, "sh", path, opened}
			}
			kernel, _, _ := settingsCore(t, Options{Launcher: boot})
			if err := (&Settings{settings: kernel.Settings}).OpenConfig(); err != nil {
				t.Fatal(err)
			}
			markedFile(t, opened, config.Path())
		}},
		{"Settings.InstallLinear", func(t *testing.T) {
			kernel, _, _ := settingsCore(t, Options{LinearCommand: []string{"/bin/qrouton", "--linear-issue"}})
			view, err := (&Settings{settings: kernel.Settings}).InstallLinear()
			if err != nil || view.Snippet == "" || view.Replaces {
				t.Fatalf("InstallLinear = %+v, %v", view, err)
			}
			markedFile(t, config.ExpandHome(lineartools.ConfigPath), "--linear-issue")
		}},
	}
}

func firstRunRows() []adapterRow {
	return []adapterRow{
		{"FirstRun.Login", func(t *testing.T) {
			kernel, _, marker := settingsCore(t, Options{})
			if got := (&FirstRun{firstRun: kernel.FirstRun}).Login(); got != "" {
				t.Fatalf("Login with gh signed out = %q", got)
			}
			markedFile(t, marker, "ran")
		}},
		{"FirstRun.ChooseRoot", func(t *testing.T) {
			kernel := core.New(coreOptions(Options{Config: &config.Config{}}, nil),
				newLocalClient(newFakeRenderer(), newStubBoot(), func() (string, error) { return "/picked", nil }))
			t.Cleanup(kernel.Shutdown)
			if got, err := (&FirstRun{firstRun: kernel.FirstRun}).ChooseRoot(); err != nil || got != "/picked" {
				t.Fatalf("ChooseRoot = %q, %v", got, err)
			}
		}},
		{"FirstRun.ThoughtsDefault", func(t *testing.T) {
			kernel, _, _ := settingsCore(t, Options{})
			want := config.DefaultThoughtsRoot("/srv/sessions")
			if got := (&FirstRun{firstRun: kernel.FirstRun}).ThoughtsDefault("/srv/sessions"); got != want {
				t.Fatalf("ThoughtsDefault = %q, want %q", got, want)
			}
		}},
		{"FirstRun.Save", func(t *testing.T) {
			kernel, cfg, _ := settingsCore(t, Options{})
			result, err := (&FirstRun{firstRun: kernel.FirstRun}).Save(core.FirstRunInput{Orgs: []string{"acme"}, Root: cfg.Root})
			if err != nil || result.Relaunching || !cfg.Snapshot().Welcomed {
				t.Fatalf("Save = %+v, %v; welcomed %v", result, err, cfg.Snapshot().Welcomed)
			}
			// The welcome starts a placement check; it must land in this test's config.json, not the user's.
			kernel.Shutdown()
			if file, err := config.ReadFile(); err != nil || file.ThoughtsPlacement == "" {
				t.Fatalf("placement check unfinished at shutdown: %+v, %v", file, err)
			}
		}},
	}
}

func placementRows() []adapterRow {
	return []adapterRow{
		{"Placement.Check", func(t *testing.T) {
			kernel, _, _ := settingsCore(t, Options{})
			if err := (&Placement{placement: kernel.Placement}).Check(); err != nil || !kernel.Sessions.Placing() {
				t.Fatalf("Check = %v, placing %v", err, kernel.Sessions.Placing())
			}
		}},
		{"Placement.Report", func(t *testing.T) {
			kernel, _, _ := settingsCore(t, Options{})
			if got := (&Placement{placement: kernel.Placement}).Report(); got.Misplaced == nil || got.Skipped == nil || got.Pending == nil {
				t.Fatalf("Report = %+v, want empty lists the page can range over", got)
			}
		}},
		{"Placement.Move", func(t *testing.T) {
			kernel, _, _ := settingsCore(t, Options{})
			got := (&Placement{placement: kernel.Placement}).Move([]string{"ghost"})
			if len(got) != 1 || got[0].Slug != "ghost" || got[0].Result != placement.ResultRefused {
				t.Fatalf("Move of no session = %+v", got)
			}
		}},
		{"Placement.Close", func(t *testing.T) {
			kernel, _, _ := settingsCore(t, Options{})
			if err := kernel.Placement.Check(); err != nil {
				t.Fatal(err)
			}
			if err := (&Placement{placement: kernel.Placement}).Close(); err != nil || kernel.Sessions.Placing() {
				t.Fatalf("Close = %v, placing %v", err, kernel.Sessions.Placing())
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
