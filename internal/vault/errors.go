package vault

import "errors"

var (
	ErrInvalidDocument     = errors.New("invalid vault document")
	ErrUnsupportedSchema   = errors.New("unsupported vault schema")
	ErrInvalidConfig       = errors.New("invalid vault configuration")
	ErrScope               = errors.New("vault profile outside session scope")
	ErrDestinationRequired = errors.New("explicit vault destination required")
	ErrNotFound            = errors.New("vault artifact not found")
	ErrConflict            = errors.New("conflicting vault identity")
	ErrAmbiguous           = errors.New("vault origin required")
	ErrUnavailable         = errors.New("vault unavailable")
)
