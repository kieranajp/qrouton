package desktop

import (
	"embed"
	"io"
	"io/fs"
	"net/http"

	"github.com/kieranajp/qrouton/internal/theme"
)

// The tree is generated; `make front` produces it.
//
//go:embed all:assets
var assetFS embed.FS

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
