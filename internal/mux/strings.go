package mux

// Literals the multiplexer ports and the Zellij adapter depend on: backend
// identifiers that cross the exec boundary inside a Handle, the environment
// variable that pins the socket directory, and the names of the files the
// adapter stages into a session.

const (
	// KindZellij identifies the Zellij backend in a Handle and in config.
	KindZellij = "zellij"

	// socketDirEnvVar pins Zellij's socket directory. Lookup, Kill, Attach, and
	// the MCP child must all agree on it or they address different servers.
	socketDirEnvVar = "ZELLIJ_SOCKET_DIR"

	// defaultSocketDir is short on purpose: macOS $TMPDIR is long enough that
	// Zellij's socket path exceeds the 104-byte unix-socket cap for real session
	// names.
	defaultSocketDir = "/tmp/zellij"

	zellijBin        = "zellij"
	zellijConfigName = "zellij-config.kdl"
	zellijLayoutName = "layout.kdl"

	// envKeyValueSep separates a key from its value in an environ entry.
	envKeyValueSep = "="

	// minZellijMinor is the first Zellij 0.x release qrouton's layout and pane
	// actions work against.
	minZellijMinor = 44

	splitVertical   = "vertical"
	splitHorizontal = "horizontal"
)
