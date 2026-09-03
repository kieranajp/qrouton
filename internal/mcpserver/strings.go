package mcpserver

const (
	// editorWindowName is the reserved registry key for the single editor
	// window; others are keyed by the caller-supplied name.
	editorWindowName = "editor"

	defaultCommandWindowName = "command"

	// diffWindowName labels an all-repos diff; a single repo appends its basename.
	diffWindowName      = "diff"
	diffWindowSeparator = ":"

	notifyWindowName = "notify"

	// readWindowLimit caps how much output read_window returns to the agent. The
	// tail is kept, because that is where fresh output lands.
	readWindowLimit = 20000

	shellBin       = "sh"
	shellLoginFlag = "-lc"

	currentDir = "."
)

const (
	commandWindowLabel = "▶ "
	diffWindowLabel    = "◆ "
	notifyWindowLabel  = "🔔 qrouton"
)

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

	noRepos           = "This session has no repositories yet."
	reposHeaderFormat = "Session repositories (%d):\n%s"
	repoLineFormat    = "- %s/%s (%s) at %s"
	repoLineRefFormat = "- %s/%s (%s @ %s) at %s"
	repoLineJoiner    = "\n"

	// escalationConfirmedMessage is the confirm-path return. In practice the
	// agent supervisor replaces this process before the poll observes a
	// confirmed outcome, so this string exists for completeness and for tests.
	escalationConfirmedMessage = "Escalated: the session is now in RPI mode."

	escalationCancelledMessage = "The picker was cancelled. Still Assistant — carry on."

	escalationTimeoutMessage = "The picker is still open; the user hasn't confirmed or cancelled yet."

	// All three hand back the whole resulting set: the user may have changed a
	// role, dropped something asked for or added something never mentioned, so
	// the set is the answer and the request is only what prompted it.
	reposConfirmedFormat = "The user answered the picker. %s"
	reposCancelledFormat = "The user cancelled; nothing was added or taken up. %s"
	reposStillOpenFormat = "The picker is still open; the user hasn't confirmed or cancelled yet. %s"

	// The shortfall names what the request did not get, so an agent does not read
	// its own request back out of the set and ask for the same thing again.
	reposShortfallFormat  = "\nYou asked for more than this: %s. Don't ask again without saying something new — check the spelling first, and take a no for an answer."
	shortfallAbsentFormat = "%s is not in the session"
	shortfallRoleFormat   = "%s is held as %s, not editing"
	repoShortfallJoiner   = "; "
	repoIDSeparator       = "/"
)

const toastFormat = "🔔  %s"

const (
	allReposDiffFormat = `for d in %s/*/; do git -C "$d" rev-parse --git-dir >/dev/null 2>&1 || continue; ` +
		`printf '\n=== %%s ===\n' "$d"; git -C "$d" --no-pager diff%s; done`

	singleRepoDiffFormat = `git -C %s --no-pager diff%s`

	stagedFlag = " --staged"
)

const (
	keyMessage = "message"
	keyOutput  = "output"
	keyRepos   = "repos"
)
