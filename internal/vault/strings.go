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
