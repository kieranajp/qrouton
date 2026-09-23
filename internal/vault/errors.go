package vault

import "errors"

var (
	ErrInvalidProfile   = errors.New("vault profile needs an id of letters, digits, dot, dash or underscore, and an absolute root")
	ErrQueryRequired    = errors.New("query is required")
	ErrUnknownKind      = errors.New("unknown artifact kind; use research, spec, plan, note or dead_end")
	ErrModelChanged     = errors.New("embedding model changed while embedding")
	ErrRefRequired      = errors.New("ref is required")
	ErrUnknownDocument  = errors.New("no indexed vault document has that reference or id")
	ErrUnavailable      = errors.New("vault root is missing or unreadable")
	ErrNotDocument      = errors.New("not a regular Markdown file inside the vault")
	ErrModelMissing     = errors.New("embedding model is not installed in Ollama")
	ErrContextOverflow  = errors.New("embedding input exceeds the model context")
	ErrEmbeddingMissing = errors.New("Ollama returned the wrong number of embeddings")
)
