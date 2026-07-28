package config

import "errors"

// Configuration problems the user has to fix by hand, so each names the field
// at fault. Missing values are not among them: the root defaults and the owners
// are prompted for on demand, so Load has nothing left to reject.
var (
	// errOrgsRequired fails the owner prompt's validation before an empty
	// value can be written to disk, where it would prompt again every launch.
	errOrgsRequired = errors.New("need at least one organization")
)
