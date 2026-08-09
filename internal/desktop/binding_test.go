package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/theme"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// The pages call these services by their fully qualified Go names, which Wails
// derives from the package path and type name. Nothing checks those strings at
// build time: get one wrong and the window renders empty with no error
// anywhere. The bundle only rewrites what surrounds these literals, so the
// source is what is read.
func TestThePagesNameTheirServicesExactly(t *testing.T) {
	for _, tc := range []struct {
		service any
		module  string
		methods []string
	}{
		{Term{}, "lib/conversation.js", []string{"Start", "Write", "Resize"}},
		{Windows{}, "window-terminal.js", []string{"Start", "Write", "Resize"}},
		{Windows{}, "window-document.js", []string{"Content"}},
	} {
		t.Run(tc.module, func(t *testing.T) {
			typ := reflect.TypeOf(tc.service)
			want := typ.PkgPath() + "." + typ.Name()

			source, err := os.ReadFile(frontendSource + tc.module)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(source), `"`+want+`"`) {
				t.Fatalf("the page does not bind %q", want)
			}
			for _, method := range tc.methods {
				if _, ok := reflect.PointerTo(typ).MethodByName(method); !ok {
					t.Fatalf("the page calls %s.%s, which is not an exported method", typ.Name(), method)
				}
				if !strings.Contains(string(source), "."+method) {
					t.Fatalf("the page never calls %s.%s", typ.Name(), method)
				}
			}
		})
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

// The format the page branches on is the port's value, spelled again in
// JavaScript. Nothing checks that at build time.
func TestTheDocumentPageKnowsTheDiffFormat(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "window-document.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `=== "`+string(workbench.FormatDiff)+`"`) {
		t.Fatalf("the document page does not branch on the %q format", workbench.FormatDiff)
	}
}
