package desktop

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/theme"
)

// The tree is generated; `make front` produces it.
//
//go:embed all:assets
var assetFS embed.FS

// Lifecycle services are guarded because missing bindings can strand the user
// outside a session or leave a backend-owned draft open.
var frontendServices = []any{Term{}, Sessions{}, Windows{}, FirstRun{}, Assembly{}, Chrome{}}

func validateFrontend(assets fs.FS) error {
	var bundle strings.Builder
	if err := fs.WalkDir(assets, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		bundle.Write(body)
		return nil
	}); err != nil {
		return err
	}
	for _, service := range frontendServices {
		typ := reflect.TypeOf(service)
		methods := reflect.PointerTo(typ)
		for i := 0; i < methods.NumMethod(); i++ {
			binding := typ.PkgPath() + "." + typ.Name() + "." + methods.Method(i).Name
			if !strings.Contains(bundle.String(), binding) {
				return fmt.Errorf(staleFrontendBindingFormat, ErrStaleFrontend, binding)
			}
		}
	}
	return nil
}

// deckLookup answers a deck's asset token with the session root its window
// belongs to and the directory the deck itself sits in.
type deckLookup func(token string) (root, dir string, ok bool)

func assetHandler(assets fs.FS, decks deckLookup) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(rootPath, http.FileServerFS(assets))
	mux.HandleFunc(theme.Path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, theme.MediaType)
		_, _ = io.WriteString(w, theme.CSS())
	})
	mux.HandleFunc(deckAssetPath, deckAsset(decks))
	return mux
}

// deckAsset serves a deck's own pictures and video, and only those: the media
// type comes from the extension table rather than from the file, so a path that
// resolves inside the session but is not media is a 404 rather than a read.
func deckAsset(decks deckLookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, rel, split := strings.Cut(strings.TrimPrefix(r.URL.Path, deckAssetPath), "/")
		media, known := deckMediaTypes[strings.ToLower(path.Ext(rel))]
		if !split || rel == "" || !known || decks == nil {
			http.NotFound(w, r)
			return
		}
		root, dir, ok := decks(token)
		if !ok {
			http.NotFound(w, r)
			return
		}
		name, err := launch.ResolveSessionFile(root, filepath.FromSlash(path.Join(dir, rel)))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		file, err := os.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set(contentTypeHeader, media)
		http.ServeContent(w, r, "", info.ModTime(), file)
	}
}
