package session

import (
	"errors"
	"fmt"
)

// ErrInvalidRole means a caller asked for a repository role that is neither
// editing nor reference.
var ErrInvalidRole = errors.New("invalid repository role")

// ErrNoPinnedRevision means a reference repository cannot be re-materialised,
// because the manifest does not say which commit it was pinned to.
var ErrNoPinnedRevision = errors.New("reference repository has no pinned revision")

// ErrNoCloneURL means a manifest cannot be resumed: Create always records a
// clone URL, so one without it was hand-edited or truncated.
var ErrNoCloneURL = errors.New("repository records no clone URL")

// ErrNotHeld means a session was asked to re-role a repository it does not hold.
var ErrNotHeld = errors.New("session does not hold that repository")

// ErrNotReference means only a reference checkout can be taken up for editing.
// An editing worktree already carries the session's own work.
var ErrNotReference = errors.New("repository is not checked out as reference")

// ErrCheckoutHasWork means git would not move a checkout onto the session branch
// without overwriting what is in it.
var ErrCheckoutHasWork = errors.New("uncommitted work in the checkout would be overwritten; commit or stash it first")

// ErrReferenceMoved means a reference checkout no longer sits at the revision the
// manifest pinned it to.
var ErrReferenceMoved = errors.New("reference checkout has moved off its pinned revision")

func invalidRole(role RepoRole, org, name string) error {
	return fmt.Errorf("%w %q for %s/%s", ErrInvalidRole, role, org, name)
}

func refuseUpgrade(err error, org, name string) error {
	return fmt.Errorf("%s/%s: %w", org, name, err)
}
