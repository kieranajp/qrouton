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
	handler := assetHandler(assets)
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
	assetHandler(assets).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, theme.Path, nil))
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
	assetHandler(assets).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, frontendRoot, nil))
	if !strings.Contains(recorder.Body.String(), theme.Path) {
		t.Fatalf("%s does not link %s", frontendRoot, theme.Path)
	}
}

// Enter confirms, which only holds while the dialog puts the keyboard on the
// button that confirms. Focus is wiring the page owns and no harness here can
// observe, so the wiring is what is read.
func TestTheConfirmDialogFocusesTheButtonThatConfirms(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "lib/shell/Confirm.svelte")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if !strings.Contains(body, `querySelector("[data-confirm]")`) {
		t.Fatal("the dialog does not focus the button carrying the confirm hook")
	}
	buttons := regexp.MustCompile(`(?s)<Button[^>]*>`).FindAllString(body, -1)
	var hooked []string
	for _, button := range buttons {
		if strings.Contains(button, "data-confirm") {
			hooked = append(hooked, button)
		}
	}
	if len(hooked) != 1 {
		t.Fatalf("%d of the dialog's buttons carry the confirm hook, want exactly the confirming one", len(hooked))
	}
	if !strings.Contains(hooked[0], `variant="destructive"`) {
		t.Fatalf("the confirm hook sits on %q, not on the destructive button", hooked[0])
	}
}

// Enter advances the dialog without going through its button, so a screen that
// will not go forward has to gate both. Neither is observable in a harness with
// no components, so the wiring is what is read.
func TestTheDialogGatesEnterWithItsAdvanceButton(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "lib/assembly/Dialog.svelte")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	if !strings.Contains(body, "else if (canAdvance) onPrimary();") {
		t.Fatal("Enter reaches onPrimary without asking whether the dialog can advance")
	}
	if !strings.Contains(body, "disabled={busy || !canAdvance}") {
		t.Fatal("the advance button stays live on a dialog that cannot advance")
	}
}

// Nothing joins the tested predicate to the gated dialog, so a page that stops
// passing either attribute reopens the trap with every check still green.
func TestTheOwnersQuestionHoldsTheDialogItDraws(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "lib/firstrun/FirstRunOverlay.svelte")
	if err != nil {
		t.Fatal(err)
	}
	for _, wiring := range []string{"canAdvance={!blocked}", "status={blocked || flow.status}"} {
		if !strings.Contains(string(source), wiring) {
			t.Fatalf("first run does not pass %s, so an unanswered owners question advances", wiring)
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

func TestTheChromePageDefaultsTheNestedAgentRecordList(t *testing.T) {
	source, err := os.ReadFile(frontendSource + "lib/chrome.svelte.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `agents: { provider: "", agents: [] }`) {
		t.Fatal("chrome.svelte.js has no non-null default for the selected agent panel")
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
