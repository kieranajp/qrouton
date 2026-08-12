package desktop

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"reflect"
	"strings"

	"github.com/kieranajp/qrouton/internal/theme"
)

// The tree is generated; `make front` produces it.
//
//go:embed all:assets
var assetFS embed.FS

var frontendServices = []any{Term{}, Sessions{}, Windows{}}

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

// assetHandler serves the built pages, plus the palette rendered on demand.
func assetHandler(assets fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(rootPath, http.FileServerFS(assets))
	mux.HandleFunc(theme.Path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(contentTypeHeader, theme.MediaType)
		_, _ = io.WriteString(w, theme.CSS())
	})
	return mux
}
