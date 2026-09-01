package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
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
	views  []string
	checks []string
	lists  int

	live map[string]bool

	pickers   []workbench.PickerRequest
	pickerErr error
	// onPicker answers from inside the Picker call, which is the only place a
	// test can stamp an outcome that escalate's poll will not read as stale.
	onPicker func(workbench.PickerRequest)

	adds      []workbench.AddReposRequest
	addResult workbench.AddReposResult
	addErr    error

	text        string
	readErr     error
	viewports   []*workbench.DocumentViewport
	viewportErr error
	existsErr   error
	listIDs     []string
	listErr     error
	openErr     error

	openEntered chan struct{}
	openGate    chan struct{}
}

// blockOpen holds the next Open until release is called, without holding the
// fake's own lock, so the rest of the host stays answerable meanwhile.
func (h *fakeHost) blockOpen() (entered <-chan struct{}, release func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.openEntered, h.openGate = make(chan struct{}), make(chan struct{})
	gate := h.openGate
	return h.openEntered, sync.OnceFunc(func() { close(gate) })
}

func (h *fakeHost) Open(_ context.Context, opts workbench.WindowOptions) (string, error) {
	h.mu.Lock()
	entered, gate := h.openEntered, h.openGate
	h.openEntered, h.openGate = nil, nil
	h.mu.Unlock()
	if gate != nil {
		close(entered)
		<-gate
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.openErr != nil {
		return "", h.openErr
	}
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

func (h *fakeHost) Viewport(_ context.Context, id string) (*workbench.DocumentViewport, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.views = append(h.views, id)
	if h.viewportErr != nil {
		return nil, h.viewportErr
	}
	if len(h.viewports) == 0 {
		return nil, nil
	}
	viewport := h.viewports[0]
	if len(h.viewports) > 1 {
		h.viewports = h.viewports[1:]
	}
	return viewport, nil
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

func (h *fakeHost) Adopt(context.Context, string, bool) error { return nil }

func (h *fakeHost) Picker(_ context.Context, req workbench.PickerRequest) error {
	h.mu.Lock()
	h.pickers = append(h.pickers, req)
	hook, err := h.onPicker, h.pickerErr
	h.mu.Unlock()
	if hook != nil {
		hook(req)
	}
	return err
}

func (h *fakeHost) AddRepos(_ context.Context, req workbench.AddReposRequest) (workbench.AddReposResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.adds = append(h.adds, req)
	if h.addErr != nil {
		return workbench.AddReposResult{}, h.addErr
	}
	return h.addResult, nil
}

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

func shortViewportPoll(t *testing.T, timeout time.Duration) {
	t.Helper()
	originalTimeout, originalInterval := viewportWaitTimeout, viewportPollInterval
	viewportWaitTimeout, viewportPollInterval = timeout, time.Millisecond
	t.Cleanup(func() { viewportWaitTimeout, viewportPollInterval = originalTimeout, originalInterval })
}

func boolPtr(value bool) *bool { return &value }

func TestForegroundResolvesEveryOpeningTool(t *testing.T) {
	for _, tc := range []struct {
		name       string
		foreground *bool
		want       bool
	}{
		{"omitted", nil, false},
		{"false", boolPtr(false), false},
		{"true", boolPtr(true), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("open file", func(t *testing.T) {
				m, host, dir := newTestManager(t)
				if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if _, _, err := m.openFile(context.Background(), openFileInput{Path: "main.go", Foreground: tc.foreground}); err != nil {
					t.Fatal(err)
				}
				if got := host.opens[0].Select; got != tc.want {
					t.Fatalf("Select = %v, want %v", got, tc.want)
				}
			})
			t.Run("run command", func(t *testing.T) {
				m, host, _ := newTestManager(t)
				if _, err := m.run(context.Background(), runCommandInput{Command: "serve", Foreground: tc.foreground}); err != nil {
					t.Fatal(err)
				}
				if got := host.opens[0].Select; got != tc.want {
					t.Fatalf("Select = %v, want %v", got, tc.want)
				}
			})
			t.Run("show diff", func(t *testing.T) {
				m, host, _ := newTestManager(t)
				if _, err := m.showDiff(context.Background(), showDiffInput{Foreground: tc.foreground}); err != nil {
					t.Fatal(err)
				}
				if got := host.opens[0].Select; got != tc.want {
					t.Fatalf("Select = %v, want %v", got, tc.want)
				}
			})
			t.Run("notify", func(t *testing.T) {
				m, host, _ := newTestManager(t)
				original := playSound
				playSound = func(string) {}
				t.Cleanup(func() { playSound = original })
				if _, err := m.notify(context.Background(), notifyInput{Message: "done", Foreground: tc.foreground}); err != nil {
					t.Fatal(err)
				}
				if got := host.opens[0].Select; got != tc.want {
					t.Fatalf("Select = %v, want %v", got, tc.want)
				}
			})
		})
	}
}

func TestOpenFileForegroundDefaultsToCanonicalThoughtsSource(t *testing.T) {
	m, host, root := newTestManager(t)
	parked := filepath.Join(t.TempDir(), "parked")
	if err := os.MkdirAll(filepath.Join(parked, "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	thought := filepath.Join(parked, "shared", "P007.md")
	if err := os.WriteFile(thought, []byte("# Plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(parked, filepath.Join(root, sessionpaths.ThoughtsDirName)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "thoughts-old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "thoughts-old", "P008.md"), []byte("# Not thoughts\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		path       string
		foreground *bool
		want       bool
	}{
		{"thoughts relative", filepath.Join("thoughts", "shared", "P007.md"), nil, true},
		{"thoughts explicit background", filepath.Join("thoughts", "shared", "P007.md"), boolPtr(false), false},
		{"thoughts absolute", thought, nil, true},
		{"thoughts lookalike", filepath.Join("thoughts-old", "P008.md"), nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := m.openFile(context.Background(), openFileInput{Path: tc.path, Foreground: tc.foreground}); err != nil {
				t.Fatal(err)
			}
			got := host.opens[len(host.opens)-1]
			if got.Select != tc.want {
				t.Fatalf("Select = %v, want %v", got.Select, tc.want)
			}
			if tc.want && got.Source != filepath.Join("thoughts", "shared", "P007.md") {
				t.Fatalf("Source = %q, want canonical thoughts source", got.Source)
			}
		})
	}
}

func TestBackgroundMarkdownOpenReadsOneUnselectedViewport(t *testing.T) {
	m, host, dir := newTestManager(t)
	if err := os.WriteFile(filepath.Join(dir, "P007.md"), []byte("# Plan\n\nText\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host.viewports = []*workbench.DocumentViewport{{
		Source: "P007.md", Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 3, To: 3}},
	}}
	message, viewport, err := m.openFile(context.Background(), openFileInput{Path: "P007.md", Line: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(host.views) != 1 || viewport == nil || viewport.Selected || viewport.Available || len(viewport.Intervals) != 0 {
		t.Fatalf("background viewport reads = %d, viewport = %+v", len(host.views), viewport)
	}
	if strings.Contains(message, "visible in a measured block") || strings.Contains(message, "scrolled") {
		t.Fatalf("background response claims visibility: %q", message)
	}

	host.viewports = []*workbench.DocumentViewport{{
		Source: "P007.md", Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 3, To: 3}},
	}}
	message, viewport, err = m.openFile(context.Background(), openFileInput{Path: "P007.md", Line: 3, Foreground: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if viewport == nil || !viewport.Selected || !strings.Contains(message, "visible in a measured block") {
		t.Fatalf("selected response = %q, viewport = %+v", message, viewport)
	}
}

func TestOpenFileOpensTheEditorWindowAndReplacesTheLastOne(t *testing.T) {
	m, host, dir := newTestManager(t)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := m.openFile(ctx, openFileInput{Path: "main.go", Line: 7}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.openFile(ctx, openFileInput{Path: "main.go"}); err != nil {
		t.Fatal(err)
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(realDir, "main.go")

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

// Showing the user a plan is the common case, and an editor is not how they read
// one.
func TestOpenFileRendersMarkdownInsteadOfLaunchingTheEditor(t *testing.T) {
	m, host, dir := newTestManager(t)
	body := "# Document panes\n\nThe pane is a tab.\n"
	if err := os.WriteFile(filepath.Join(dir, "P007.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	host.viewports = []*workbench.DocumentViewport{{
		Source: "P007.md", Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 3, To: 3}},
	}}
	message, viewport, err := m.openFile(context.Background(), openFileInput{Path: "P007.md", Foreground: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if viewport == nil || !viewport.Available {
		t.Fatalf("open viewport = %+v", viewport)
	}
	if len(host.opens) != 1 {
		t.Fatalf("opened %d windows, want 1", len(host.opens))
	}
	opts := host.opens[0]
	if opts.Kind != workbench.KindDocument || opts.Format != workbench.FormatMarkdown {
		t.Fatalf("markdown opened as a %q/%q", opts.Kind, opts.Format)
	}
	if opts.Content != body || opts.Source != "P007.md" {
		t.Fatalf("pane content = %q from %q", opts.Content, opts.Source)
	}
	if opts.Badge != "P007" || opts.Label != "Document panes" {
		t.Fatalf("pane tab = %q %q", opts.Badge, opts.Label)
	}
	if opts.Span != (workbench.LineSpan{}) {
		t.Fatalf("a pane nobody aimed carries a span: %+v", opts.Span)
	}
	if strings.Contains(message, "requested line") || strings.Contains(message, "scrolled") {
		t.Fatalf("the agent was told it requested a line or caused a scroll: %q", message)
	}
}

// A long document is no use open at the top when the agent means one passage of
// it, so the span travels to the pane and the agent is told what was marked.
func TestOpenFileAimsARenderedPaneAtTheLinesTheAgentAsksFor(t *testing.T) {
	m, host, dir := newTestManager(t)
	if err := os.WriteFile(filepath.Join(dir, "P007.md"), []byte("# Plan\n\nPhase 1.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	host.viewports = []*workbench.DocumentViewport{
		{Source: "P007.md", Available: true, Selected: true, Intervals: []workbench.LineInterval{{Line: 7, To: 7}}},
		{Source: "P007.md", Available: true, Selected: true, Intervals: []workbench.LineInterval{{Line: 7, To: 19}}},
	}

	message, _, err := m.openFile(ctx, openFileInput{Path: "P007.md", Line: 7, Foreground: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if got := host.opens[0].Span; got != (workbench.LineSpan{Line: 7, Through: 7}) {
		t.Fatalf("pane span = %+v, want line 7 alone", got)
	}
	if !strings.Contains(message, "line 7") {
		t.Fatalf("message = %q, want the marked line named", message)
	}

	message, _, err = m.openFile(ctx, openFileInput{Path: "P007.md", Line: 7, Through: 19, Foreground: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if got := host.opens[1].Span; got != (workbench.LineSpan{Line: 7, Through: 19}) {
		t.Fatalf("pane span = %+v, want lines 7 to 19", got)
	}
	if !strings.Contains(message, "lines 7-19") {
		t.Fatalf("message = %q, want the marked range named", message)
	}
}

func TestOpenFileDoesNotClaimLiftedBlankOrPastEndLinesAreVisible(t *testing.T) {
	m, host, dir := newTestManager(t)
	if err := os.WriteFile(filepath.Join(dir, "P007.md"), []byte("# Plan\n\nPhase 1.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, line := range []int{1, 2, 99} {
		host.viewports = []*workbench.DocumentViewport{{
			Source: "P007.md", Available: true, Selected: true,
			Intervals: []workbench.LineInterval{{Line: 3, To: 3}},
		}}
		message, viewport, err := m.openFile(context.Background(), openFileInput{Path: "P007.md", Line: line, Foreground: boolPtr(true)})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(message, "scrolled") || strings.Contains(message, "marked") ||
			strings.Contains(message, "requested source range visible") {
			t.Fatalf("line %d response invents visibility: %q", line, message)
		}
		if !strings.Contains(message, fmt.Sprintf("requested line %d", line)) || !strings.Contains(message, "could not be verified") {
			t.Fatalf("line %d response is not explicit about the unverified target: %q", line, message)
		}
		if viewport == nil || len(viewport.Intervals) != 1 || viewport.Intervals[0].Line != 3 {
			t.Fatalf("line %d viewport = %+v", line, viewport)
		}
	}
}

// A workbench that could not resolve an editor still serves the tools: what
// qrouton renders itself opens as a pane, and only the rest is refused — one
// tool call at a time, rather than the whole session.
func TestOpenFileWithNoEditorRendersWhatItCanAndRefusesTheRest(t *testing.T) {
	dir := t.TempDir()
	host := &fakeHost{}
	m := newWindowManager(dir, launch.EditorCommand{}, host)
	shortViewportPoll(t, 8*time.Millisecond)
	for name, body := range map[string]string{"P007.md": "# Plan\n\nPhase 1.\n", "main.go": "package main\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()

	if _, _, err := m.openFile(ctx, openFileInput{Path: "P007.md"}); err != nil {
		t.Fatalf("a rendered pane needed an editor: %v", err)
	}
	if _, _, err := m.openFile(ctx, openFileInput{Path: "main.go"}); !errors.Is(err, launch.ErrNoEditor) {
		t.Fatalf("opening a source file with no editor = %v, want %v", err, launch.ErrNoEditor)
	}
	if len(host.opens) != 1 {
		t.Fatalf("opened %d windows, want only the rendered pane", len(host.opens))
	}
	if host.opens[0].Kind != workbench.KindDocument {
		t.Fatalf("the pane opened as a %q", host.opens[0].Kind)
	}
}

// The editor does not gate the server: an absent one reaches the handle, and
// only a malformed one stops the process.
func TestRunRefusesAMalformedEditorButNotAnAbsentOne(t *testing.T) {
	if err := Run(t.TempDir(), "", "{}"); !errors.Is(err, workbench.ErrHandleIncomplete) {
		t.Fatalf("an absent editor = %v, want the handle to be the only complaint", err)
	}
	if err := Run(t.TempDir(), "{", "{}"); !errors.Is(err, launch.ErrInvalidEditor) {
		t.Fatalf("a malformed editor = %v, want %v", err, launch.ErrInvalidEditor)
	}
}

func TestOpenFileTimesOutSuccessfullyWithAnUnavailableViewport(t *testing.T) {
	m, host, dir := newTestManager(t)
	shortViewportPoll(t, 8*time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "P007.md"), []byte("# Plan\n\nPhase 1.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host.viewports = []*workbench.DocumentViewport{{Source: "P007.md", Intervals: []workbench.LineInterval{}}}
	message, viewport, err := m.openFile(context.Background(), openFileInput{Path: "P007.md", Line: 3})
	if err != nil {
		t.Fatal(err)
	}
	if viewport == nil || viewport.Available || viewport.Selected || viewport.Intervals == nil {
		t.Fatalf("timeout viewport = %+v", viewport)
	}
	if strings.Contains(message, "scrolled") || strings.Contains(message, "visible in a measured block") {
		t.Fatalf("timeout response claims visibility: %q", message)
	}
	if !strings.Contains(message, "could not be verified") || !strings.Contains(message, "not selected") {
		t.Fatalf("timeout response does not describe the unavailable viewport: %q", message)
	}
}

func TestOpenFilePollsPastUnavailableUntilTheRequestedBlockIsMeasured(t *testing.T) {
	m, host, dir := newTestManager(t)
	shortViewportPoll(t, 50*time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "P007.md"), []byte("# Plan\n\nA paragraph\nstill the paragraph\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host.viewports = []*workbench.DocumentViewport{
		{Source: "P007.md", Selected: true, Intervals: []workbench.LineInterval{}},
		{Source: "P007.md", Available: true, Selected: true,
			Intervals: []workbench.LineInterval{{Line: 3, To: 4}}},
	}
	message, viewport, err := m.openFile(context.Background(), openFileInput{Path: "P007.md", Line: 4, Foreground: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if len(host.views) < 2 {
		t.Fatalf("viewport was queried %d times, want at least two", len(host.views))
	}
	if viewport == nil || !viewport.Available || len(viewport.Intervals) != 1 || viewport.Intervals[0].To != 4 {
		t.Fatalf("eventual viewport = %+v", viewport)
	}
	if !strings.Contains(message, "scrolled") || !strings.Contains(message, "visible in a measured block") {
		t.Fatalf("measured response does not verify the requested block: %q", message)
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

	out, viewport, err := m.read(ctx, readWindowInput{Name: "server", Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if out != "listening on :3000" {
		t.Fatalf("read output = %q", out)
	}
	if viewport != nil {
		t.Fatalf("terminal read viewport = %+v", viewport)
	}
	want := readCall{id: host.ids[0], full: true}
	if len(host.reads) != 1 || host.reads[0] != want {
		t.Fatalf("reads = %v, want [%v]", host.reads, want)
	}
	if _, _, err := m.read(ctx, readWindowInput{Name: "missing"}); err == nil {
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

	out, _, err := m.read(ctx, readWindowInput{Name: "server"})
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

	out, _, err := m.read(ctx, readWindowInput{Name: "server"})
	if err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf(noOutputFormat, "server"); out != want {
		t.Fatalf("read output = %q, want %q", out, want)
	}
}

func TestReadWindowReportsMarkdownViewportStates(t *testing.T) {
	for _, tc := range []struct {
		name     string
		viewport *workbench.DocumentViewport
		want     string
	}{
		{"unavailable", &workbench.DocumentViewport{Source: "P007.md", Selected: true, Intervals: []workbench.LineInterval{}}, viewportUnavailableSelected},
		{"measured empty", &workbench.DocumentViewport{Source: "P007.md", Available: true, Selected: true, Intervals: []workbench.LineInterval{}}, viewportMeasuredEmpty},
		{"disjoint", &workbench.DocumentViewport{Source: "P007.md", Available: true, Selected: true,
			Intervals: []workbench.LineInterval{{Line: 3, To: 5}, {Line: 12, To: 12}}},
			"visible source blocks: lines 3-5, line 12"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, host, _ := newTestManager(t)
			_, err := m.open(context.Background(), editorWindowName, workbench.WindowOptions{
				Kind: workbench.KindDocument, Format: workbench.FormatMarkdown,
				Source: "P007.md", Content: "# Plan\n\nText\n",
			})
			if err != nil {
				t.Fatal(err)
			}
			host.text = "# Plan\n\nText\n"
			host.viewports = []*workbench.DocumentViewport{tc.viewport}
			out, viewport, err := m.read(context.Background(), readWindowInput{Name: editorWindowName})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.want) {
				t.Fatalf("read output = %q, want %q", out, tc.want)
			}
			if viewport == nil || viewport.Source != "P007.md" || viewport.Intervals == nil {
				t.Fatalf("structured viewport = %+v", viewport)
			}
		})
	}
}

func TestReadWindowKeepsMarkdownViewportAfterTruncatedSource(t *testing.T) {
	m, host, _ := newTestManager(t)
	_, err := m.open(context.Background(), editorWindowName, workbench.WindowOptions{
		Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Source: "P007.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	host.text = "HEAD" + strings.Repeat("x", readWindowLimit) + "TAIL"
	host.viewports = []*workbench.DocumentViewport{{
		Source: "P007.md", Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 20, To: 24}},
	}}
	out, _, err := m.read(context.Background(), readWindowInput{Name: editorWindowName})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, truncatedPrefix) || strings.Contains(out, "HEAD") {
		t.Fatalf("source was not truncated from its head: %q", out[:len(truncatedPrefix)+8])
	}
	if !strings.Contains(out, "TAIL\n\n") || !strings.HasSuffix(out, "visible source blocks: lines 20-24.") {
		t.Fatalf("viewport summary did not survive source truncation: %q", out[len(out)-100:])
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
			_, _, err := m.read(context.Background(), readWindowInput{Name: "server"})
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

	out, _, err := m.read(ctx, readWindowInput{Name: "server"})
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

	if _, _, err := m.read(ctx, readWindowInput{Name: "server"}); err != nil {
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

func TestNotifyOpensADurableAttentionTabAndRingsTheSessionSound(t *testing.T) {
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
	if !opts.Attention {
		t.Fatal("notification tab does not request attention")
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

// The picker is the running workbench's to draw, over the session escalate
// names. Nothing is spawned and nothing takes the keyboard, so the tool opens no
// window at all.
func TestEscalateQueuesThePickerOnItsOwnSessionAndOpensNoWindow(t *testing.T) {
	m, host, dir := newTestManager(t)
	shortEscalatePoll(t, 2*time.Second)

	// Cancelled from inside the picker call, so the stanza is stamped after the
	// spawn escalationOutcome compares against. A goroutine sleeping against that
	// timestamp instead is a race the machine decides: delay the caller past the
	// sleep and the outcome is written first, read as stale, and polled until the
	// timeout returns the wrong message. The budget above is now only the failure
	// path's, since the answer arrives before the first tick.
	host.onPicker = func(workbench.PickerRequest) {
		_ = session.WriteManifest(dir, session.Manifest{
			Escalation: &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()},
		})
	}

	before := time.Now()
	message, err := m.escalate(context.Background(), escalateInput{Name: "webhook retry", BranchPrefix: "fix"})
	if err != nil {
		t.Fatal(err)
	}
	if message != escalationCancelledMessage {
		t.Fatalf("message = %q, want the cancelled message", message)
	}
	if len(host.opens) != 0 {
		t.Fatalf("escalate opened %+v; the workbench draws the picker itself", host.opens)
	}
	if len(host.pickers) != 1 || host.pickers[0].SessionRoot != dir {
		t.Fatalf("escalate queued %+v, want one request for its own session", host.pickers)
	}
	// A session with no repositories yet has no branch, so the picker needs the
	// name and prefix the agent proposed to cut one.
	if got := host.pickers[0]; got.Name != "webhook retry" || got.Prefix != "fix" {
		t.Fatalf("queued request = %+v, want the name and prefix escalate was given", got)
	}
	// The deadline travels with the request, so the workbench never draws a picker
	// whose answer nothing is waiting for.
	if got := host.pickers[0].Deadline; !got.After(before) {
		t.Fatalf("queued deadline = %s, want one ahead of the request", got)
	}
}

func TestEscalateReportsAWorkbenchThatCannotDrawThePicker(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.pickerErr = errors.New("unreachable")
	if _, err := m.escalate(context.Background(), escalateInput{Name: "webhook retry"}); err == nil {
		t.Fatal("escalate succeeded with no workbench to draw the picker")
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

// A window tool waiting on the desktop process holds up its own caller and
// nothing else: the registry lock is not carried across the socket round trip.
func TestASlowOpenDoesNotStallTheOtherWindowTools(t *testing.T) {
	m, host, _ := newTestManager(t)
	ctx := context.Background()
	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "dev"}); err != nil {
		t.Fatal(err)
	}

	entered, release := host.blockOpen()
	defer release()
	slow := make(chan error, 1)
	go func() {
		_, err := m.run(ctx, runCommandInput{Command: "build", Name: "build"})
		slow <- err
	}()
	<-entered

	done := make(chan []string, 1)
	go func() {
		names := m.list(ctx)
		if _, err := m.closeWindow(ctx, windowNameInput{Name: "dev"}); err != nil {
			t.Error(err)
		}
		done <- names
	}()
	select {
	case names := <-done:
		if !slices.Equal(names, []string{"dev"}) {
			t.Fatalf("list = %v, want [dev] while an open is in flight", names)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("list_windows and close_window blocked behind an open that had not returned")
	}

	release()
	if err := <-slow; err != nil {
		t.Fatal(err)
	}
}

// Two opens racing on one name settle on the later claim, and the window the
// slower one opened is closed rather than left adrift.
func TestConcurrentOpensOfOneNameKeepTheLaterClaim(t *testing.T) {
	m, host, _ := newTestManager(t)
	ctx := context.Background()

	entered, release := host.blockOpen()
	defer release()
	slow := make(chan error, 1)
	go func() {
		_, err := m.run(ctx, runCommandInput{Command: "first", Name: "dev"})
		slow <- err
	}()
	<-entered

	second := make(chan error, 1)
	go func() {
		_, err := m.run(ctx, runCommandInput{Command: "second", Name: "dev"})
		second <- err
	}()
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a second open on the same name blocked behind the first")
	}
	winner, err := m.liveWindow(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}

	release()
	if err := <-slow; err != nil {
		t.Fatal(err)
	}
	got, err := m.liveWindow(ctx, "dev")
	if err != nil || got != winner {
		t.Fatalf("dev resolves to %q (%v), want the later open's window %q", got, err, winner)
	}
	loser := host.ids[len(host.ids)-1]
	if loser == winner || !slices.Contains(host.closes, loser) {
		t.Fatalf("window %q from the earlier open was left adrift; closes = %v", loser, host.closes)
	}
	if names := m.list(ctx); !slices.Equal(names, []string{"dev"}) {
		t.Fatalf("list = %v, want [dev]", names)
	}
}

// A refused open leaves no name behind for the agent to read from.
func TestAFailedOpenRegistersNothing(t *testing.T) {
	m, host, _ := newTestManager(t)
	ctx := context.Background()
	host.openErr = errors.New("workbench unreachable")

	if _, err := m.run(ctx, runCommandInput{Command: "serve", Name: "dev"}); err == nil {
		t.Fatal("run reported a window the host refused to open")
	}
	if names := m.list(ctx); len(names) != 0 {
		t.Fatalf("list = %v, want no windows", names)
	}
	if _, _, err := m.read(ctx, readWindowInput{Name: "dev"}); err == nil {
		t.Fatal("read found a window the host never opened")
	}
}

// The window tools are the same in both modes; escalate is the assistant's
// alone, because the orchestrator's own prompt tells it it has no such tool and
// the workflow it would escalate into is the one it is already running.
func TestMCPServerAdvertisesExactlyTheWindowTools(t *testing.T) {
	window := []string{
		toolOpenFile, toolRunCommand, toolReadWindow, toolShowDiff,
		toolNotify, toolCloseWindow, toolListWindows, toolSharePage,
		toolListRepos, toolAddRepos,
	}
	for _, tc := range []struct {
		mode session.SessionMode
		want []string
	}{
		{session.ModeAssistant, append(slices.Clone(window), toolEscalate)},
		{session.ModeRPI, window},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			advertised := advertisedTools(t, newMCPServer(t.TempDir(), testEditor, &fakeHost{}, tc.mode))
			for _, name := range tc.want {
				if !advertised[name] {
					t.Errorf("tool %q was not advertised", name)
				}
				delete(advertised, name)
			}
			for name := range advertised {
				t.Errorf("tool %q is advertised but no longer part of the surface", name)
			}
		})
	}
}

func TestListReposReportsEveryRoleFromTheManifest(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := session.WriteManifest(dir, session.Manifest{
		Name: "webhook retry",
		Slug: "webhook-retry-1234",
		Repos: []session.ManifestRepo{
			{
				Name: "api", Org: "acme", Role: session.RepoRoleEditing,
				Branch: "fix/webhook-retry", DefaultBranch: "main", WorktreePath: "src/api",
			},
			{
				Name: "docs", Org: "acme", Role: session.RepoRoleReference,
				Revision: "deadbeef", DefaultBranch: "trunk", WorktreePath: "src/docs",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	cs := connectedClient(t, newMCPServer(dir, testEditor, &fakeHost{}, session.ModeRPI))
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: toolListRepos, Arguments: struct{}{}})
	if err != nil || result.IsError {
		t.Fatalf("list_repos = %+v, %v", result, err)
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content = %#v, want a text block", result.Content[0])
	}
	for _, want := range []string{
		"acme/api (editing) at fix/webhook-retry in src/api",
		"acme/docs (reference) at deadbeef in src/docs",
	} {
		if !strings.Contains(text.Text, want) {
			t.Errorf("text %q does not carry %q", text.Text, want)
		}
	}

	output := structuredOutput(t, result.StructuredContent)
	if got := output["branch"]; got != "fix/webhook-retry" {
		t.Errorf("branch = %#v, want the session branch", got)
	}
	var repos []repoEntry
	body, err := json.Marshal(output["repos"])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &repos); err != nil {
		t.Fatal(err)
	}
	want := []repoEntry{
		{
			Name: "api", Org: "acme", Role: "editing", Branch: "fix/webhook-retry",
			DefaultBranch: "main", WorktreePath: "src/api",
		},
		{
			Name: "docs", Org: "acme", Role: "reference", Revision: "deadbeef",
			DefaultBranch: "trunk", WorktreePath: "src/docs",
		},
	}
	if !reflect.DeepEqual(repos, want) {
		t.Errorf("repos = %+v, want %+v", repos, want)
	}
}

// An empty session is a fact worth stating, not an error, and a manifest that
// will not load is the error.
func TestListReposSeparatesAnEmptySessionFromAnUnreadableOne(t *testing.T) {
	ctx := context.Background()
	empty := t.TempDir()
	if err := session.WriteManifest(empty, session.Manifest{Name: "fresh", Slug: "fresh-0001"}); err != nil {
		t.Fatal(err)
	}
	got, err := listRepos(empty)
	if err != nil || got.Message != noReposHeld || len(got.Repos) != 0 || got.Branch != "" {
		t.Fatalf("listRepos on an empty session = %+v, %v", got, err)
	}

	cs := connectedClient(t, newMCPServer(t.TempDir(), testEditor, &fakeHost{}, session.ModeRPI))
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: toolListRepos, Arguments: struct{}{}})
	if err == nil && !result.IsError {
		t.Fatalf("list_repos on a session with no manifest = %+v, want an error", result)
	}
}

func connectedClient(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ss.Close() })
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func advertisedTools(t *testing.T, server *mcp.Server) map[string]bool {
	t.Helper()
	advertised := map[string]bool{}
	for name := range listedTools(t, server) {
		advertised[name] = true
	}
	return advertised
}

func listedTools(t *testing.T, server *mcp.Server) map[string]*mcp.Tool {
	t.Helper()
	ctx := context.Background()
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

	advertised := map[string]*mcp.Tool{}
	for tool, err := range cs.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		advertised[tool.Name] = tool
	}
	return advertised
}

func TestOpeningToolSchemasExposeOptionalForeground(t *testing.T) {
	tools := listedTools(t, newMCPServer(t.TempDir(), testEditor, &fakeHost{}, session.ModeRPI))
	want := map[string]bool{toolOpenFile: true, toolRunCommand: true, toolShowDiff: true, toolNotify: true}
	for name, tool := range tools {
		schema := structuredOutput(t, tool.InputSchema)
		properties, _ := schema["properties"].(map[string]any)
		foreground, present := properties["foreground"]
		if present != want[name] {
			t.Errorf("tool %q foreground schema = %v, want %v", name, present, want[name])
			continue
		}
		if !present {
			continue
		}
		property, _ := foreground.(map[string]any)
		if description, _ := property["description"].(string); !strings.Contains(description, "Sparse logical-selection override") {
			t.Errorf("tool %q foreground description = %q", name, description)
		}
		required, _ := schema["required"].([]any)
		for _, required := range required {
			if required == "foreground" {
				t.Errorf("tool %q makes foreground required", name)
			}
		}
	}
}

// The mode comes off disk, so a session escalated before this server started
// gets the surface its fresh orchestrator prompt describes. An unreadable
// manifest is not a licence to hand out the assistant's tool.
func TestServerModeFollowsTheManifest(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest *session.Manifest
		want     session.SessionMode
	}{
		{"assistant", &session.Manifest{Mode: session.ModeAssistant}, session.ModeAssistant},
		{"rpi", &session.Manifest{Mode: session.ModeRPI}, session.ModeRPI},
		{"unset", &session.Manifest{}, session.ModeRPI},
		{"no manifest", nil, session.ModeRPI},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.manifest != nil {
				if err := session.WriteManifest(dir, *tc.manifest); err != nil {
					t.Fatal(err)
				}
			}
			if got := sessionMode(dir); got != tc.want {
				t.Fatalf("sessionMode = %q, want %q", got, tc.want)
			}
			advertised := advertisedTools(t, newMCPServer(dir, testEditor, &fakeHost{}, sessionMode(dir)))
			if advertised[toolEscalate] != (tc.want == session.ModeAssistant) {
				t.Fatalf("%s session advertises %s = %v", tc.want, toolEscalate, advertised[toolEscalate])
			}
		})
	}
}

func TestMCPHandlersReturnStructuredMarkdownViewports(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "P007.md"), []byte("# Plan\n\nText\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{viewports: []*workbench.DocumentViewport{{
		Source: "P007.md", Available: true, Selected: true,
		Intervals: []workbench.LineInterval{{Line: 3, To: 3}},
	}}}
	server := newMCPServer(dir, testEditor, host, session.ModeAssistant)
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

	opened, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: toolOpenFile, Arguments: openFileInput{Path: "P007.md", Line: 3, Foreground: boolPtr(true)},
	})
	if err != nil || opened.IsError {
		t.Fatalf("open_file = %+v, %v", opened, err)
	}
	openOutput := structuredOutput(t, opened.StructuredContent)
	assertStructuredViewport(t, openOutput["viewport"], true, true, 3, 3)

	host.mu.Lock()
	host.text = "# Plan\n\nText\n"
	host.viewports = []*workbench.DocumentViewport{{
		Source: "P007.md", Available: true, Selected: true,
		Intervals: []workbench.LineInterval{},
	}}
	host.mu.Unlock()
	read, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: toolReadWindow, Arguments: readWindowInput{Name: editorWindowName},
	})
	if err != nil || read.IsError {
		t.Fatalf("read_window = %+v, %v", read, err)
	}
	readOutput := structuredOutput(t, read.StructuredContent)
	assertStructuredViewport(t, readOutput["viewport"], true, true, 0, 0)
	if output, ok := readOutput["output"].(string); !ok || !strings.Contains(output, viewportMeasuredEmpty) {
		t.Fatalf("structured read output = %#v", readOutput["output"])
	}
}

func structuredOutput(t *testing.T, value any) map[string]any {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	if err := json.Unmarshal(body, &output); err != nil {
		t.Fatal(err)
	}
	return output
}

func assertStructuredViewport(t *testing.T, value any, available, selected bool, line, to int) {
	t.Helper()
	viewport, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("viewport = %#v", value)
	}
	if viewport["source"] != "P007.md" || viewport["available"] != available || viewport["selected"] != selected {
		t.Fatalf("viewport fields = %#v", viewport)
	}
	intervals, ok := viewport["intervals"].([]any)
	if !ok {
		t.Fatalf("viewport intervals = %#v", viewport["intervals"])
	}
	if line == 0 {
		if len(intervals) != 0 {
			t.Fatalf("viewport intervals = %#v, want empty", intervals)
		}
		return
	}
	if len(intervals) != 1 {
		t.Fatalf("viewport intervals = %#v", intervals)
	}
	interval := intervals[0].(map[string]any)
	if interval["line"] != float64(line) || interval["to"] != float64(to) {
		t.Fatalf("viewport interval = %#v, want %d-%d", interval, line, to)
	}
}

// A struct tag cannot interpolate a slice, so the escalate tool's prefix
// vocabulary is written out by hand. This is what holds it to the one list the
// picker actually offers.
func TestEscalatePrefixSchemaEnumeratesTheAssemblyPrefixes(t *testing.T) {
	field, ok := reflect.TypeFor[escalateInput]().FieldByName("BranchPrefix")
	if !ok {
		t.Fatal("escalateInput has no BranchPrefix field")
	}
	description := field.Tag.Get("jsonschema")
	prefixes := assembly.Prefixes()
	if want := "one of " + strings.Join(prefixes, ", "); description != want {
		t.Fatalf("branch_prefix description = %q, want %q", description, want)
	}
}

// share_page stages a file and reports where; qrouton never sends it anywhere,
// so the handler's whole contract is the path it hands back.
func TestSharePageStagesAPageInsideTheSession(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "thoughts", "shared"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join("thoughts", "shared", "spec.md")
	if err := os.WriteFile(filepath.Join(dir, source), []byte("# Spec\n\nProse.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	message, err := sharePage(dir, sharePageInput{Path: source})
	if err != nil {
		t.Fatalf("sharePage: %v", err)
	}
	page := filepath.Join(sessionpaths.SharePages(dir), "thoughts-shared-spec.html")
	if !strings.Contains(message, page) {
		t.Errorf("message %q does not name %q", message, page)
	}
	body, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("read staged page: %v", err)
	}
	if !strings.HasPrefix(string(body), "<title>Spec</title>") {
		t.Errorf("staged page opens %.40q", body)
	}
}

func TestSharePageRefusesAPathOutsideTheSession(t *testing.T) {
	if _, err := sharePage(t.TempDir(), sharePageInput{Path: "../elsewhere.md"}); err == nil {
		t.Error("shared a document from outside the session")
	}
}

// The wire carries the role the agent gave, empty included: the default belongs
// to the desktop side, which is the one place that knows what a role means.
func TestAddReposSendsEveryNameAndRoleAsGiven(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.addResult = workbench.AddReposResult{Added: []string{"org/svc"}}

	if _, err := m.addRepos(context.Background(), addReposInput{Repos: []repoAdditionInput{
		{Name: "org/svc", Role: "editing"},
		{Name: "docs"},
	}}); err != nil {
		t.Fatal(err)
	}
	if len(host.adds) != 1 {
		t.Fatalf("host received %d add requests, want one", len(host.adds))
	}
	want := []workbench.RepoAddition{{Name: "org/svc", Role: "editing"}, {Name: "docs"}}
	if !reflect.DeepEqual(host.adds[0].Repos, want) {
		t.Fatalf("request = %+v, want %+v", host.adds[0].Repos, want)
	}
}

func TestAddReposRefusesAnEmptyListAndABlankName(t *testing.T) {
	m, host, _ := newTestManager(t)

	if _, err := m.addRepos(context.Background(), addReposInput{}); !errors.Is(err, ErrReposRequired) {
		t.Fatalf("an empty list = %v, want ErrReposRequired", err)
	}
	_, err := m.addRepos(context.Background(), addReposInput{Repos: []repoAdditionInput{{Name: "  "}}})
	if !errors.Is(err, ErrRepoNameRequired) {
		t.Fatalf("a blank name = %v, want ErrRepoNameRequired", err)
	}
	if len(host.adds) != 0 {
		t.Fatalf("a refused call still reached the workbench: %+v", host.adds)
	}
}

// A promotion is not an addition. An agent that reads "added" for a repo it
// already had would conclude it has two checkouts.
func TestAddReposReportsPromotedHeldAndAddedApart(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.addResult = workbench.AddReposResult{
		Status:   workbench.AddReposConfirmed,
		Added:    []string{"org/extra"},
		Promoted: []string{"org/docs"},
		Held:     []string{"org/svc"},
		Dropped:  []string{"org/kraken"},
	}

	message, err := m.addRepos(context.Background(), addReposInput{Repos: []repoAdditionInput{
		{Name: "org/extra"}, {Name: "org/docs", Role: "editing"}, {Name: "org/svc"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Added org/extra.",
		"Took up for editing on the session branch: org/docs.",
		"Already held, unchanged: org/svc.",
		"Not added, because the user did not approve them: org/kraken.",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("message %q does not carry %q", message, want)
		}
	}
}

func TestAddReposSaysWhenNothingChanged(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.addResult = workbench.AddReposResult{
		Status: workbench.AddReposConfirmed, Held: []string{"org/svc"}}

	message, err := m.addRepos(context.Background(), addReposInput{
		Repos: []repoAdditionInput{{Name: "org/svc"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "Already held") {
		t.Fatalf("message = %q", message)
	}

	host.addResult = workbench.AddReposResult{Status: workbench.AddReposConfirmed}
	message, err = m.addRepos(context.Background(), addReposInput{
		Repos: []repoAdditionInput{{Name: "org/svc"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if message != noReposChanged {
		t.Fatalf("an add that changed nothing said %q", message)
	}
}

// The tab is the user's only view of a call that blocks for minutes, so it is
// opened before the wait and replaced with the outcome — one name, not two tabs.
func TestAddReposOpensTheReposTabAndReplacesItWithTheOutcome(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.addResult = workbench.AddReposResult{
		Status: workbench.AddReposConfirmed, Added: []string{"org/extra"}}

	if _, err := m.addRepos(context.Background(), addReposInput{
		Repos: []repoAdditionInput{{Name: "org/extra"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(host.opens) != 2 {
		t.Fatalf("opened %d windows, want the repos tab then its replacement: %+v", len(host.opens), host.opens)
	}
	// The tab is the user's cue to go and answer, so it has to name what is being
	// asked for and say that something is waiting on them.
	if !strings.Contains(host.opens[0].Content, "org/extra") ||
		!strings.Contains(host.opens[0].Content, "waiting for your answer") {
		t.Errorf("the waiting tab = %q", host.opens[0].Content)
	}
	if !strings.Contains(host.opens[1].Content, "Added org/extra.") {
		t.Errorf("the outcome tab = %q", host.opens[1].Content)
	}
	for i, opened := range host.opens {
		if opened.Kind != workbench.KindDocument {
			t.Errorf("window %d is %v, want a document", i, opened.Kind)
		}
		// Agent tabs are background by default, and this one wants nothing.
		if opened.Select || opened.Attention {
			t.Errorf("window %d takes the screen: select=%v attention=%v", i, opened.Select, opened.Attention)
		}
	}
	// One logical name, so the outcome replaces the waiting line rather than
	// stacking a second tab beside it: the first window is closed as it goes.
	if !reflect.DeepEqual(host.closes, []string{host.ids[0]}) {
		t.Fatalf("closed %v, want the waiting tab %q replaced", host.closes, host.ids[0])
	}
}

func TestAddReposSurfacesAWorkbenchFailureAsAToolError(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.addErr = errors.New("no repository \"org/kraken\" among the owners searched")

	_, err := m.addRepos(context.Background(), addReposInput{
		Repos: []repoAdditionInput{{Name: "org/kraken"}},
	})
	if err == nil {
		t.Fatal("a failed add returned no error")
	}
	if !strings.Contains(err.Error(), "org/kraken") {
		t.Fatalf("error = %v, want the workbench's own refusal", err)
	}
	// The tab says so too, rather than leaving the fetching line up.
	if len(host.opens) != 2 || !strings.Contains(host.opens[1].Content, "failed") {
		t.Fatalf("the tab was not replaced with the failure: %+v", host.opens)
	}
}

// A refusal and a timeout are outcomes the agent can act on, not failures: it
// must not retry them as though the call had broken.
func TestAddReposReportsADeclinedProposalAndATimeoutAsNormalResults(t *testing.T) {
	for _, tc := range []struct {
		status string
		says   string
	}{
		{workbench.AddReposDeclined, "declined"},
		{workbench.AddReposExpired, "timed out"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			m, host, _ := newTestManager(t)
			host.addResult = workbench.AddReposResult{Status: tc.status}

			message, err := m.addRepos(context.Background(), addReposInput{
				Repos: []repoAdditionInput{{Name: "org/extra"}},
			})
			if err != nil {
				t.Fatalf("a %s proposal returned an error: %v", tc.status, err)
			}
			if !strings.Contains(message, tc.says) {
				t.Fatalf("message = %q, want it to say %q", message, tc.says)
			}
		})
	}
}

// The deadline is the agent's, and it travels with the request so the workbench
// never draws an overlay for an answer nobody is waiting for.
func TestAddReposSendsItsDeadlineWithTheProposal(t *testing.T) {
	m, host, _ := newTestManager(t)
	host.addResult = workbench.AddReposResult{Status: workbench.AddReposConfirmed}

	before := time.Now()
	if _, err := m.addRepos(context.Background(), addReposInput{
		Repos: []repoAdditionInput{{Name: "org/extra"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(host.adds) != 1 {
		t.Fatalf("host received %d requests", len(host.adds))
	}
	got := host.adds[0].Deadline
	if got.Before(before.Add(addReposTimeout-time.Minute)) || got.After(time.Now().Add(addReposTimeout)) {
		t.Fatalf("deadline = %s, want about %s out", got, addReposTimeout)
	}
}
