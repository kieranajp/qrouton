package update

import "time"

const (
	// Repository is where releases are published, and the only source an
	// install will take a new bundle from.
	Repository = "kieranajp/qrouton"

	// checksumAsset is published beside the archive by the release workflow.
	// The updater refuses an artifact whose streamed hash misses it.
	checksumAsset = "checksums.txt"

	// floorAsset carries the oldest version the release will talk to. A
	// release without one imposes no floor.
	floorAsset = "minimum-version.txt"

	// archiveSuffix is what the release workflow names the macOS bundle. One
	// universal archive serves both architectures, so the framework's own
	// matcher — which wants a GOARCH in the filename — never matches it.
	archivePrefix = "qrouton-"
	archiveSuffix = "-macos-universal.zip"

	apiBase        = "https://api.github.com"
	latestPath     = "/repos/%s/releases/latest"
	acceptHeader   = "Accept"
	apiMediaType   = "application/vnd.github+json"
	assetMediaType = "application/octet-stream"

	// darwinPlatform is the only platform releases are cut for. The floor and
	// the self-update are both silent elsewhere rather than wrong.
	darwinPlatform = "darwin"

	// appBundleSuffix ends the directory the framework's helper replaces.
	appBundleSuffix = ".app"

	// checkTimeout bounds the launch check. An install that cannot reach
	// GitHub opens on what it has rather than waiting on a network.
	checkTimeout = 10 * time.Second

	// checkInterval re-checks a workbench left open for days, so a long-lived
	// window is not the way to stay behind. settleInterval is how often a
	// staged release asks whether the workbench has gone quiet.
	checkInterval  = 6 * time.Hour
	settleInterval = 30 * time.Second

	// floorLimit bounds the floor asset read: the file is one short version
	// string, and anything larger is not the file we asked for. releaseLimit
	// bounds the feed itself, whose asset list grows with every architecture.
	floorLimit   = 64
	releaseLimit = 1 << 20
)
