package workbench

import (
	"path/filepath"
	"strings"
)

type ImageRef struct {
	Source string `json:"source"`
}

var rasterMediaTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".webp": "image/webp", ".avif": "image/avif",
}

func RasterMediaType(name string) (string, bool) {
	media, ok := rasterMediaTypes[strings.ToLower(filepath.Ext(name))]
	return media, ok
}
