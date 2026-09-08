package desktop

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
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

func openTestGallery(t *testing.T, w *Windows, owner *sessionState) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(owner.root(), "image.png"), []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := w.openWindow(owner, workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatImages, Images: []workbench.ImageRef{{Source: "image.png"}, {Source: "image.png"}, {Source: "image.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestImageFocusCommitsOwnerKindBoundsAndRevisionTogether(t *testing.T) {
	w, _ := testWindows(t)
	owner := w.shown()
	other := w.sessions.add(t.TempDir(), nil, nil)
	id := openTestGallery(t, w, owner)
	elsewhere := openTestGallery(t, w, other)
	plain, err := w.openWindow(owner, workbench.WindowOptions{Kind: workbench.KindDocument, Select: true})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := w.Content(id)
	for _, request := range []struct {
		slug, id string
		index    int
	}{
		{owner.slug(), id, 0}, {owner.slug(), id, -1}, {owner.slug(), id, 4},
		{owner.slug(), plain, 1}, {owner.slug(), "missing", 1}, {other.slug(), id, 1}, {owner.slug(), elsewhere, 1},
	} {
		if _, err := w.FocusImage(request.slug, request.id, request.index); err == nil {
			t.Fatalf("accepted %+v", request)
		}
		after, _ := w.Content(id)
		if !reflect.DeepEqual(before, after) || w.Surfaces(owner.slug()).Selected != plain {
			t.Fatal("failed focus changed gallery or tab selection")
		}
	}
	for n, index := range []int{3, 3, 1} {
		selected, err := w.FocusImage(owner.slug(), id, index)
		if err != nil || selected.CurrentIndex != index || selected.Revision != uint64(n+2) {
			t.Fatalf("selection = %+v, %v", selected, err)
		}
		after, _ := w.Content(id)
		if !reflect.DeepEqual(after.Images, before.Images) || w.Surfaces(owner.slug()).Selected != id {
			t.Fatal("focus changed identity/order/token or missed tab")
		}
	}
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, w, other, controlHooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	host := (workbench.Handle{Socket: socket, SessionRoot: owner.root()}).WindowHost()
	if _, err := host.FocusImage(context.Background(), id, 2); err == nil {
		t.Fatal("socket borrowed the supplied root's ownership")
	}
	current, _ := w.Content(id)
	if current.Revision != 4 || current.CurrentIndex != 1 {
		t.Fatal("cross-session request mutated gallery")
	}
}

func TestConcurrentImageFocusFollowsDesktopCommitOrder(t *testing.T) {
	w, _ := testWindows(t)
	owner := w.shown()
	id := openTestGallery(t, w, owner)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, w, owner, controlHooks{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	host := (workbench.Handle{Socket: socket, SessionRoot: owner.root()}).WindowHost()
	results := make(chan workbench.ImageSelection, 30)
	failures := make(chan error, 30)
	var group sync.WaitGroup
	for i := 0; i < 30; i++ {
		group.Go(func() {
			var result workbench.ImageSelection
			var err error
			if i%2 == 0 {
				result, err = w.FocusImage(owner.slug(), id, i%3+1)
			} else {
				result, err = host.FocusImage(context.Background(), id, i%3+1)
			}
			if err != nil {
				failures <- err
				return
			}
			results <- result
		})
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	revisions := map[uint64]bool{}
	var latest workbench.ImageSelection
	for result := range results {
		if revisions[result.Revision] {
			t.Fatal("duplicate commit revision")
		}
		revisions[result.Revision] = true
		if result.Revision > latest.Revision {
			latest = result
		}
	}
	doc, _ := w.Content(id)
	if len(revisions) != 30 || doc.Revision != 31 || doc.CurrentIndex != latest.CurrentIndex {
		t.Fatal(fmt.Sprintf("final state %+v disagrees with commits %+v", doc, latest))
	}
}

func TestImageTokensOnlyReachLiveAdmittedEntries(t *testing.T) {
	w, _ := testWindows(t)
	owner := w.shown()
	other := w.sessions.add(t.TempDir(), nil, nil)
	first := openTestGallery(t, w, owner)
	second := openTestGallery(t, w, other)
	deck, err := w.openWindow(owner, workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatMarkdown, Deck: true, Source: "deck.md"})
	if err != nil {
		t.Fatal(err)
	}
	deckWindow, _ := w.window(deck)
	page, _ := w.Content(first)
	otherPage, _ := w.Content(second)
	handler := assetHandler(fstest.MapFS{}, w.deckDirectory, w.imageAsset)
	status := func(url string) int {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, url, nil))
		return response.Code
	}
	prefix := strings.TrimSuffix(page.Images[0].URL, "1")
	for _, url := range []string{prefix + "4", prefix + "-1", prefix + "+1", prefix + "01", prefix + "1/neighbor.png", prefix + "../1", prefix + "%2e%2e/1", prefix + "1%2f..%2f2", prefix + "1.png", "/images/" + deckWindow.asset + "/1"} {
		if got := status(url); got != http.StatusNotFound {
			t.Errorf("%s = %d", url, got)
		}
	}
	if err := os.WriteFile(filepath.Join(owner.root(), "neighbor.png"), []byte("neighbor"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := status(prefix + "neighbor.png"); got != 404 {
		t.Fatal("unlisted neighbor available")
	}
	if err := w.Close(first); err != nil {
		t.Fatal(err)
	}
	if status(page.Images[0].URL) != 404 || status(otherPage.Images[0].URL) != 200 {
		t.Fatal("closing one gallery changed another token")
	}
	if _, err := w.FocusImage(owner.slug(), first, 1); err == nil {
		t.Fatal("focused closed gallery")
	}
	w.sessions.boot.teardown = w.stop
	w.sessions.retire(other)
	if status(otherPage.Images[0].URL) != 404 {
		t.Fatal("retired gallery token is live")
	}
	if _, err := w.focusImage(other, second, 1); err == nil {
		t.Fatal("focused retired gallery")
	}
}

func TestImageAssetRevalidatesChangedFilesAndRefreshesUnchangedMetadata(t *testing.T) {
	w, _ := testWindows(t)
	owner := w.shown()
	root := owner.root()
	home := t.TempDir()
	if err := os.Symlink(home, filepath.Join(root, "thoughts")); err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(home, "capture.png")
	if err := os.WriteFile(name, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	options := workbench.WindowOptions{Kind: workbench.KindDocument, Format: workbench.FormatImages, Images: []workbench.ImageRef{{Source: "thoughts/capture.png"}, {Source: "thoughts/capture.png"}}}
	old, err := w.openWindow(owner, options)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := w.Content(old)
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	handler := assetHandler(fstest.MapFS{}, nil, w.imageAsset)
	read := func(url string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, url, nil))
		return response
	}
	if got := read(before.Images[0].URL); got.Body.String() != "before" {
		t.Fatal("initial bytes missing")
	}
	if _, err := w.FocusImage(owner.slug(), old, 2); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("after!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(name, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(old); err != nil {
		t.Fatal(err)
	}
	fresh, err := w.openWindow(owner, options)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := w.Content(fresh)
	if old == fresh || before.Images[0].URL == after.Images[0].URL || after.CurrentIndex != 1 || after.Revision != 1 {
		t.Fatal("reopen retained old lifetime")
	}
	response := read(after.Images[0].URL)
	if response.Code != 200 || response.Body.String() != "after!" || response.Header().Get(cacheControlHeader) != cacheControlNoStore || response.Header().Get(contentTypeOptionsHeader) != contentTypeNoSniff {
		t.Fatalf("fresh bytes = %+v %q", response, response.Body.String())
	}
	if read(before.Images[0].URL).Code != 404 {
		t.Fatal("old URL still available")
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if read(after.Images[0].URL).Code != 404 {
		t.Fatal("deleted file still available")
	}
	if err := os.Mkdir(name, 0o755); err != nil {
		t.Fatal(err)
	}
	if read(after.Images[0].URL).Code != 404 {
		t.Fatal("directory still available")
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(external, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, name); err != nil {
		t.Fatal(err)
	}
	if read(after.Images[0].URL).Code != 404 {
		t.Fatal("external symlink acquired authority")
	}
	w.stopAll()
	if read(after.Images[0].URL).Code != 404 {
		t.Fatal("shutdown token still available")
	}
}
