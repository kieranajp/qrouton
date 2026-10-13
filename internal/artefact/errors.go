package artefact

import "errors"

var (
	ErrUnknownKind     = errors.New("unknown artefact kind")
	ErrNumberTaken     = errors.New("could not find a free artefact number")
	ErrOutsideThoughts = errors.New("artefact folder resolves outside the folder the session thoughts link points at")
	ErrNoThoughtsLink  = errors.New("session has no thoughts link")
	ErrBadTemplate     = errors.New("artefact template has no frontmatter")
)
