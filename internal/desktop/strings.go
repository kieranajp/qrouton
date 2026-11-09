package desktop

import (
	"time"

	"github.com/kieranajp/qrouton/internal/github"
)

const (
	placementLeavingNote  = "Moving a session out of a shared folder stops sharing it. Your sync tool will usually delete the folder on your teammates' machines, but versioned copies may remain."
	placementRelaunchNote = "A saved change to your folders is waiting for a relaunch, so this check uses the routing sessions are created with today."

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

	noOwnersError = "add at least one organisation or username to search"

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

	reposRefreshEvent      = "repos:refresh"
	assemblyProgressEvent  = "assembly:progress"
	assemblyRequestedEvent = "assembly:requested"
	orgsChangedEvent       = "orgs:changed"
	uiScaleEvent           = "ui:scale"
	updateEvent            = "update:status"
)

// A Save refusal names the settings field it belongs under before the sentence.
const (
	settingsFieldOrgs     = "orgs"
	settingsFieldRoot     = "root"
	settingsFieldEditor   = "editor"
	settingsFieldLaunch   = "launch"
	settingsFieldStickers = "stickers"
	settingsFieldUIScale  = "uiScale"
	settingsFieldThoughts = "thoughts"

	thoughtsOrgJoin  = ", "
	thoughtsOrgSplit = ","

	settingsRefusalFormat = "%s: %s"
	settingsWrappedFormat = "%s: %w"
	settingsEmpty         = "cannot be empty"
	stickerFieldFormat    = "%s.%d"
	stickerDuplicate      = "another sticker already uses this colour and symbol"
	stickerIDBytes        = 4
	uiScaleRefusal        = "choose a scale from 80% to 150% in steps of 10%"
	unknownUIScaleAction  = "unknown UI scale action"
)

// What a scale shortcut asks the workbench to do.
const (
	uiScaleActionIn    = "in"
	uiScaleActionOut   = "out"
	uiScaleActionReset = "reset"
)

const viewMenuLabel = "View"

const (
	// loginTimeout bounds the GitHub lookup behind the owners screen's help line,
	// which fills in after the screen has already drawn.
	loginTimeout       = 5 * time.Second
	githubCheckTimeout = 4 * time.Second
	ticketFetchTimeout = 15 * time.Second

	// branchListTimeout bounds the branch listing GitHub is asked for, which the
	// base menu opens without waiting for.
	branchListTimeout = 5 * time.Second

	// repoRefreshTimeout bounds each owner's repository listing.
	repoRefreshTimeout = 30 * time.Second

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
