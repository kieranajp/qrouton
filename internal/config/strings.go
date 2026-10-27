package config

const (
	// appDirName is the directory qrouton owns inside each XDG base directory.
	appDirName = "qrouton"

	configFileName   = "config.json"
	cacheFileName    = "repos.json"
	webglLogFileName = "webgl.jsonl"

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
	envAssign    = "="

	homePrefix = "~"
	homeSlash  = homePrefix + "/"

	dirMode  = 0o755
	fileMode = 0o644
)

const (
	// defaultRoot is where sessions live when config.json names no root, so a
	// launch never has to stop and ask. Mirrors go under <root>/.mirrors.
	defaultRoot = "~/work"

	DefaultThoughtsID = "default"

	UIScaleMin     = 80
	UIScaleMax     = 150
	UIScaleStep    = 10
	UIScaleDefault = 100
)

// Session manifests written before stickers were user-defined carry these ids,
// so the derived list keeps them.
const (
	LegacyStickerStar        = "star"
	LegacyStickerBookmark    = "bookmark"
	LegacyStickerQuestion    = "question"
	LegacyStickerExclamation = "exclamation"
)

var stickerSymbols = []string{"star", "bookmark", "question", "exclamation", "dot", "square", "diamond", "triangle", "heart", "bolt", "flag", "check", "cross", "pause"}

const (
	fingerprintFieldJoin = "\x00"
	fingerprintOrgJoin   = ","
	fingerprintEntryJoin = "\n"
	fingerprintBytes     = 8
)
