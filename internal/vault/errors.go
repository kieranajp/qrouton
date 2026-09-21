package vault

import "errors"

var (
	ErrImportState         = errors.New("durable import state unavailable")
	ErrImportSource        = errors.New("invalid import source selection")
	ErrImportStale         = errors.New("import sources changed; regenerate preview")
	ErrImportRepair        = errors.New("import requires repair")
	ErrImportLink          = errors.New("unresolved local link; repair the body or link mapping")
	ErrImportRepository    = errors.New("repository alias or historical pin requires repair")
	ErrImportWrite         = errors.New("canonical import write unavailable")
	ErrCrossVaultImport    = errors.New("cross-vault promotion is not supported")
	ErrPublicationDisabled = errors.New("source session publication is disabled")
	ErrDestinationChanged  = errors.New("destination root changed; explicit revalidation required")

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

var ErrImportPending = errors.New("import dependencies are not ready")
