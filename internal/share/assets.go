package share

import "embed"

// The bundle is generated; `make front` produces it from
// internal/desktop/frontend.
//
//go:embed all:assets
var assetFS embed.FS
