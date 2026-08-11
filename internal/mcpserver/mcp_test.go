package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var testEditor = launch.EditorCommand{Argv: []string{"vi", "+{line}", "{path}"}, Template: true}

type readCall struct {
	id   string
	full bool
}

// fakeHost stands in for the desktop process: it records what the window
// manager asked of it, and lets a test script the answers, failures included.
type fakeHost struct {
	mu sync.Mutex

	opens  []workbench.WindowOptions
	ids    []string
	closes []string
	reads  []readCall
	checks []string
	lists  int

	live map[string]bool

	text      string
	readErr   error
	existsErr error
	listIDs   []string
	listErr   error
}

func (h *fakeHost) Open(_ context.Context, opts workbench.WindowOptions) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := fmt.Sprintf("window-%d", len(h.opens)+1)
	h.opens = append(h.opens, opts)
	h.ids = append(h.ids, id)
	if h.live == nil {
		h.live = map[string]bool{}
	}
	h.live[id] = true
	return id, nil
}

func (h *fakeHost) Close(_ context.Context, id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closes = append(h.closes, id)
	delete(h.live, id)
	return nil
}

func (h *fakeHost) Read(_ context.Context, id string, full bool) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reads = append(h.reads, readCall{id: id, full: full})
	return h.text, h.readErr
}

func (h *fakeHost) Exists(_ context.Context, id string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, id)
	if h.existsErr != nil {
		return false, h.existsErr
	}
	return h.live[id], nil
}

func (h *fakeHost) List(_ context.Context) ([]string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lists++
	if h.listErr != nil {
		return nil, h.listErr
	}
	if h.listIDs != nil {
		return slices.Clone(h.listIDs), nil
	}
	ids := make([]string, 0, len(h.live))
	for id := range h.live {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}

func (h *fakeHost) Adopt(context.Context, string) error { return nil }

// drop takes a window away behind the manager's back, the way a user closing a
// window by hand does.
func (h *fakeHost) drop(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.live, id)
}

func newTestManager(t *testing.T) (*windowManager, *fakeHost, string) {
	t.Helper()
	dir := t.TempDir()
	host := &fakeHost{}
	return newWindowManager(dir, testEditor, host), host, dir
}

// shortEscalatePoll shrinks escalate's poll interval and timeout for the
// duration of a test, restoring them on cleanup.
func shortEscalatePoll(t *testing.T, timeout time.Duration) {
	t.Helper()
	originalTimeout, originalInterval := escalateTimeout, escalatePollInterval
	escalateTimeout, escalatePollInterval = timeout, 5*time.Millisecond
	t.Cleanup(func() { escalateTimeout, escalatePollInterval = originalTimeout, originalInterval })
}

