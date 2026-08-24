package share

import "errors"

var (
	// ErrNoDocument is returned for an empty document: a page with nothing on it
	// is not worth a colleague's click.
	ErrNoDocument = errors.New("document is empty")

	// ErrNoBundle means `make front` has not run, so the page would carry no
	// renderer and no styles.
	ErrNoBundle = errors.New("share bundle is missing")
)
