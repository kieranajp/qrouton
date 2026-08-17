package config

// Paths, environment overrides, and the defaults a launch never asks about.

const (
	// appDirName is the directory qrouton owns inside each XDG base directory.
	appDirName = "qrouton"

	configFileName = "config.json"
	cacheFileName  = "repos.json"

	// XDG bases, with the fallbacks the design pinned. Not os.UserConfigDir: on
	// darwin that is ~/Library/Application Support, and qrouton follows XDG on
	// every platform so one config path documents them all.
	configHomeEnvVar   = "XDG_CONFIG_HOME"
	cacheHomeEnvVar    = "XDG_CACHE_HOME"
	configHomeFallback = ".config"
	cacheHomeFallback  = ".cache"

	// Runtime overrides, for scripting and for tests.
	rootEnvVar = "QROUTON_ROOT"
	orgsEnvVar = "QROUTON_ORGS"

	orgSeparator = ","

	homePrefix = "~"
	homeSlash  = homePrefix + "/"

	dirMode  = 0o755
	fileMode = 0o644
)

const (
	// defaultRoot is where sessions live when config.json names no root, so a
	// launch never has to stop and ask. Mirrors go under <root>/.mirrors.
	defaultRoot = "~/work"
)