func TestOpenFileOpensTheEditorWindowAndReplacesTheLastOne(t *testing.T) {
	m, host, dir := newTestManager(t)
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := m.openFile(ctx, openFileInput{Path: "doc.md", Line: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.openFile(ctx, openFileInput{Path: "doc.md"}); err != nil {
		t.Fatal(err)
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "doc.md")

	if len(host.opens) != 2 {
		t.Fatalf("opened %d windows, want 2", len(host.opens))
	}
	first := host.opens[0]
	if first.Kind != workbench.KindTerminal {
		t.Fatalf("editor window kind = %q, want %q", first.Kind, workbench.KindTerminal)
	}
	if want := testEditor.Args(path, 7); !slices.Equal(first.Command, want) {
		t.Fatalf("editor command = %v, want %v", first.Command, want)
	}
	if !first.CloseOnExit {
		t.Fatal("the editor window outlives the editor process")
	}
	if want := testEditor.Args(path, 1); !slices.Equal(host.opens[1].Command, want) {
		t.Fatalf("second editor command = %v, want %v", host.opens[1].Command, want)
	}
	if !slices.Equal(host.closes, []string{host.ids[0]}) {
		t.Fatalf("closed %v, want the first editor window %q", host.closes, host.ids[0])
	}
	if got := m.list(ctx); !slices.Equal(got, []string{editorWindowName}) {
		t.Fatalf("registry = %v, want [%s]", got, editorWindowName)
	}
}

func TestRunCommandOpensATerminalWindowInTheResolvedCwd(t *testing.T) {
	m, host, dir := newTestManager(t)
	repo := filepath.Join(dir, "src", "app")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "npm run dev", Name: "dev", Cwd: "src/app"}); err != nil {
		t.Fatal(err)
	}
	realRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}

	if len(host.opens) != 1 {
		t.Fatalf("opened %d windows, want 1", len(host.opens))
	}
	opts := host.opens[0]
	if opts.Kind != workbench.KindTerminal {
		t.Fatalf("command window kind = %q, want %q", opts.Kind, workbench.KindTerminal)
	}
	if opts.Cwd != realRepo {
		t.Fatalf("cwd = %q, want %q", opts.Cwd, realRepo)
	}
	if want := []string{shellBin, shellLoginFlag, "npm run dev"}; !slices.Equal(opts.Command, want) {
		t.Fatalf("command = %v, want %v", opts.Command, want)
	}
	if !opts.CloseOnExit {
		t.Fatal("the command window survives its own command")
	}
	if got := m.list(ctx); !slices.Equal(got, []string{"dev"}) {
		t.Fatalf("registry = %v, want [dev]", got)
	}
}

func TestRunCommandRejectsCwdEscapeReservedNameAndEmptyCommand(t *testing.T) {
	m, host, _ := newTestManager(t)
	for _, tc := range []struct {
		name  string
		input runCommandInput
		want  error
	}{
		{"a cwd outside the session", runCommandInput{Command: "ls", Cwd: "../"}, nil},
		{"the reserved editor name", runCommandInput{Command: "ls", Name: editorWindowName}, ErrReservedWindowName},
		{"an empty command", runCommandInput{Command: "   "}, ErrCommandRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := m.run(context.Background(), tc.input)
			if err == nil {
				t.Fatalf("accepted %s", tc.name)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
	if len(host.opens) != 0 {
		t.Fatalf("rejected calls opened %d windows", len(host.opens))
	}
}

func TestReadWindowReturnsTheHostsText(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.text = "listening on :3000\n"
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}

	out, err := m.read(ctx, readWindowInput{Name: "server", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if out != "listening on :3000" {
		t.Fatalf("read output = %q", out)
	}
	want := readCall{id: host.ids[0], full: true}
	if len(host.reads) != 1 || host.reads[0] != want {
		t.Fatalf("reads = %v, want [%v]", host.reads, want)
	}
	if _, err := m.read(ctx, readWindowInput{Name: "missing"}); err == nil {
		t.Fatal("read of an unregistered window should error")
	}
}

func TestReadWindowKeepsTheTailOfALongWindow(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.text = "HEAD" + strings.Repeat("x", readWindowLimit) + "TAIL"
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}

	out, err := m.read(ctx, readWindowInput{Name: "server"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(truncatedPrefix)+readWindowLimit {
		t.Fatalf("read returned %d bytes, want %d", len(out), len(truncatedPrefix)+readWindowLimit)
	}
	if !strings.HasPrefix(out, truncatedPrefix) {
		t.Fatalf("truncated output does not say so: %q", out[:len(truncatedPrefix)])
	}
	if strings.Contains(out, "HEAD") || !strings.HasSuffix(out, "TAIL") {
		t.Fatal("read kept the head of the output rather than the tail")
	}
}

func TestReadWindowReportsAWindowWithNoOutput(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.text = "\n  \n"
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}

	out, err := m.read(ctx, readWindowInput{Name: "server"})
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf(noOutputFormat, "server"); out != want {
		t.Fatalf("read output = %q, want %q", out, want)
	}
}

func TestCloseWindowClosesItAndDropsItFromTheRegistry(t *testing.T) {
	m, host, _ := newTestManager(t)
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "tail -f log", Name: "logs"}); err != nil {
		t.Fatal(err)
	}

	if _, err := m.closeWindow(ctx, windowNameInput{Name: "logs"}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(host.closes, []string{host.ids[0]}) {
		t.Fatalf("closed %v, want [%s]", host.closes, host.ids[0])
	}
	if got := m.list(ctx); len(got) != 0 {
		t.Fatalf("registry = %v, want empty", got)
	}
	if _, err := m.closeWindow(ctx, windowNameInput{Name: "logs"}); err == nil {
		t.Fatal("closing an unknown window should error")
	}
}

// The registry is qrouton's own map and nothing in it learns that the user
// closed a window. Addressing a name whose window is gone must say so — and
// forget it — rather than handing a dead id to the workbench and surfacing that
// failure instead of a reason.
func TestReadAndClosePruneAWindowTheUserClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*windowManager) error
	}{
		{toolReadWindow, func(m *windowManager) error {
			_, err := m.read(context.Background(), readWindowInput{Name: "server"})
			return err
		}},
		{toolCloseWindow, func(m *windowManager) error {
			_, err := m.closeWindow(context.Background(), windowNameInput{Name: "server"})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, host, _ := newTestManager(t)
			ctx := context.Background()
			if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "server"}); err != nil {
				t.Fatal(err)
			}
			host.drop(host.ids[0])

			err := tc.call(m)
			if err == nil {
				t.Fatal("addressing a window the user closed should fail")
			}
			if err.Error() != windowGone("server").Error() {
				t.Fatalf("error does not explain the window is gone: %v", err)
			}
			if got := m.list(ctx); len(got) != 0 {
				t.Fatalf("registry still holds a closed window: %v", got)
			}
		})
	}
}

