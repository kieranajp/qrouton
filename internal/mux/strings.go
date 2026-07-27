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

	// minZellijMajor and minZellijMinor are the first Zellij release whose
	// layout schema and pane actions qrouton works against.
	minZellijMajor = 0
	minZellijMinor = 44

	// versionSeparator splits Zellij's "zellij 0.44.0" version output, and
	// versionComponentSep its numeric components.
	versionSeparator    = " "
	versionComponentSep = "."
	versionFieldCount   = 2

	// SplitVertical and SplitHorizontal are the Node.Split values a layout
	// author sets; the adapter renders them as Zellij split directions.
	SplitVertical   = "vertical"
	SplitHorizontal = "horizontal"
)

// Zellij subcommands and action names the adapter shells out to.
const (
	versionFlag = "--version"
	configFlag  = "--config"

	listSessionsCmd    = "list-sessions"
	listSessionsNoFmt  = "-n"
	deleteSessionCmd   = "delete-session"
	forceFlag          = "--force"
	attachCmd          = "attach"
	newSessionWithFlag = "--new-session-with-layout"
	sessionFlag        = "--session"
	actionCmd          = "action"

	newPaneAction        = "new-pane"
	closePaneAction      = "close-pane"
	dumpScreenAction     = "dump-screen"
	toggleFloatingAction = "toggle-floating-panes"

	floatingFlag    = "--floating"
	pinnedFlag      = "--pinned"
	xFlag           = "--x"
	yFlag           = "--y"
	widthFlag       = "--width"
	heightFlag      = "--height"
	nameFlag        = "--name"
	cwdFlag         = "--cwd"
	closeOnExitFlag = "--close-on-exit"
	paneIDFlag      = "--pane-id"
	fullFlag        = "--full"
	endOfFlags      = "--"

	trueValue = "true"

	// exitedMarker appears in list-sessions output for a session that has been
	// recorded but has exited.
	exitedMarker = "EXITED"

	// configAssetPath is the vendored Zellij config staged into every session.
	configAssetPath = "assets/zellij-config.kdl"

	// sessionDirPlaceholder marks where Stage substitutes the session directory
	// into the vendored config's Run-block keybindings.
	sessionDirPlaceholder = "@@SESSION_DIR@@"
)

// KDL fragments the layout renderer emits. Zellij layouts are indented KDL, so
// the chrome and the indent unit are spelled out once here.
const (
	kdlIndent = "    "

	kdlLayoutOpen   = "layout {\n"
	kdlBlockClose   = "}\n"
	kdlTabBar       = kdlIndent + "pane size=1 borderless=true {\n" + kdlIndent + kdlIndent + "plugin location=\"zellij:tab-bar\"\n" + kdlIndent + "}\n"
	kdlStatusBar    = kdlIndent + "pane size=2 borderless=true {\n" + kdlIndent + kdlIndent + "plugin location=\"zellij:status-bar\"\n" + kdlIndent + "}\n"
	kdlFloatingOpen = kdlIndent + "floating_panes {\n"

	kdlPaneKeyword   = "pane"
	kdlCommandFormat = "command %q\n"
	kdlArgsKeyword   = "args "
	kdlSizeAttr      = " size="
	kdlNameAttr      = " name=%q"
	kdlCloseOnExit   = " close_on_exit=true"
	kdlFocus         = " focus=true"
	kdlSplitAttr     = "split_direction=%q"
	kdlGeometryAttrs = "x=%q y=%q width=%q height=%q name=%q"

	// kdlSessionName makes the generated session self-attaching, which is how a
	// fresh launch lands the user inside it.
	kdlSessionName = "session_name %q\nattach_to_session true\n"
)
