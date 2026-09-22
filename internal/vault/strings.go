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

const (
	ollamaEndpoint = "http://127.0.0.1:11434"
	embeddingProbe = "A local document about software research."
)

const (
	EmbeddingModel       = "all-minilm:22m-l6-v2-fp16"
	EmbeddingDimensions  = 384
	EmbeddingMaxTokens   = 256
	PreprocessingVersion = "minilm-wordpiece-v1"
	ChunkerVersion       = "headings-v1"
)

const (
	StateReady             = "ready"
	IndexWaiting           = "waiting"
	IndexBuilding          = "indexing"
	DependencyReady        = "ready"
	DependencyUnavailable  = "service_unavailable"
	DependencyModelMissing = "model_missing"
	DependencyIncompatible = "incompatible"
	identityFilename       = "identities.json"
	workerLockFilename     = "writer.lock"
	currentFilename        = "current"
	generationManifest     = "manifest.json"
	generationVectors      = "vectors.bin"
	indexSchemaVersion     = 1
	maxIndexBytes          = 256 << 20
)

const (
	maxImportRecordBytes     = 128 << 20
	importDirectoryName      = "imports"
	importUnsupportedField   = "unsupported field retained in the import archive"
	importRequiredField      = "required metadata is missing"
	queuePrefix              = "queue-"
	policyPrefix             = "policy-"
	conflictPrefix           = "conflict-"
	evidencePrefix           = "evidence-"
	publicationLockFilename  = ".qrouton-publication.lock"
	ImportPending            = "pending"
	ImportWritten            = "written"
	ImportPaused             = "paused"
	ImportConflict           = "conflict"
	ImportUnavailable        = "unavailable"
	ImportDestinationChanged = "destination_changed"
)

const (
	importIdentityRequired  = "use an explicit session/local-id identity"
	importNamespaceRequired = "session namespace must match the artifact identity"
	importDateRequired      = "use the recorded date in YYYY-MM-DD format"
	ImportRepairRequired    = "repair_required"
)

const (
	corpusOtherVault           = "repository scope belongs to another vault"
	corpusOtherOrganisation    = "repository organisations are outside the selected vault"
	corpusSessionMismatch      = "declared session disagrees with corpus folder"
	corpusHeadingEvidence      = "first level-one heading"
	corpusAuthorDefault        = "reviewed corpus author"
	corpusStateDefault         = "reviewed corpus state"
	corpusWorkstreamDefault    = "reviewed corpus workstream"
	corpusSessionWorkstream    = "session namespace"
	corpusMetadataRepair       = "reviewed document repair"
	corpusBodyRepair           = "reviewed body repair"
	corpusNamespaceRepair      = "namespace mapping"
	corpusUnknownHistory       = "legacy historical revision unknown; staleness is unknown"
	corpusUnresolvedRepository = "repository declaration is unresolved: "
	corpusAliasConflict        = "repository aliases disagree"
	corpusInvalidRevision      = "recorded revision is not a commit identifier"
	corpusManifestRevision     = "recorded repository revision"
	corpusReferenceDefault     = "legacy role not recorded; reference default"
	corpusReferenceWarning     = "legacy role not recorded; reference default for "
	corpusRevisionConflict     = "recorded repository revisions disagree"
	corpusFullCommit           = "recorded full commit"
	corpusExpandedCommit       = "expanded recorded commit "
	corpusUnresolvedCommit     = "recorded abbreviated commit could not be resolved: "
	corpusNoRevision           = "no recoverable historical revision"
	corpusRepositoryConflict   = "repository declarations disagree: "
	corpusDependencyRepair     = "linked source requires repair or belongs to another vault"
	corpusUnknownCitation      = "portable code citation has unknown historical revision"
)
