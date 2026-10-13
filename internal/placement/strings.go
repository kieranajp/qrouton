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
	detailOutsideFormat  = "Links outside the configured roots: %s"
	detailBrokenFormat   = "Link is broken or not a directory: %s"
	detailMissingFormat  = "Destination root is missing: %s"
	detailCrossFormat    = "Cross-device move: %s"
	detailUnreadableFmt  = "Unreadable manifest: %s"
	detailAgentFormat    = "Agent running (pid %d)"
	detailMarkerFormat   = "%s is not a regular file"
	detailNotASlug       = "Not a session folder name"
	detailNotMisplaced   = "Already in place"
	detailPendingFormat  = "Interrupted move could not be finished: %s"
)
