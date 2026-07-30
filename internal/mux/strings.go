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
	socketDirEnvVar     = "ZELLIJ_SOCKET_DIR"
	zellijPaneIDEnvVar  = "ZELLIJ_PANE_ID"
	zellijSessionEnvVar = "ZELLIJ_SESSION_NAME"

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

	listSessionsCmd   = "list-sessions"
	listSessionsNoFmt = "-n"
	deleteSessionCmd  = "delete-session"
	forceFlag         = "--force"
	attachCmd         = "attach"
	layoutFlag        = "--layout"
	createBackground  = "--create-background"
	sessionFlag       = "--session"
	actionCmd         = "action"

	newPaneAction        = "new-pane"
	closePaneAction      = "close-pane"
	dumpScreenAction     = "dump-screen"
	listPanesAction      = "list-panes"
	renamePaneAction     = "rename-pane"
	stackPanesAction     = "stack-panes"
	toggleFloatingAction = "toggle-floating-panes"
	listClientsAction    = "list-clients"

	// focusPaneIDAction focuses a pane by id. There is no focus-by-*name*
	// action, which is why qrouton keeps its own name-to-id registry, but ids
	// it already holds are directly addressable.
	//
	// Added in Zellij 0.44.1, one patch above qrouton's floor — and the floor
	// check only reads the minor. Rather than reject 0.44.0 over a nicety,
	// zellijHost.returnFocus treats a failure here as "not available" and falls
	// back to toggling the floating layer.
	focusPaneIDAction = "focus-pane-id"

	// repositionAction re-resolves a floating pane's coordinates against the
	// current viewport. Its x/y/width/height take the same bare-integer or
	// percent values new-pane does, so one Geometry serves both, and the pinned
	// and borderless flags it also accepts are Option<bool> — omitting them
	// leaves the pane's own settings alone. Present since 0.44.0.
	repositionAction = "change-floating-pane-coordinates"

	// listClientsHeader is the column header list-clients prints even when no
	// client is attached; a row after it is a real client.
	listClientsHeader = "CLIENT_ID"

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
	allFlag         = "--all"
	jsonFlag        = "--json"
	endOfFlags      = "--"

	trueValue          = "true"
	terminalPanePrefix = "terminal_"
	pluginPanePrefix   = "plugin_"

	// exitedMarker appears in list-sessions output for a session that has been
	// recorded but has exited.
	exitedMarker = "EXITED"

	// configAssetPath is the vendored Zellij config staged into every session.
	configAssetPath = "assets/zellij-config.kdl"

	// sessionDirPlaceholder marks where Stage substitutes the session directory
	// into the vendored config's Run-block keybindings.
	sessionDirPlaceholder = "@@SESSION_DIR@@"

	// helpScriptPlaceholder marks where Stage substitutes the global
	// quick-reference panel's path into the Alt-? Run-block keybinding. mux
	// deliberately doesn't know qrouton's config layout — the caller resolves
	// the path and hands it in via Workspace.HelpScript.
	helpScriptPlaceholder = "@@HELP_SCRIPT@@"

	// binaryPlaceholder marks where Stage substitutes qrouton's own executable
	// into the Run-block keybindings that invoke a subcommand (Alt-g, Alt-e,
	// Alt-n).
	// An absolute path, not a bare name: a locally built binary is usually not
	// on PATH, and a chord that works only after `make install` is a chord
	// that looks broken.
	binaryPlaceholder = "@@QROUTON_BIN@@"
)

// KDL fragments the layout renderer emits. Zellij layouts are indented KDL, so
// the chrome and the indent unit are spelled out once here.
const (
	kdlIndent = "    "

	kdlLayoutOpen = "layout {\n"
	kdlBlockClose = "}\n"
	// status-bar, for the keybinding hints after Ctrl-g: compact-bar showed the
	// mode but nothing about what the mode could do, which made it useless in
	// practice. It renders from the live keybind set, so it reflects this config
	// rather than Zellij's defaults. One row, as stock; the tab strip stays out
	// since a qrouton session has a single tab.
	kdlBar = kdlIndent + "pane size=1 borderless=true {\n" + kdlIndent + kdlIndent + "plugin location=\"zellij:status-bar\"\n" + kdlIndent + "}\n"

	kdlPaneKeyword   = "pane"
	kdlCommandFormat = "command %q\n"
	kdlArgsKeyword   = "args "
	kdlSizeAttr      = " size="
	kdlNameAttr      = " name=%q"
	kdlCloseOnExit   = " close_on_exit=true"
	kdlBorderless    = " borderless=true"
	kdlFocus         = " focus=true"
	kdlSplitAttr     = "split_direction=%q"
	kdlStackedAttr   = "stacked=true"

	// kdlSessionName makes the generated session self-attaching, which is how a
	// fresh launch lands the user inside it.
	kdlSessionName = "session_name %q\nattach_to_session true\n"
)
