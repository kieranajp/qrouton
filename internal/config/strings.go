package config

// Paths, environment overrides, and first-run wizard copy.

const (
	// appDirName is the directory qrouton owns inside each XDG base directory.
	appDirName = "qrouton"

	configFileName = "config.json"
	cacheFileName  = "repos.json"

	// helpScriptFileName is the quick-reference panel Alt-? and the startup
	// floating pane run. One copy under the config dir, not one per session —
	// restaged idempotently at every launch so template changes propagate.
	helpScriptFileName = "help.sh"

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

// First-run wizard copy and defaults.
const (
	wizardRootTitle       = "Root directory"
	wizardRootDescription = "Sessions live flat under it; repo mirrors under <root>/.mirrors"
	wizardRootDefault     = "~/work"

	wizardOrgsTitle       = "GitHub orgs"
	wizardOrgsDescription = "Comma-separated organizations whose repos the session picker lists"
	wizardOrgsDefault     = "lifesum"
)
