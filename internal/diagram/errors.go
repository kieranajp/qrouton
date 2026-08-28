package diagram

import "errors"

// Sentinel errors a failed diagram carries. Everything except ErrNoRuler is a
// refusal rather than a fault: the source compiled, and the output was not
// something qrouton will put in a webview that has no content policy.
var (
	// ErrEmbeddedScript means the SVG carries a script element or an inline
	// event handler. d2 has never been seen to emit either.
	ErrEmbeddedScript = errors.New(embeddedScriptError)

	// ErrEmbeddedMarkup means the SVG carries a foreignObject, which is how a
	// |md| block reaches the page — arbitrary HTML through d2's goldmark path.
	ErrEmbeddedMarkup = errors.New(embeddedMarkupError)

	// ErrRemoteImage means an icon points somewhere other than a data URI, so
	// displaying it would make the workbench fetch over the network.
	ErrRemoteImage = errors.New(remoteImageError)

	// ErrUnsafeLink means a link uses a scheme other than http or https. The
	// webview's context menu hands href to the OS without filtering it.
	ErrUnsafeLink = errors.New(unsafeLinkError)

	// ErrTimedOut means layout did not finish inside the renderer's budget.
	ErrTimedOut = errors.New(timedOutError)

	// ErrNoRuler means the font ruler could not be built, so nothing can be
	// measured and no diagram in this process will ever render.
	ErrNoRuler = errors.New(noRulerError)
)