func TestReadWindowKeepsALiveWindow(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.text = "listening on :3000"
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}

	out, err := m.read(ctx, readWindowInput{Name: "server"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "listening on :3000" {
		t.Fatalf("read output = %q", out)
	}
	if !slices.Equal(host.checks, []string{host.ids[0]}) {
		t.Fatalf("liveness checks = %v, want [%s]", host.checks, host.ids[0])
	}
	if got := m.list(ctx); !slices.Equal(got, []string{"server"}) {
		t.Fatalf("live window was pruned: %v", got)
	}
}

// A failing liveness check is not evidence of absence. Reading "closed" out of a
// workbench hiccup would have the agent reopen windows that are fine, so the
// window stays and the caller's own action reports whatever is actually wrong.
func TestReadWindowKeepsTheWindowWhenLivenessCannotBeChecked(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.text = "listening on :3000"
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "server"}); err != nil {
		t.Fatal(err)
	}
	host.existsErr = errors.New("workbench unreachable")

	if _, err := m.read(ctx, readWindowInput{Name: "server"}); err != nil {
		t.Fatalf("an unanswerable liveness check should not fail the read: %v", err)
	}
	if got := m.list(ctx); !slices.Equal(got, []string{"server"}) {
		t.Fatalf("window pruned on a failed liveness check: %v", got)
	}
}

