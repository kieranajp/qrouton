package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// deckSession lays out a session the way one is on disk: a root, a thoughts
// directory beside a deck, and a file outside the session for traversal to aim
// at.
func deckSession(t *testing.T) (root, outside string) {
	t.Helper()
	home := t.TempDir()
	root = filepath.Join(home, "session")
	decks := filepath.Join(root, "thoughts", "decks")
	if err := os.MkdirAll(decks, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"shot.png":   "\x89PNG picture",
		"clip.mp4":   "video",
		"clip.webm":  "video",
		"notes.md":   "# not media",
		"deck.md":    "---\nmarp: true\n---\n",
		"deck.marps": "",
	} {
		if err := os.WriteFile(filepath.Join(decks, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	outside = filepath.Join(home, "secret.png")
	if err := os.WriteFile(outside, []byte("not the session's"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, outside
}

// sessionFile stands in for the core's opener: the file at source inside root,
// or nothing when it resolves outside or is a directory.
func sessionFile(root, source string) (*os.File, error) {
	name, err := launch.ResolveSessionFile(root, source)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, core.ErrNoSuchAsset
	}
	return file, nil
}

// deckOpenerAt opens files beside the deck in root's thoughts for goodtoken.
func deckOpenerAt(root string) deckOpener {
	return func(token, rel string) (*os.File, error) {
		if token != "goodtoken" {
			return nil, core.ErrNoSuchAsset
		}
		return sessionFile(root, path.Join("thoughts/decks", rel))
	}
}

// galleryOpenerOf opens the index'th of sources inside root, refusing what is
// not gallery media.
func galleryOpenerOf(root string, sources ...string) galleryOpener {
	return func(_ string, index int) (*os.File, string, error) {
		if index < 1 || index > len(sources) {
			return nil, "", core.ErrNoSuchAsset
		}
		file, err := sessionFile(root, sources[index-1])
		if err != nil {
			return nil, "", err
		}
		media, known := workbench.AssetMedia(file.Name())
		if !known {
			_ = file.Close()
			return nil, "", core.ErrNoSuchAsset
		}
		return file, media.Type, nil
	}
}

func deckHandler(root string) http.Handler {
	return assetHandler(fstest.MapFS{"index.html": {Data: []byte("page")}}, nil, deckOpenerAt(root))
}

func TestTheDeckRouteServesItsOwnMediaAndNothingElse(t *testing.T) {
	root, _ := deckSession(t)
	handler := deckHandler(root)
	for _, tc := range []struct {
		name, path, media string
		status            int
	}{
		{"a picture beside the deck", "/deck/goodtoken/shot.png", "image/png", http.StatusOK},
		{"a video beside the deck", "/deck/goodtoken/clip.mp4", "video/mp4", http.StatusOK},
		{"a WebM beside the deck", "/deck/goodtoken/clip.webm", "video/webm", http.StatusOK},
		{"a file that is not media", "/deck/goodtoken/notes.md", "", http.StatusNotFound},
		{"a file that does not exist", "/deck/goodtoken/absent.png", "", http.StatusNotFound},
		{"a token no window holds", "/deck/othertoken/shot.png", "", http.StatusNotFound},
		{"no token at all", "/deck/shot.png", "", http.StatusNotFound},
		{"a token and no file", "/deck/goodtoken/", "", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if recorder.Code != tc.status {
				t.Fatalf("%s answered %d, want %d", tc.path, recorder.Code, tc.status)
			}
			if got := recorder.Header().Get(contentTypeHeader); tc.media != "" && got != tc.media {
				t.Fatalf("%s served as %q, want %q", tc.path, got, tc.media)
			}
		})
	}
}

// The mux cleans a dotted URL before any handler sees it, so containment is
// asserted against the handler itself as well as through the route.
func TestTheDeckRouteRefusesToClimbOutOfTheSession(t *testing.T) {
	root, outside := deckSession(t)
	open := deckOpenerAt(root)
	climb := "/deck/goodtoken/../../../" + filepath.Base(outside)

	recorder := httptest.NewRecorder()
	deckAsset(open).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, climb, nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("the handler answered %d for a path outside the session", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	deckHandler(root).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, climb, nil))
	if recorder.Code == http.StatusOK {
		t.Fatalf("the route served a path outside the session")
	}
	if strings.Contains(recorder.Body.String(), "not the session's") {
		t.Fatalf("the route handed back a file outside the session")
	}
}

func TestADeckAddressesItsOwnWindowAndNoOther(t *testing.T) {
	root, _ := deckSession(t)
	recorder := httptest.NewRecorder()
	// A handler whose lookup answers nothing is every window that is not an open
	// deck: another session's tab, a terminal, a plain markdown document.
	assetHandler(fstest.MapFS{}, nil, func(string, string) (*os.File, error) { return nil, core.ErrNoSuchAsset }).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/deck/goodtoken/shot.png", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("a window that is no deck served its neighbours: %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	deckHandler(root).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/deck/goodtoken/shot.png", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("the deck's own picture answered %d", recorder.Code)
	}
}

func TestTheScaleRouteAnswersTheStoredScaleUncached(t *testing.T) {
	for _, tc := range []struct {
		lookup func() int
		want   string
	}{
		{nil, "--ui-scale: 1;"},
		{func() int { return 150 }, "--ui-scale: 1.5;"},
		{func() int { return 80 }, "--ui-scale: 0.8;"},
	} {
		recorder := httptest.NewRecorder()
		assetHandler(fstest.MapFS{}, tc.lookup, nil).
			ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, uiScalePath, nil))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), tc.want) {
			t.Fatalf("answered %d %q, want %q", recorder.Code, recorder.Body.String(), tc.want)
		}
		if got := recorder.Header().Get(cacheControlHeader); got != cacheControlNoStore {
			t.Fatalf("Cache-Control = %q", got)
		}
	}
}

