package desktop

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/theme"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// The tree is generated; `make front` produces it.
//
//go:embed all:assets
var assetFS embed.FS

// Lifecycle services are guarded because missing bindings can strand the user
// outside a session or leave a backend-owned draft open.
var frontendServices = []any{Term{}, Sessions{}, Windows{}, FirstRun{}, Placement{}, Assembly{}, Chrome{}, BugReports{}, Ollama{}}

func validateFrontend(assets fs.FS) error {
	bundle, err := bundleText(assets)
	if err != nil {
		return err
	}
	for _, service := range frontendServices {
		typ := reflect.TypeOf(service)
		methods := reflect.PointerTo(typ)
		for i := 0; i < methods.NumMethod(); i++ {
			binding := bindingName(typ, methods.Method(i).Name)
			if !strings.Contains(bundle, binding) {
				return fmt.Errorf(staleFrontendBindingFormat, ErrStaleFrontend, binding)
			}
		}
	}
	return nil
}

// bundleText concatenates every file in the asset tree.
func bundleText(assets fs.FS) (string, error) {
	var bundle strings.Builder
	err := fs.WalkDir(assets, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		bundle.Write(body)
		return nil
	})
	return bundle.String(), err
}

// bindingName is the fully qualified name Wails binds a service method under.
func bindingName(typ reflect.Type, method string) string {
	return typ.PkgPath() + "." + typ.Name() + "." + method
}

// deckOpener opens a file beside the deck a token names; galleryOpener opens
// one asset of the gallery a token names, with its media type. Both refuse
// anything outside the window's session.
type deckOpener func(token, rel string) (*os.File, error)
type galleryOpener func(token string, index int) (*os.File, string, error)

func assetHandler(assets fs.FS, uiScale func() int, decks deckOpener, galleryOpeners ...galleryOpener) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(rootPath, http.FileServerFS(assets))
	mux.HandleFunc(theme.Path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, theme.MediaType)
		_, _ = io.WriteString(w, theme.CSS())
	})
	mux.HandleFunc(uiScalePath, func(w http.ResponseWriter, _ *http.Request) {
		percent := config.UIScaleDefault
		if uiScale != nil {
			percent = uiScale()
		}
		w.Header().Set(contentTypeHeader, theme.MediaType)
		w.Header().Set(cacheControlHeader, cacheControlNoStore)
		_, _ = fmt.Fprintf(w, uiScaleCSSFormat, strconv.FormatFloat(float64(percent)/100, 'f', -1, 64))
	})
	mux.HandleFunc(deckAssetPath, deckAsset(decks))
	var open galleryOpener
	if len(galleryOpeners) > 0 {
		open = galleryOpeners[0]
	}
	gallery := galleryAsset(open)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, galleryAssetPath) {
			gallery(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func galleryAsset(open galleryOpener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, rawIndex, split := strings.Cut(strings.TrimPrefix(r.URL.Path, galleryAssetPath), "/")
		index, err := strconv.Atoi(rawIndex)
		if !split || token == "" || index < 1 || err != nil || open == nil || strconv.Itoa(index) != rawIndex || r.URL.RawPath != "" {
			http.NotFound(w, r)
			return
		}
		file, media, err := open(token, index)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		serveMedia(w, r, file, media, map[string]string{cacheControlHeader: cacheControlNoStore, contentTypeOptionsHeader: contentTypeNoSniff})
	}
}

// deckAsset serves a deck's own pictures and video, and only those: the media
// type comes from the extension table rather than from the file, so a path that
// resolves inside the session but is not media is a 404 rather than a read.
func deckAsset(open deckOpener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, rel, split := strings.Cut(strings.TrimPrefix(r.URL.Path, deckAssetPath), "/")
		var media string
		known := false
		if m, ok := workbench.AssetMedia(rel); ok {
			media, known = m.Type, true
		} else if extra, ok := deckExtraMediaTypes[strings.ToLower(path.Ext(rel))]; ok {
			media, known = extra, true
		}
		if !split || rel == "" || !known || open == nil {
			http.NotFound(w, r)
			return
		}
		file, err := open(token, rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		serveMedia(w, r, file, media, nil)
	}
}

func serveMedia(w http.ResponseWriter, r *http.Request, file *os.File, media string, headers map[string]string) {
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set(contentTypeHeader, media)
	for name, value := range headers {
		w.Header().Set(name, value)
	}
	http.ServeContent(w, r, "", info.ModTime(), file)
}
