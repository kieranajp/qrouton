package vault

const (
	SchemaVersion     = 1
	StateDisabled     = "disabled"
	StateInitializing = "initializing"
	StateIncomplete   = "incomplete"
	StateUnavailable  = "unavailable"
	ModeUncalibrated  = "uncalibrated"
	maxDocumentBytes  = 8 << 20
)
