package mcpserver

// Window labels and the messages qrouton returns to the agent after driving a
// window. The messages double as instructions — they tell the agent what it can
// do next with the window it just opened — so they are worded here rather than
// assembled inline.

const (
	// editorWindowName is the reserved registry key for the single editor
	// window; others are keyed by the caller-supplied name.
	editorWindowName = "editor"

	// defaultCommandWindowName is used when run_command is called without a name.
	defaultCommandWindowName = "command"

	// diffWindowName labels an all-repos diff; a single repo appends its basename.
	diffWindowName      = "diff"
	diffWindowSeparator = ":"

	notifyWindowName = "notify"

	// escalateWindowName is the reserved registry key for the picker; a second
	// escalate replaces a stale one.
	escalateWindowName = "escalate"

	// readWindowLimit caps how much output read_window returns to the agent. The
	// tail is kept, because that is where fresh output lands.
	readWindowLimit = 20000

	shellBin       = "sh"
	shellLoginFlag = "-lc"

	currentDir = "."

	// The picker subcommand and flags escalate opens, resolved against the
	// running qrouton binary rather than a bare "qrouton" on PATH.
	pickSubcommand = "pick"
	sessionRootArg = "--session-root"
	nameArg        = "--name"
	prefixArg      = "--prefix"
	escalateArg    = "--escalate"
)

// Window titles, as they appear in the title bar.
const (
	editorWindowLabel   = "Editor"
	commandWindowLabel  = "▶ "
	diffWindowLabel     = "◆ "
	notifyWindowLabel   = "🔔 qrouton"
	escalateWindowLabel = "escalate"
)

// Messages returned to the agent.
const (
	openedFileFormat = "Opened %s at line %d in an editor tab " +
		"(it stays open until the user quits the editor; the conversation keeps focus)."

	sharedPageFormat = "Rendered %s as a self-contained page at %s, styled the way the " +
		"workbench draws it. Publish that file verbatim — it needs no styling of " +
		"your own and fetches nothing at runtime — then give the user the link. " +
		"qrouton does not send it anywhere."

	renderedFileFormat = "Showing %s rendered in a pane the user can read and scroll " +
		"(it stays open until they close it; the conversation keeps focus). %s"

	renderedSpanVisibleFormat = "Showing %s rendered in a pane, scrolled to %s with the requested source " +
		"range visible in a measured block (the conversation keeps focus). %s"

	renderedSpanUnverifiedFormat = "Showing %s rendered in a pane and requested %s, but its position could " +
		"not be verified (the conversation keeps focus). %s"

	singleLineFormat = "line %d"
	lineRangeFormat  = "lines %d-%d"

	runningFormat = "Running in tab %q (cwd %s). The user can close the tab, " +
		"or call " + toolReadWindow + " with name %q to see its " +
		"output and " + toolCloseWindow + " %q to close it."

	noOutputFormat  = "Window %q has produced no output yet."
	truncatedPrefix = "…(earlier output truncated)…\n"

	viewportUnavailableUnselected = "Viewport: unavailable because the document tab is not selected."
	viewportUnavailableSelected   = "Viewport: selected, but browser geometry is unavailable."
	viewportMeasuredEmpty         = "Viewport: selected and measured; no source-mapped blocks are visible."
	viewportMeasuredFormat        = "Viewport: selected and measured; visible source blocks: %s."
	viewportRangeJoiner           = ", "

	closedFormat = "Closed window %q."

	showingDiffFormat = "Showing the diff for %s in window %q."
	emptyDiffFormat   = "No changes in %s."
	allReposScope     = "all session repos"
	sessionRootScope  = "the session root"

	notifiedFormat = "Notified the user: %s"

	noWindowsOpen     = "No qrouton-managed windows are open."
	openWindowsPrefix = "Open windows: "
	openWindowsSuffix = "."
	windowNameJoiner  = ", "

	// escalationConfirmedMessage is the confirm-path return. In practice the
	// agent supervisor replaces this process before the poll observes a
	// confirmed outcome, so this string exists for completeness and for tests.
	escalationConfirmedMessage = "Escalated: the session is now in RPI mode."

	escalationCancelledMessage = "The picker was cancelled. Still Assistant — carry on."

	escalationTimeoutMessage = "The picker is still open; the user hasn't confirmed or cancelled yet."
)

const toastFormat = "🔔  %s"

// The shell show_diff captures. Both forms disable Git's pager; the all-repos
// form walks the worktrees.
const (
	allReposDiffFormat = `for d in %s/*/; do git -C "$d" rev-parse --git-dir >/dev/null 2>&1 || continue; ` +
		`printf '\n=== %%s ===\n' "$d"; git -C "$d" --no-pager diff%s; done`

	singleRepoDiffFormat = `git -C %s --no-pager diff%s`

	stagedFlag = " --staged"
)

// Keys the tools' structured payloads carry their text under. read_window is
// returning a window's output; everything else is talking to the agent.
const (
	keyMessage = "message"
	keyOutput  = "output"
)
