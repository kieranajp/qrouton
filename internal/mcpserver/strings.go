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

	// readPaneLimit caps how much pane output read_pane returns to the agent. The
	// tail is kept, because that is where fresh command output lands.
	readPaneLimit = 20000

	shellBin       = "sh"
	shellLoginFlag = "-lc"

	currentDir = "."
)

// Pane labels, as they appear in the pane's title bar. Each names the keys that
// act on it, since the pane is the only place the user sees them.
const (
	editorPaneLabel  = "Editor — Alt-f to view · Alt-x to close"
	commandPaneLabel = "▶ "
	diffPaneLabel    = "◆ "
	notifyPaneLabel  = "🔔 notification"
)

// Messages returned to the agent.
const (
	openedFileFormat = "Opened %s at line %d in the editor pane " +
		"(stays open for reference; focus is back on the agent)."

	runningFormat = "Running in pane %q (cwd %s). Call " + toolReadPane +
		" with name %q to see its output; " + toolClosePane + " %q to dismiss it."

	noOutputFormat  = "Pane %q has produced no output yet."
	truncatedPrefix = "…(earlier output truncated)…\n"

	closedFormat = "Closed pane %q."

	showingDiffFormat = "Showing the diff for %s in pane %q (Alt-f to scroll it)."
	allReposScope     = "all session repos"
	sessionRootScope  = "the session root"

	notifiedFormat = "Notified the user: %s"

	noPanesOpen     = "No qrouton-managed panes are open."
	openPanesPrefix = "Open panes: "
	openPanesSuffix = "."
	paneNameJoiner  = ", "
)

// The toast notify opens: it rings the bell, plays the generated cross-platform
// sound (best effort), shows the message, then closes itself.
const (
	toastCommandFormat = `%s >/dev/null 2>&1 & printf '\a\n  🔔  %%s\n\n  (auto-closes; Alt-x to dismiss)\n' %s; sleep %d`
	toastSeconds       = 8
)

// The shell show_diff runs. A single repo relies on git's own pager and colour
// (the pane is a tty); the all-repos form forces colour through an explicit
// pager as it walks the worktrees. The footer keeps an empty diff from rendering
// as a blank pane.
const (
	diffFooter = `printf '\n[end of diff — Alt-x to close]\n'`

	allReposDiffFormat = `for d in %s/*/; do git -C "$d" rev-parse --git-dir >/dev/null 2>&1 || continue; ` +
		`printf '\n=== %%s ===\n' "$d"; git -C "$d" -c color.ui=always diff%s; done | less -FRX; %s`

	singleRepoDiffFormat = `git -C %s diff%s; %s`

	stagedFlag = " --staged"
)