func TestTheGalleryRouteAnswersRangeRequestsForVideo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sim.mov"), []byte("0123456789abcdefghij"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := assetHandler(fstest.MapFS{"index.html": {Data: []byte("page")}}, nil, nil, galleryOpenerOf(root, "sim.mov"))
	request := httptest.NewRequest(http.MethodGet, "/gallery/token/1", nil)
	request.Header.Set("Range", "bytes=0-9")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Header().Get("Content-Range") != "bytes 0-9/20" ||
		response.Header().Get("Content-Type") != "video/quicktime" || response.Body.String() != "0123456789" {
		t.Fatalf("range = %d, %v, %q", response.Code, response.Header(), response.Body.String())
	}
}

func TestMediaRoutesKeepTheirDistinctAdmissionAndHeaders(t *testing.T) {
	root, _ := deckSession(t)
	gallery := galleryAsset(galleryOpenerOf(root, filepath.Join("thoughts", "decks", "shot.png"), filepath.Join("thoughts", "decks", "clip.webm")))
	for _, tc := range []struct {
		name    string
		handler http.Handler
		path    string
		status  int
		private bool
	}{
		{"gallery image", gallery, "/gallery/token/1", http.StatusOK, true},
		{"deck image", deckHandler(root), "/deck/goodtoken/shot.png", http.StatusOK, false},
		{"gallery excludes deck-only media", gallery, "/gallery/token/2", http.StatusNotFound, false},
		{"deck includes extra media", deckHandler(root), "/deck/goodtoken/clip.webm", http.StatusOK, false},
		{"gallery requires canonical index", gallery, "/gallery/token/01", http.StatusNotFound, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			tc.handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d", response.Code, tc.status)
			}
			if tc.status == http.StatusOK {
				if got := response.Header().Get(cacheControlHeader); (got == cacheControlNoStore) != tc.private {
					t.Fatalf("Cache-Control = %q, private = %v", got, tc.private)
				}
				if got := response.Header().Get(contentTypeOptionsHeader); (got == contentTypeNoSniff) != tc.private {
					t.Fatalf("X-Content-Type-Options = %q, private = %v", got, tc.private)
				}
			}
		})
	}
	request := httptest.NewRequest(http.MethodGet, "/deck/goodtoken/clip.mp4", nil)
	request.Header.Set("Range", "bytes=1-3")
	response := httptest.NewRecorder()
	deckHandler(root).ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.String() != "ide" {
		t.Fatalf("deck range = %d %q", response.Code, response.Body.String())
	}
}