func TestShowDiffOpensADocumentWindowForOneRepoAndForAllRepos(t *testing.T) {
	m, host, dir := newTestManager(t)
	if err := os.MkdirAll(filepath.Join(dir, "src", "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	if _, err := m.showDiff(ctx, showDiffInput{Repo: "src/app", Base: "main", Staged: true}); err != nil {
		t.Fatal(err)
	}
	single := host.opens[0]
	if single.Kind != workbench.KindDocument {
		t.Fatalf("diff window kind = %q, want %q", single.Kind, workbench.KindDocument)
	}
	if strings.TrimSpace(single.Content) == "" {
		t.Fatal("diff window opened with no captured diff")
	}
	if single.Format != workbench.FormatDiff {
		t.Fatalf("diff window format = %q, want %q; its page would render it as grey text", single.Format, workbench.FormatDiff)
	}
	if got := m.list(ctx); !slices.Equal(got, []string{"diff:app"}) {
		t.Fatalf("registry = %v, want [diff:app]", got)
	}

	if _, err := m.showDiff(ctx, showDiffInput{}); err != nil {
		t.Fatal(err)
	}
	all := host.opens[1]
	if all.Kind != workbench.KindDocument || all.Format != workbench.FormatDiff || strings.TrimSpace(all.Content) == "" {
		t.Fatalf("all-repos diff window = %+v", all)
	}
	if got := m.list(ctx); !slices.Equal(got, []string{diffWindowName, "diff:app"}) {
		t.Fatalf("registry = %v, want [%s diff:app]", got, diffWindowName)
	}

	if _, err := m.showDiff(ctx, showDiffInput{Repo: "../outside"}); err == nil {
		t.Fatal("accepted a repo path outside the session")
	}
}

func TestNotifyOpensAnExpiringToastAndRingsTheSessionSound(t *testing.T) {
	m, host, dir := newTestManager(t)
	var played string
	original := playSound
	playSound = func(script string) { played = script }
	t.Cleanup(func() { playSound = original })
	ctx := context.Background()

	if _, err := m.notify(ctx, notifyInput{Message: "build finished"}); err != nil {
		t.Fatal(err)
	}
	opts := host.opens[0]
	if opts.Kind != workbench.KindDocument {
		t.Fatalf("toast kind = %q, want %q", opts.Kind, workbench.KindDocument)
	}
	if opts.TTL != toastLifetime {
		t.Fatalf("toast TTL = %s, want %s", opts.TTL, toastLifetime)
	}
	if !strings.Contains(opts.Content, "build finished") {
		t.Fatalf("toast content = %q", opts.Content)
	}
	if opts.Format != "" {
		t.Fatalf("the toast declared the %q format; only show_diff does", opts.Format)
	}
	if want := sessionpaths.NotifyScript(dir); played != want {
		t.Fatalf("played %q, want %q", played, want)
	}
	if _, err := m.notify(ctx, notifyInput{Message: "  "}); !errors.Is(err, ErrMessageRequired) {
		t.Fatalf("empty message error = %v, want ErrMessageRequired", err)
	}
}

func TestEscalateOpensTheFocusedPicker(t *testing.T) {
	m, host, dir := newTestManager(t)
	shortEscalatePoll(t, 200*time.Millisecond)

	// A cancelled stanza lets escalate return promptly once its poll notices it,
	// so the test doesn't wait out the full timeout to inspect the window.
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = session.WriteManifest(dir, session.Manifest{
			Escalation: &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()},
		})
	}()

	message, err := m.escalate(context.Background(), escalateInput{Name: "webhook retry", BranchPrefix: "fix"})
	if err != nil {
		t.Fatal(err)
	}
	if message != escalationCancelledMessage {
		t.Fatalf("message = %q, want the cancelled message", message)
	}

	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opts := host.opens[0]
	if opts.Kind != workbench.KindTerminal {
		t.Fatalf("picker kind = %q, want %q", opts.Kind, workbench.KindTerminal)
	}
	want := []string{bin, pickSubcommand, sessionRootArg, dir, nameArg, "webhook retry", prefixArg, "fix"}
	if !slices.Equal(opts.Command, want) {
		t.Fatalf("picker command = %v, want %v", opts.Command, want)
	}
	if !opts.Focus {
		t.Fatal("the picker opened without keyboard focus; no agent is waiting for the keyboard back")
	}
	if !opts.CloseOnExit {
		t.Fatal("the picker window outlives the picker")
	}
}

