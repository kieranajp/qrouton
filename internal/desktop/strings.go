package desktop

import (
	"time"

	"github.com/kieranajp/qrouton/internal/github"
)

const (
	applicationName        = "qrouton"
	applicationDescription = "qrouton workbench"

	mainWindowName   = "conversation"
	mainWindowTitle  = "qrouton"
	titleSeparator   = " — "
	mainWindowWidth  = 1100
	mainWindowHeight = 760

	notesWindowName   = "presenter-notes"
	notesWindowTitle  = "Speaker notes"
	notesWindowWidth  = 520
	notesWindowHeight = 720

	sessionUnloadLabel   = "Unload"
	sessionUnloadFailed  = "Session could not be unloaded"
	sessionReorderFailed = "Session could not be moved"

	// The page URL is a directory: http.FileServer 301-redirects
	// /index.html to /, and the webview does not follow the redirect.
	frontendRoot = "/"

	inspectQuery = "inspect"

	// A directory too, for the redirect frontendRoot dodges.
	notesRoot = "/notes/"

	assetRoot = "assets"

	staleFrontendError         = "workbench frontend is stale; run make front before building qrouton"
	staleFrontendBindingFormat = "%w: built pages do not call %s"

	// frontendSource is where the pages are written, not where they are built.
	frontendSource = "frontend/src/"

	rootPath          = "/"
	contentTypeHeader = "Content-Type"

	// deckAssetPath leads a deck's own media, addressed by the per-window token
	// that follows it. Window ids are sequential and never appear in a URL.
	deckAssetPath = "/deck/"

	// Served live so a new window paints at the stored scale on its first frame.
	uiScalePath      = "/tokens/scale.css"
	uiScaleCSSFormat = ":root { --ui-scale: %s; }\n"
)

// Events the Go side emits at the pages; a window's or a conversation's own id
// is appended so each page hears only its own stream.
const (
	presenterNotesEvent  = "presenter:notes"
	presenterClosedEvent = "presenter:closed"

	updateEvent = "update:status"
)

const viewMenuLabel = "View"

const (
	// updateInterval paces the release check; updateCheckTimeout bounds each one.
	updateInterval     = 6 * time.Hour
	updateCheckTimeout = 10 * time.Second
)

const (
	updateEndpoint        = github.APIBaseDefault + "/repos/" + github.IssueRepository + "/releases/latest"
	updateFallbackURL     = github.WebBase + "/" + github.IssueRepository + "/releases/latest"
	updateBrewCommand     = "brew upgrade --cask qrouton"
	updateUserAgentHeader = "User-Agent"
	updateUserAgentPrefix = "qrouton/"
)

var deckExtraMediaTypes = map[string]string{
	".svg":  "image/svg+xml",
	".webm": "video/webm",
}

const (
	galleryLabel             = "GALLERY"
	galleryAssetPath         = "/gallery/"
	cacheControlHeader       = "Cache-Control"
	cacheControlNoStore      = "no-store"
	contentTypeOptionsHeader = "X-Content-Type-Options"
	contentTypeNoSniff       = "nosniff"
)

const (
	switcherLabel     = "Switch session"
	switcherSearch    = "Search sessions"
	switcherClose     = "Close session picker"
	switcherEmpty     = "No sessions available"
	switcherNoMatches = "No matching sessions"
	switcherOpening   = "Opening…"
	switcherFailed    = "Session could not be opened. Try again."
	switcherCurrent   = "Current"
	switcherResults   = "sessions"
	switcherHelp      = "Arrow keys choose · Enter open · Esc close"
)
