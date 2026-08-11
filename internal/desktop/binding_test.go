package desktop

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/workbench"
)

// The pages call these services by their fully qualified Go names, which Wails
// derives from the package path and type name. Nothing checks those strings at
// build time: get one wrong and the window renders empty with no error
// anywhere.
func TestThePagesNameTheirServicesExactly(t *testing.T) {
	for _, tc := range []struct {
		service any
		page    string
		methods []string
	}{
		{Term{}, "index.html", []string{"Start", "Write", "Resize"}},
		{Windows{}, "terminal/index.html", []string{"Start", "Write", "Resize"}},
		{Windows{}, "document/index.html", []string{"Content"}},
	} {
		t.Run(tc.page, func(t *testing.T) {
			typ := reflect.TypeOf(tc.service)
			want := typ.PkgPath() + "." + typ.Name()

			page, err := assetFS.ReadFile(assetRoot + "/" + tc.page)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(page), `"`+want+`"`) {
				t.Fatalf("the page does not bind %q", want)
			}
			for _, method := range tc.methods {
				if _, ok := reflect.PointerTo(typ).MethodByName(method); !ok {
					t.Fatalf("the page calls %s.%s, which is not an exported method", typ.Name(), method)
				}
				if !strings.Contains(string(page), "."+method) {
					t.Fatalf("the page never calls %s.%s", typ.Name(), method)
				}
			}
		})
	}
}

// An agent window's page hears one stream, so its event names have to carry the
// window id the Go side appends.
func TestTheAgentTerminalPageSubscribesToItsOwnWindowsEvents(t *testing.T) {
	page, err := assetFS.ReadFile(assetRoot + "/terminal/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"` + windowDataEvent + `" + id`, `"` + windowExitEvent + `" + id`} {
		if !strings.Contains(string(page), want) {
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
	handler := http.FileServerFS(assets)
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
		if !strings.Contains(recorder.Body.String(), "wails/runtime.js") {
			t.Fatalf("%s served something that is not a workbench page", url)
		}
	}
}

// The format the page branches on is the port's value, spelled again in
// JavaScript. Nothing checks that at build time.
func TestTheDocumentPageKnowsTheDiffFormat(t *testing.T) {
	page, err := assetFS.ReadFile(assetRoot + "/document/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `=== "`+string(workbench.FormatDiff)+`"`) {
		t.Fatalf("the document page does not branch on the %q format", workbench.FormatDiff)
	}
}
