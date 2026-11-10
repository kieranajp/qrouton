package desktop

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/api"
	"github.com/kieranajp/qrouton/internal/api/pagetest"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/status"
	"go.uber.org/goleak"
)

func filled[T any]() T {
	var v T
	pagetest.FillGo(&v)
	return v
}

// translated serves a core whose hub the test drives, with the translator
// emitting to r.
func translated(t *testing.T) (*api.Hub, *fakeRenderer) {
	t.Helper()
	opts, _ := testOptions(t)
	opts.Config = &config.Config{Root: opts.Root}
	r := newFakeRenderer()
	kernel := testWorkbench(t, r, &opts)
	translate(t, kernel, &opts, r)
	return opts.hub, r
}

func emitted(t *testing.T, r *fakeRenderer, name string) []byte {
	t.Helper()
	var payload any
	waitFor(t, name, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		var ok bool
		payload, ok = r.events[name]
		return ok
	})
	out, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func marshalled(t *testing.T, v any) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Catches an event whose page payload drifts from what the core's own type
// marshalled to, or that reaches the page under another name.
func TestEveryEventPageJSONMatchesTheCoreType(t *testing.T) {
	hub, r := translated(t)
	ollama, fields, tree := filled[core.OllamaStatus](), filled[status.Fields](), filled[status.AgentTree]()
	surfaces, doc, diagram := filled[core.Surfaces](), filled[core.Document](), filled[core.RenderedDiagram]()
	refresh, progress := filled[core.RefreshEvent](), filled[core.ProgressEvent]()
	for _, tc := range []struct {
		name    string
		publish func()
		want    any
	}{
		{ollamaEvent, func() { hub.Ollama(ollama) }, ollama},
		{chromeEvent, func() { hub.Chrome(fields) }, fields},
		{agentTreeEvent + "octopus", func() { hub.AgentTree("octopus", tree) }, tree},
		{windowsEvent, func() { hub.WindowsOpened(surfaces) }, surfaces},
		{windowContentEvent + "window-3", func() { hub.WindowContent("window-3", doc) }, doc},
		{windowDiagramEvent + "window-4", func() { hub.WindowDiagram("window-4", diagram) }, diagram},
		{ptyExitEvent + "term-1", func() { hub.TerminalExit(core.TerminalConversation, "term-1", 3) }, 3},
		{windowExitEvent + "window-5", func() { hub.TerminalExit(core.TerminalTab, "window-5", 4) }, 4},
		{reposRefreshEvent, func() { hub.RepositoryRefresh(refresh) }, refresh},
		{assemblyProgressEvent, func() { hub.AssemblyProgress(progress) }, progress},
		{assemblyRequestedEvent, func() { hub.AssemblyRequested("https://linear.app/x") }, "https://linear.app/x"},
		{orgsChangedEvent, func() { hub.OrgsChanged([]string{"acme", "beta"}) }, []string{"acme", "beta"}},
		{uiScaleEvent, func() { hub.UIScale(110) }, 110},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.publish()
			pagetest.Same(t, emitted(t, r, tc.name), marshalled(t, tc.want))
		})
	}
}

// Catches a terminal data event whose encoded, replay, first or exit_code
// differ from what the old Chunk sent for the same bytes, frozen before it went,
// or that arrives under the other kind of terminal's name.
func TestTerminalOutputPageJSONMatchesTheOldChunk(t *testing.T) {
	hub, r := translated(t)
	code := 2
	hub.TerminalOutput(core.TerminalTab, "window-1", core.Output{Data: []byte("\x1b[31mred\xff"), Replay: true, First: true, ExitCode: &code})
	hub.TerminalOutput(core.TerminalConversation, "term-1", core.Output{Data: []byte("live")})
	page := map[string]map[string]any{}
	for name, event := range map[string]string{"replay": windowDataEvent + "window-1", "live": ptyDataEvent + "term-1"} {
		var got map[string]any
		if err := json.Unmarshal(emitted(t, r, event), &got); err != nil {
			t.Fatal(err)
		}
		chunk := map[string]any{}
		for _, key := range []string{"encoded", "replay", "first", "exit_code"} {
			if value, ok := got[key]; ok && value != false && value != nil {
				chunk[key] = value
			}
		}
		page[name] = chunk
	}
	pagetest.Golden(t, filepath.Join("testdata", "pagejson", "chunks.json"), marshalled(t, page))
}

// Catches a translator or subscriber goroutine that outlives Close.
func TestClosingWithALiveTranslatorLeavesNoGoroutine(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	opts, _ := testOptions(t)
	opts.Config = &config.Config{Root: opts.Root}
	r := newFakeRenderer()
	opts.hub = api.NewHub()
	kernel := core.New(coreOptions(opts, func() {}), newLocalClient(r, opts.Launcher, nil), opts.hub)
	in, err := startInProcess(kernel, opts.hub, opts.Socket, "", r.Emit)
	if err != nil {
		t.Fatal(err)
	}
	opts.hub.UIScale(90)
	emitted(t, r, uiScaleEvent)
	in.Close()
}
