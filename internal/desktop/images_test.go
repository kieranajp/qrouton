package desktop

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kieranajp/qrouton/internal/workbench"
)

func TestImageGalleryRegistryContentReadAndAssetRoute(t *testing.T) {
	w, _ := testWindows(t)
	root := w.shown().root()
	for _, name := range []string{"first.PNG", "second.webp"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("image "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	id, err := w.openWindow(w.shown(), workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatImages,
		Images: []workbench.ImageRef{{Source: "first.PNG"}, {Source: "second.webp"}, {Source: "first.PNG"}}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := w.Content(id)
	if err != nil || page.CurrentIndex != 1 || page.Revision != 1 || len(page.Images) != 3 || page.Images[0].URL == page.Images[2].URL {
		t.Fatalf("content = %+v, %v", page, err)
	}
	text, err := w.readWindow(id, false)
	if err != nil || text != "1. first.PNG\n2. second.webp\n3. first.PNG\nCurrent image: 1 of 3" {
		t.Fatalf("read = %q, %v", text, err)
	}
	handler := assetHandler(fstest.MapFS{"index.html": {Data: []byte("page")}}, nil, w.imageAsset)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, page.Images[1].URL, nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/webp" || response.Header().Get(cacheControlHeader) != cacheControlNoStore || response.Header().Get(contentTypeOptionsHeader) != contentTypeNoSniff {
		t.Fatalf("asset = %d, %v", response.Code, response.Header())
	}
	for _, path := range []string{"/images/bad/1", strings.TrimSuffix(page.Images[0].URL, "1") + "0", page.Images[0].URL + "/extra"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s = %d", path, response.Code)
		}
	}
}

func TestImageGalleryDesktopAdmissionRejectsMalformedOptions(t *testing.T) {
	w, _ := testWindows(t)
	root := w.shown().root()
	if err := os.WriteFile(filepath.Join(root, "ok.png"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []workbench.WindowOptions{
		{Kind: workbench.KindDocument, Format: workbench.FormatImages},
		{Kind: workbench.KindTerminal, Format: workbench.FormatImages, Images: []workbench.ImageRef{{Source: "ok.png"}}, Command: []string{"true"}},
		{Kind: workbench.KindDocument, Format: workbench.FormatImages, Source: "ok.png", Images: []workbench.ImageRef{{Source: "ok.png"}}},
		{Kind: workbench.KindDocument, Format: workbench.FormatImages, Deck: true, Images: []workbench.ImageRef{{Source: "ok.png"}}},
		{Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Images: []workbench.ImageRef{{Source: "ok.png"}}},
		{Kind: workbench.KindDocument, Format: workbench.FormatImages, Images: []workbench.ImageRef{{Source: "missing.png"}}},
	}
	for i, opts := range cases {
		if _, err := w.openWindow(w.shown(), opts); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	if len(w.list()) != 0 {
		t.Fatalf("invalid opens registered %v", w.list())
	}
}
