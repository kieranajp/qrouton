package workbench

import (
	"path/filepath"
	"strings"
)

type AssetRef struct {
	Source string `json:"source"`
}

type MediaKind string

const (
	MediaImage MediaKind = "image"
	MediaVideo MediaKind = "video"
)

type Media struct {
	Type string
	Kind MediaKind
}

var assetMedia = map[string]Media{
	".png":  {"image/png", MediaImage},
	".jpg":  {"image/jpeg", MediaImage},
	".jpeg": {"image/jpeg", MediaImage},
	".gif":  {"image/gif", MediaImage},
	".webp": {"image/webp", MediaImage},
	".avif": {"image/avif", MediaImage},
	".mp4":  {"video/mp4", MediaVideo},
	".m4v":  {"video/mp4", MediaVideo},
	".mov":  {"video/quicktime", MediaVideo},
	".webm": {"video/webm", MediaVideo},
}

func AssetMedia(name string) (Media, bool) {
	media, ok := assetMedia[strings.ToLower(filepath.Ext(name))]
	return media, ok
}
