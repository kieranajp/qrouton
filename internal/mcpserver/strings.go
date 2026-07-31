package mcpserver

// Pane labels, geometry, and the messages qrouton returns to the agent after
// driving a pane. The messages double as instructions — they tell the agent what
// it can do next with the pane it just opened — so they are worded here rather
// than assembled inline.

const (
	// editorPaneName is the reserved registry key for the single editor pane;
	// other panes are keyed by the caller-supplied name.
	editorPaneName = "editor"

	// defaultCommandPaneName is used when run_command is called without a name.
	defaultCommandPaneName = "command"

	// diffPaneName labels an all-repos diff; a single repo appends its basename.
	diffPaneName      = "diff"
	diffPaneSeparator = ":"

	notifyPaneName = "notify"

	// helpPaneName is the registry key for the quick-reference panel, so a
	// second help call replaces a panel the user left open rather than
	// stacking another on top of it.
	helpPaneName = "help"

	// escalatePaneName is the reserved registry key for the picker pane the
	// escalate tool spawns; a second escalate call replaces a stale picker,
	// matching how a second open_file replaces the previous editor pane. It
	// also matches the pane name the Alt-e keybinding gives its own picker.
	escalatePaneName  = "escalate"
	escalatePaneLabel = "escalate · Esc to cancel"

	// readPaneLimit caps how much pane output read_pane returns to the agent. The
	// tail is kept, because that is where fresh command output lands.
	readPaneLimit = 20000

	shellBin       = "sh"
	shellLoginFlag = "-lc"

	currentDir = "."

	// The picker subcommand and flags escalate spawns, resolved against the
	// running qrouton binary (os.Executable()) rather than a bare "qrouton" on
	// PATH — the same self-exec convention internal/launch/supervise.go uses.
	pickSubcommand = "pick"
	sessionRootArg = "--session-root"
	nameArg        = "--name"
	prefixArg      = "--prefix"
)

// Pane labels, as they appear in the pane's title bar. Each names the keys that
// act on it, since the pane is the only place the user sees them.
//
// The editor's label deliberately promises no Esc: the pane holds the user's
// real editor, whose own Esc means what that editor says it means. Quitting it
// is what closes the pane.
const (
	editorPaneLabel  = "agent · Editor · Alt-f to view · quit to close"
	commandPaneLabel = "agent · ▶ "
	diffPaneLabel    = "agent · ◆ "
	notifyPaneLabel  = "agent · 🔔 notification · Esc to close"
	dismissPaneLabel = " · Esc to close"
)

// Messages returned to the agent.
const (
	openedFileFormat = "Opened %s at line %d in the editor pane " +
		"(stays open for reference until the user quits the editor; focus is back on the agent)."

	runningFormat = "Running in pane %q (cwd %s). The user can press Esc to stop and dismiss " +
		"the pane, or call " + toolReadPane + " with name %q to see its " +
		"output and " + toolClosePane + " %q to close it."

	noOutputFormat  = "Pane %q has produced no output yet."
	truncatedPrefix = "…(earlier output truncated)…\n"

	closedFormat    = "Closed pane %q."
	minimizedFormat = "Minimized pane %q into the dock; it is still running. " +
		"Use " + toolRestore + " to return it to the right-hand overlay."
	restoredFormat         = "Restored pane %q from the dock."
	alreadyMinimizedFormat = "Pane %q is already minimized in the dock."
	alreadyVisibleFormat   = "Pane %q is already visible in the right-hand overlay."

	showingDiffFormat = "Showing the diff for %s in pane %q (Alt-f to scroll it)."
	allReposScope     = "all session repos"
	sessionRootScope  = "the session root"

	notifiedFormat = "Notified the user: %s"

	helpShownMessage = "Floated the workspace quick-reference panel; it has keyboard focus " +
		"and closes on Esc. Answer the user's question too."

	noPanesOpen     = "No qrouton-managed panes are open."
	openPanesPrefix = "Open panes: "
	openPanesSuffix = "."
	paneNameJoiner  = ", "

	// escalationConfirmedMessage is the confirm-path return. In practice the
	// agent supervisor replaces this process before the poll below ever
	// observes a confirmed outcome, so this string exists for completeness
	// and for tests, not because the agent is expected to read it.
	escalationConfirmedMessage = "Escalated: the session is now in RPI mode."

	escalationCancelledMessage = "The picker was cancelled. Still Assistant — carry on."

	escalationTimeoutMessage = "The picker is still open; the user hasn't confirmed or cancelled yet."
)

// The toast notify opens: it rings the bell, plays the generated cross-platform
// sound (best effort), and shows the message. Waiting is not its business —
// dismissible adds the wait, with toastSeconds as the auto-close.
const (
	toastCommandFormat = `%s >/dev/null 2>&1 & printf '\a\n  🔔  %%s\n\n  (Esc dismisses; auto-closes)\n' %s`
	toastSeconds       = 8
)

// The shell show_diff runs. Both forms disable Git's pager so the shared Esc
// wait owns terminal input; the all-repos form forces colour as it walks the
// worktrees, and Zellij owns scrolling. The footer keeps an empty diff from
// rendering as a blank pane; dismissible keeps the output up until dismissal.
const (
	diffFooter = `printf '\n[end of diff — Esc to close]\n'`

	allReposDiffFormat = `for d in %s/*/; do git -C "$d" rev-parse --git-dir >/dev/null 2>&1 || continue; ` +
		`printf '\n=== %%s ===\n' "$d"; git -C "$d" -c color.ui=always --no-pager diff%s; done; %s`

	singleRepoDiffFormat = `git -C %s --no-pager diff%s; %s`

	stagedFlag = " --staged"

	// dismissibleFormat runs a pane's non-interactive payload beside the shared
	// Esc wait. The wait owns terminal input from the moment the pane appears,
	// and the wrapper stops and reaps a payload that is still alive when the
	// user dismisses it. A completed payload's output remains until then.
	dismissibleFormat = `(%s) </dev/null & qrouton_payload_pid=$!; %s; ` +
		`kill "$qrouton_payload_pid" 2>/dev/null; wait "$qrouton_payload_pid" 2>/dev/null || :`
)
