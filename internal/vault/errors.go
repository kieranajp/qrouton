package vault

import "errors"

var (
	ErrInvalidProfile   = errors.New("a thoughts root needs an id of letters, digits, dot, dash or underscore, and an absolute path")
	ErrDuplicateProfile = errors.New("two thoughts roots share an id")
	ErrOverlappingRoots = errors.New("thoughts roots must not share, nest in, or link into each other")
	ErrQueryRequired    = errors.New("query is required")
	ErrUnknownKind      = errors.New("unknown artifact kind; use research, spec, plan, note or dead_end")
	ErrModelChanged     = errors.New("embedding model changed while embedding")
	ErrRefRequired      = errors.New("ref is required")
	ErrUnknownDocument  = errors.New("no indexed vault document has that reference or id")
	ErrUnavailable      = errors.New("vault root is missing or unreadable")
	ErrNotDocument      = errors.New("not a regular Markdown file inside the vault")
	ErrModelMissing     = errors.New("embedding model is not installed in Ollama")
	ErrContextOverflow  = errors.New("embedding input exceeds the model context")
	ErrRejected         = errors.New("Ollama rejected the request")
	ErrEmbeddingMissing = errors.New("Ollama returned the wrong number of embeddings")
)
