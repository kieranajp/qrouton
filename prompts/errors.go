package prompts

import "errors"

// Errors loading or rendering a prompt. Each means a prompt source is malformed,
// which is a bug in prompts/ rather than anything a user did.
var (
	ErrInvalidPromptID   = errors.New("invalid prompt id")
	ErrUnsupportedPrompt = errors.New("unsupported prompt id")
	ErrNoTemplate        = errors.New("no template for artefact kind")

	ErrNoFrontmatter           = errors.New("agent prompt has no frontmatter")
	ErrUnterminatedFrontmatter = errors.New("agent prompt has unterminated frontmatter")
	ErrIncompleteAgentPrompt   = errors.New("agent prompt requires name and description")
	ErrInvalidAgentFrontmatter = errors.New("agent prompt frontmatter is not valid YAML")

	// ErrUserOwnedAsset means a discovery file at the session root is not one
	// qrouton stamped, so overwriting it would destroy the user's own work.
	ErrUserOwnedAsset = errors.New("refusing to replace user-owned asset")
)
