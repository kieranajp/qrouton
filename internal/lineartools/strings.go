package lineartools

const (
	// ConfigPath is where Linear reads custom scripts from, in the form the user
	// is shown rather than the expanded one.
	ConfigPath = "~/.linear/coding-tools.json"

	// issueTemplate is the placeholder Linear substitutes the issue for.
	issueTemplate = "{{issue.identifier}}"
)

const (
	dirMode  = 0o755
	fileMode = 0o644
)
