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

	// dismissScriptFileName is the shared Esc wait every floated pane ends
	// with. It sits beside help.sh, which finds it by $0's directory rather
	// than having a path threaded through Go and the staged keybindings.
	dismissScriptFileName = "dismiss.sh"

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

// Owner-prompt copy and defaults.
const (
	// defaultRoot is where sessions live when config.json names no root, so a
	// launch never has to stop and ask. Mirrors go under <root>/.mirrors.
	defaultRoot = "~/work"

	wizardOrgsTitle       = "GitHub orgs"
	wizardOrgsDescription = "Comma-separated organizations whose repos the session picker lists"
	wizardOrgsDefault     = "lifesum"
)
