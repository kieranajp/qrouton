package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
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

func deckHandler(root string) http.Handler {
	lookup := func(token string) (string, string, bool) {
		if token != "goodtoken" {
			return "", "", false
		}
		return root, "thoughts/decks", true
	}
	return assetHandler(fstest.MapFS{"index.html": {Data: []byte("page")}}, lookup)
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
	lookup := func(string) (string, string, bool) { return root, "thoughts/decks", true }
	climb := "/deck/goodtoken/../../../" + filepath.Base(outside)

	recorder := httptest.NewRecorder()
	deckAsset(lookup).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, climb, nil))
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
	assetHandler(fstest.MapFS{}, func(string) (string, string, bool) { return "", "", false }).
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
