package session

import (
	"errors"
	"fmt"
)

// ErrInvalidRole means a caller asked for a repository role that is neither
// active nor reference.
var ErrInvalidRole = errors.New("invalid repository role")

// ErrNoPinnedRevision means a reference repository cannot be re-materialised,
// because the manifest does not say which commit it was pinned to.
var ErrNoPinnedRevision = errors.New("reference repository has no pinned revision")

// ErrNoCloneURL means a manifest cannot be resumed: Create always records a
// clone URL, so one without it was hand-edited or truncated.
var ErrNoCloneURL = errors.New("repository records no clone URL")

func invalidRole(role RepoRole, org, name string) error {
	return fmt.Errorf("%w %q for %s/%s", ErrInvalidRole, role, org, name)
}
