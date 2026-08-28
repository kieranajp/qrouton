package diagram

// A fence qrouton will render: at least three backticks or tildes, indented
// under four spaces, whose info string starts with exactly this word.
const (
	fenceInfo      = "d2"
	fenceBacktick  = '`'
	fenceTilde     = '~'
	minFenceLength = 3
	maxFenceIndent = 4
)

// URL schemes the guard reads out of the rendered SVG.
const (
	dataScheme  = "data:"
	httpScheme  = "http:"
	httpsScheme = "https:"
)

// Why a diagram was refused. Each names the construct rather than the rule, so
// the line the page prints beside the code says what to change.
const (
	embeddedScriptError = "diagram contains a script or an event handler"
	embeddedMarkupError = "diagram embeds HTML; |md| blocks are not rendered"
	remoteImageError    = "diagram fetches a remote image; icons must be inline"
	unsafeLinkError     = "diagram links to something other than http or https"
	timedOutError       = "diagram took too long to lay out"
	noRulerError        = "text measurement is unavailable"
)
