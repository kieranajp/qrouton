package update

import "errors"

// ErrNoRelease means the feed named no release to compare against. Not a
// failure: a repository with nothing published leaves an install up to date.
var ErrNoRelease = errors.New("no published release to update to")
