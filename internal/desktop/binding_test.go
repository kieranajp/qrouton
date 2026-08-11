package desktop

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/theme"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// builtBundle concatenates every file frontend() serves — the tree the
// binary actually embeds — so callers grep what shipped, not what fed it.
func builtBundle(t *testing.T) string {
	t.Helper()
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	if err := fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		all.Write(b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return all.String()
}

// A page names its Go service by the fully qualified name Wails derives from
// the package path and type name. The bundle is read rather than the source
// because a rename and a tree-shaken-away call both render an empty window,
// and only one of the two is visible before the build.
func TestTheBuiltPagesNameTheirServicesExactly(t *testing.T) {
	bundle := builtBundle(t)
	for _, tc := range []struct {
		service any
		methods []string
	}{
		{Term{}, []string{"Start", "Write", "Resize"}},
		{Windows{}, []string{
			"Start", "Write", "Resize", "Content", "Surfaces", "Close", "OpenShell", "OpenDocument",
			"OpenPicker",
		}},
	} {
		typ := reflect.TypeOf(tc.service)
		qualified := typ.PkgPath() + "." + typ.Name()
		for _, method := range tc.methods {
			t.Run(typ.Name()+"."+method, func(t *testing.T) {
				if _, ok := reflect.PointerTo(typ).MethodByName(method); !ok {
					t.Fatalf("a page calls %s.%s, which is not an exported method", typ.Name(), method)
				}
				if !strings.Contains(bundle, qualified+"."+method) {
					t.Fatalf("the built pages no longer call %s.%s", typ.Name(), method)
				}
			})
		}
	}
}

// An agent window's page hears one stream, so its event names have to carry the
// window id the Go side appends.
func TestTheAgentTerminalPageSubscribesToItsOwnWindowsEvents(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "window-terminal.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"` + windowDataEvent + `" + id`, `"` + windowExitEvent + `" + id`} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("the terminal page does not subscribe to %s", want)
		}
	}
}

// Every window URL is served as-is. A path the handler redirects instead of
// serving comes up as a blank window with nothing reported anywhere, so each
// page's own URL is checked against the handler that will serve it.
func TestEveryWindowURLIsServedWithoutARedirect(t *testing.T) {
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	handler := assetHandler(assets)
	for _, url := range []string{
		frontendRoot,
		terminalPage + windowIDQuery + "window-1",
		documentPage + windowIDQuery + "window-2",
	} {
		request := httptest.NewRequest(http.MethodGet, url, nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s answered %d (%q), not the page", url, recorder.Code, recorder.Header().Get("Location"))
		}
		if !strings.Contains(recorder.Body.String(), `<script type="module"`) {
			t.Fatalf("%s served something that is not a built workbench page", url)
		}
	}
}

func TestTheHandlerServesThePaletteBesideTheEmbeddedTree(t *testing.T) {
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	assetHandler(assets).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, theme.Path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s answered %d", theme.Path, recorder.Code)
	}
	if got := recorder.Header().Get(contentTypeHeader); got != theme.MediaType {
		t.Fatalf("served the palette as %q, not %q", got, theme.MediaType)
	}
	for _, name := range []string{theme.RoleAccentAction, theme.RoleStateWaiting} {
		if !strings.Contains(recorder.Body.String(), "--"+name+":") {
			t.Fatalf("the served palette declares no --%s", name)
		}
	}
}

func TestEveryPageLinksThePalette(t *testing.T) {
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{frontendRoot, terminalPage, documentPage} {
		recorder := httptest.NewRecorder()
		assetHandler(assets).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, url, nil))
		if !strings.Contains(recorder.Body.String(), theme.Path) {
			t.Fatalf("%s does not link %s", url, theme.Path)
		}
	}
}

// The formats the pane registry keys on are the port's values, spelled again in
// JavaScript. Nothing checks that at build time, and a format no pane claims
// draws as plain text rather than erroring.
func TestThePaneRegistryDrawsEveryDocumentFormat(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "lib/panes/index.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []workbench.DocumentFormat{workbench.FormatDiff, workbench.FormatMarkdown} {
		if !strings.Contains(string(source), string(format)+":") {
			t.Errorf("the pane registry has no pane for the %q format", format)
		}
	}
}

// The page's NOTHING supplies one default per status.Fields key, and a field
// added without one drops off the pane silently rather than erroring. Nested
// structs render through their own components, so only the direct keys count.
func TestTheChromePageDefaultsEveryField(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "lib/chrome.svelte.js")
	if err != nil {
		t.Fatal(err)
	}
	typ := reflect.TypeOf(status.Fields{})
	for i := 0; i < typ.NumField(); i++ {
		tag, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if tag == "" || tag == "-" {
			continue
		}
		if !strings.Contains(string(source), tag+":") {
			t.Fatalf("chrome.svelte.js's NOTHING has no default for status.Fields.%s (json %q)", typ.Field(i).Name, tag)
		}
	}
}
