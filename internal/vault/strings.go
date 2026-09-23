package vault

import "time"

const (
	OllamaModel = "all-minilm:22m-l6-v2-fp16"
	OllamaURL   = "http://127.0.0.1:11434"

	schemaVersion = 1
	// preprocessing versions the chunking and window inputs the cache was built from.
	preprocessing = 1

	windowChars      = 1000
	embedBatch       = 64
	walkInterval     = 2 * time.Second
	probeTimeout     = 2 * time.Second
	maxDocumentBytes = 1 << 20
	readLimit        = 20000
	snippetChars     = 320

	rrfK         = 60
	defaultLimit = 5
	maxLimit     = 20
	saveEvery    = 8
	channelTop   = 3
	bm25K1       = 1.2
	bm25B        = 0.75
)

const (
	StatusReady       = "ready"
	StatusPartial     = "partial"
	StatusUnavailable = "unavailable"

	AdmittedRRF   = "rrf"
	AdmittedBM25  = "bm25_top"
	AdmittedDense = "dense_top"

	KindResearch = "research"

	RelationDerivedFrom = "derived_from"
	RelationSupersedes  = "supersedes"
)

var kinds = map[string]bool{KindResearch: true, "spec": true, "plan": true, "note": true, "dead_end": true}

const (
	frontmatterOpen  = "---\n"
	frontmatterClose = "\n---"
	refSeparator     = ":"
	crumbSeparator   = " > "
	ellipsis         = "…"
	cacheExtension   = ".gob"
	cacheDirName     = "qrouton/vault"
	cacheMode        = 0o644
	cacheDirMode     = 0o755

	ollamaTags        = "/api/tags"
	ollamaEmbed       = "/api/embed"
	ollamaContentType = "application/json"
	ollamaOverflow    = "exceeds the context length"
)

var stopwords = `a an the and or of to in on for with by at from as is are was were be been being it its this that these those what which who whom how why when where does do did has have had can could should would will shall may might must not no than then there their they them we our us you your i me my he she his her so if but about into over after before under between through during without within across also any each every some such only own same other more most very just`
