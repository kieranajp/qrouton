package desktop

import "embed"

// The frontend ships as static files with no build step.
//
//go:embed all:assets
var assetFS embed.FS
