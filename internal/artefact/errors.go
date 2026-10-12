package artefact

import "errors"

var (
	ErrUnknownKind = errors.New("unknown artefact kind")
	ErrNumberTaken = errors.New("could not find a free artefact number")
	ErrBadTemplate = errors.New("artefact template has no frontmatter")
)
