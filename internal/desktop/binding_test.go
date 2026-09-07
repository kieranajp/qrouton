package desktop

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

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
	for _, service := range frontendServices {
		typ := reflect.TypeOf(service)
		qualified := typ.PkgPath() + "." + typ.Name()
		methods := reflect.PointerTo(typ)
		for i := 0; i < methods.NumMethod(); i++ {
			method := methods.Method(i).Name
			t.Run(typ.Name()+"."+method, func(t *testing.T) {
				if !strings.Contains(bundle, qualified+"."+method) {
					t.Fatalf("the built pages no longer call %s.%s", typ.Name(), method)
				}
			})
		}
	}
}

func TestAStaleFrontendIsRejectedBeforeTheWindowOpens(t *testing.T) {
	err := validateFrontend(fstest.MapFS{"index.html": {Data: []byte("old workbench")}})
	if !errors.Is(err, ErrStaleFrontend) {
		t.Fatalf("validateFrontend error = %v, want ErrStaleFrontend", err)
	}
	if !strings.Contains(err.Error(), "make front") {
		t.Fatalf("stale frontend error gives no repair: %v", err)
	}
}

func TestTheConversationURLIsServedWithoutARedirect(t *testing.T) {
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	handler := assetHandler(assets, nil)
	request := httptest.NewRequest(http.MethodGet, frontendRoot, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s answered %d (%q), not the page", frontendRoot, recorder.Code, recorder.Header().Get("Location"))
	}
	if !strings.Contains(recorder.Body.String(), `<script type="module"`) {
		t.Fatalf("%s served something that is not the built workbench page", frontendRoot)
	}
}

func TestBuiltAssetsContainNoStandaloneAgentPages(t *testing.T) {
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"terminal/index.html", "document/index.html"} {
		if _, err := fs.Stat(assets, page); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("retired standalone page %q remains in built assets (%v)", page, err)
		}
	}
}

func TestTheHandlerServesThePaletteBesideTheEmbeddedTree(t *testing.T) {
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	assetHandler(assets, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, theme.Path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s answered %d", theme.Path, recorder.Code)
	}
	if got := recorder.Header().Get(contentTypeHeader); got != theme.MediaType {
		t.Fatalf("served the palette as %q, not %q", got, theme.MediaType)
	}
	for _, name := range []string{theme.RoleAccentAction, theme.RoleStateWaiting, theme.RoleActionDestructive} {
		if !strings.Contains(recorder.Body.String(), "--"+name+":") {
			t.Fatalf("the served palette declares no --%s", name)
		}
	}
}

func TestTheConversationPageLinksThePalette(t *testing.T) {
	assets, err := frontend()
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	assetHandler(assets, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, frontendRoot, nil))
	if !strings.Contains(recorder.Body.String(), theme.Path) {
		t.Fatalf("%s does not link %s", frontendRoot, theme.Path)
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

// Without this one wire the toggle beside a held reference row answers, the tally
// counts it, and the session is never told. The bundle is read rather than the
// source: a page built before the wire existed fails in exactly the same silence.
func TestThePickerSendsTheRowsItWasToldToTakeUp(t *testing.T) {
	wire := regexp.MustCompile(`upgrades:[\w$.]*\.upgrading`)
	if !wire.MatchString(builtBundle(t)) {
		t.Fatal("the built pages confirm the picker without the rows to take up for editing")
	}
}
