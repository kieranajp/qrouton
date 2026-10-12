package placement

const (
	linkStaging = ".moving"
	moveMarker  = ".thoughtsmove"
	markerMode  = 0o644
)

const (
	detailMoveFormat     = "%s -> %s"
	detailExistsFormat   = "%s already exists"
	detailUnmarkedFormat = "%s already exists and was not moved there by qrouton"
	detailOutsideFormat  = "links outside the configured roots: %s"
	detailBrokenFormat   = "link is broken or not a directory: %s"
	detailMissingFormat  = "destination root is missing: %s"
	detailCrossFormat    = "cross-device move: %s"
	detailUnreadableFmt  = "unreadable manifest: %s"
	detailAgentFormat    = "agent running (pid %d)"
	detailNotASlug       = "not a session folder name"
	detailNotMisplaced   = "already in place"
	detailPendingFormat  = "interrupted move could not be finished: %s"
)