func TestEscalateRejectsBlankName(t *testing.T) {
	m, _, _ := newTestManager(t)
	if _, err := m.escalate(context.Background(), escalateInput{Name: "   "}); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("blank name error = %v, want ErrNameRequired", err)
	}
}

func TestEscalateBlocksUntilConfirmed(t *testing.T) {
	m, _, dir := newTestManager(t)
	shortEscalatePoll(t, time.Second)

	start := time.Now()
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = session.WriteManifest(dir, session.Manifest{
			Escalation: &session.EscalationOutcome{Status: session.EscalationConfirmed, At: time.Now()},
		})
	}()

	message, err := m.escalate(context.Background(), escalateInput{Name: "webhook retry"})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("escalate returned after %s, before the confirmed stanza landed", elapsed)
	}
	if message != escalationConfirmedMessage {
		t.Fatalf("message = %q, want the confirmed message", message)
	}
}

func TestEscalateBlocksUntilCancelled(t *testing.T) {
	m, _, dir := newTestManager(t)
	shortEscalatePoll(t, time.Second)

	start := time.Now()
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = session.WriteManifest(dir, session.Manifest{
			Escalation: &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()},
		})
	}()

	message, err := m.escalate(context.Background(), escalateInput{Name: "webhook retry"})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("escalate returned after %s, before the cancelled stanza landed", elapsed)
	}
	if message != escalationCancelledMessage {
		t.Fatalf("message = %q, want the cancelled message", message)
	}
}

func TestEscalateTimesOutWhenPickerStaysOpen(t *testing.T) {
	m, _, _ := newTestManager(t)
	shortEscalatePoll(t, 20*time.Millisecond)

	message, err := m.escalate(context.Background(), escalateInput{Name: "webhook retry"})
	if err != nil {
		t.Fatal(err)
	}
	if message != escalationTimeoutMessage {
		t.Fatalf("message = %q, want the timeout message", message)
	}
}

func TestListWindowsSortsNamesAndDropsClosedOnes(t *testing.T) {
	m, host, _ := newTestManager(t)
	ctx := context.Background()
	for _, name := range []string{"tests", "dev"} {
		if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: name}); err != nil {
			t.Fatal(err)
		}
	}

	if got := m.list(ctx); !slices.Equal(got, []string{"dev", "tests"}) {
		t.Fatalf("list = %v, want [dev tests]", got)
	}
	host.drop(host.ids[0])
	if got := m.list(ctx); !slices.Equal(got, []string{"dev"}) {
		t.Fatalf("list = %v, want [dev] once the tests window is gone", got)
	}
}

// An unreadable window list is not evidence any window is gone, so the registry
// stands rather than emptying itself on a workbench hiccup.
func TestListWindowsKeepsEverythingWhenTheHostCannotList(t *testing.T) {
	m, host, _ := newTestManager(t)
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "dev"}); err != nil {
		t.Fatal(err)
	}
	host.listErr = errors.New("workbench unreachable")

	if got := m.list(ctx); !slices.Equal(got, []string{"dev"}) {
		t.Fatalf("list = %v, want [dev]", got)
	}
	if host.lists != 1 {
		t.Fatalf("host asked for its window list %d times, want 1", host.lists)
	}
}

func TestMCPServerAdvertisesExactlyTheWindowTools(t *testing.T) {
	ctx := context.Background()
	server := newMCPServer(t.TempDir(), testEditor, &fakeHost{})
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	advertised := map[string]bool{}
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		advertised[tool.Name] = true
	}
	for _, name := range []string{
		toolOpenFile, toolRunCommand, toolReadWindow, toolShowDiff,
		toolNotify, toolCloseWindow, toolListWindows, toolEscalate,
	} {
		if !advertised[name] {
			t.Errorf("tool %q was not advertised", name)
		}
		delete(advertised, name)
	}
	for name := range advertised {
		t.Errorf("tool %q is advertised but no longer part of the surface", name)
	}
}
