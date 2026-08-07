package repos

import "time"

// The repo watcher's copy: its heading, the states a worktree can be in, and
// the line formats that lay them out.

const (
	heading         = "repos"
	refreshInterval = 3 * time.Second

	noManifestLabel = "No session manifest"

	// The zero-repo empty state a scratch session shows until escalation.
	emptyStateLabel = "no repositories yet"
	emptyStateHint  = "Alt-e to escalate"

	stateClean       = "clean"
	stateMissing     = "missing — resume to restore"
	stateUnavailable = "unavailable"

	changedFormat   = "%d changed"
	referencePrefix = "reference · "

	// detachedPrefix marks a pinned reference checkout, whose HEAD is a revision
	// rather than a branch.
	detachedPrefix = "@ "

	repoLineFormat  = "%s  %s · %s"
	repoStateFormat = "%s  %s"
)
