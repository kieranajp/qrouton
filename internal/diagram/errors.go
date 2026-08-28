package diagram

import "errors"

// Sentinel errors a failed diagram carries. Everything except ErrNoRuler is a
// refusal rather than a fault: the source compiled, and the output was not
// something qrouton will put in a webview that has no content policy.
var (
	ErrEmbeddedScript = errors.New(embeddedScriptError)
	// A foreignObject is how a |md| block reaches the page: arbitrary HTML.
	ErrEmbeddedMarkup = errors.New(embeddedMarkupError)
	ErrRemoteImage    = errors.New(remoteImageError)
	// The webview's context menu hands href to the OS without filtering it.
	ErrUnsafeLink       = errors.New(unsafeLinkError)
	ErrMalformedSVG     = errors.New(malformedError)
	ErrUnknownConstruct = errors.New(unknownConstructError)
	ErrTimedOut         = errors.New(timedOutError)
	ErrNoRuler          = errors.New(noRulerError)
)
