package placement

import "errors"

var (
	ErrNoSessions   = errors.New("sessions root is unreadable")
	ErrMarkerUnsafe = errors.New("move marker is not a regular file")
)
