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

	renderedFileFormat = "Showing %s rendered in a pane the user can read and scroll " +
		"(it stays open until they close it; the conversation keeps focus)."

	renderedSpanFormat = "Showing %s rendered in a pane, scrolled to %s with those lines marked " +
		"(it stays open until the user closes it; the conversation keeps focus)."

	singleLineFormat = "line %d"
	lineRangeFormat  = "lines %d-%d"

	runningFormat = "Running in tab %q (cwd %s). The user can close the tab, " +
		"or call " + toolReadWindow + " with name %q to see its " +
		"output and " + toolCloseWindow + " %q to close it."

	noOutputFormat  = "Window %q has produced no output yet."
	truncatedPrefix = "…(earlier output truncated)…\n"

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
