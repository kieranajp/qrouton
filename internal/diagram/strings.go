package diagram

// A fence qrouton will render: at least three backticks or tildes, indented
// under four columns, whose info string starts with exactly this word.
const (
	fenceInfo      = "d2"
	fenceBacktick  = '`'
	fenceTilde     = '~'
	minFenceLength = 3
	maxFenceIndent = 4
	// CommonMark measures indent in columns; a tab advances to the next multiple.
	tabStop = 4
)

// Names and schemes the guard reads out of the rendered SVG.
const (
	dataScheme     = "data:"
	httpScheme     = "http:"
	httpsScheme    = "https:"
	fragmentPrefix = "#"

	anchorElement        = "a"
	imageElement         = "image"
	useElement           = "use"
	scriptElement        = "script"
	foreignObjectElement = "foreignobject"
	directive            = "a document type declaration"

	hrefAttribute = "href"
	eventPrefix   = "on"
	dataPrefix    = "data-"
	xmlnsSpace    = "xmlns"
)

// Why a diagram was refused. Each names the construct rather than the rule, so
// the line the page prints beside the code says what to change.
const (
	embeddedScriptError   = "diagram contains a script or an event handler"
	embeddedMarkupError   = "diagram embeds HTML; |md| blocks are not rendered"
	remoteImageError      = "diagram fetches a remote image; icons must be inline"
	unsafeLinkError       = "diagram links to something other than http or https"
	malformedError        = "diagram output is not well-formed XML"
	unknownConstructError = "diagram contains markup qrouton does not render"
	unknownElementError   = "diagram contains an element qrouton does not render: "
	unknownAttributeError = "diagram carries an attribute qrouton does not allow: "
	timedOutError         = "diagram took too long to lay out"
	noRulerError          = "text measurement is unavailable"
)
