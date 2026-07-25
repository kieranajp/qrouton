package config

import "errors"

// Configuration problems the user has to fix by hand, so each names the field
// at fault.
var (
	// ErrNoOrgs means the repo picker would have nothing to list.
	ErrNoOrgs = errors.New("orgs must contain at least one GitHub organization")

	// ErrNoRoot means qrouton does not know where to assemble sessions.
	ErrNoRoot = errors.New("root must be set (or export " + rootEnvVar + ")")

	// errRootRequired fails wizard validation before an empty root can be
	// written to disk, where it would break Load on every subsequent start.
	errRootRequired = errors.New("root directory is required")

	// errOrgsRequired fails wizard validation for the same reason.
	errOrgsRequired = errors.New("need at least one organization")
)
