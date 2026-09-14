package session

import (
	"errors"
	"fmt"
)

var ErrInvalidRole = errors.New("invalid repository role")

var ErrNoPinnedRevision = errors.New("reference repository has no pinned revision")

// ErrNoCloneURL means a manifest cannot be resumed: Create always records a
// clone URL, so one without it was hand-edited or truncated.
var ErrNoCloneURL = errors.New("repository records no clone URL")

var ErrNotHeld = errors.New("session does not hold that repository")

// ErrNotReference means only a reference checkout can be taken up for editing.
// An editing worktree already carries the session's own work.
var ErrNotReference = errors.New("repository is not checked out as reference")

var ErrCheckoutHasWork = errors.New("uncommitted work in the checkout would be overwritten; commit or stash it first")

var ErrReferenceMoved = errors.New("reference checkout has moved off its pinned revision")

// ErrNoBaseBranch means there is nothing to cut a branch from: no base was
// chosen and GitHub reported no default branch, which is what a repository
// without commits looks like.
var ErrNoBaseBranch = errors.New("repository has no branch to work from")

func invalidRole(role RepoRole, org, name string) error {
	return fmt.Errorf("%w %q for %s/%s", ErrInvalidRole, role, org, name)
}

func refuseRepo(err error, org, name string) error {
	return fmt.Errorf("%s/%s: %w", org, name, err)
}

func mismatchedManifest(dir, slug string) error {
	return fmt.Errorf("session directory %q holds the manifest of %q, so nothing was removed", dir, slug)
}
