package vault

import "errors"

var (
	ErrIdentityState = errors.New("vault identity state unavailable")
	ErrIndexState    = errors.New("vault index unavailable")
	ErrIndexCorrupt  = errors.New("vault index generation invalid")
	ErrSourceChanged = errors.New("vault sources changed during indexing")

	ErrProviderUnavailable = errors.New("embedding service unavailable")
	ErrModelMissing        = errors.New("exact embedding model missing")
	ErrInvalidVector       = errors.New("incompatible embedding output")
	ErrFingerprintChanged  = errors.New("embedding fingerprint changed")
	ErrInputBudget         = errors.New("embedding input exceeds token budget")
	ErrModelPull           = errors.New("embedding model download failed")
	ErrProviderResponse    = errors.New("invalid embedding service response")
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
