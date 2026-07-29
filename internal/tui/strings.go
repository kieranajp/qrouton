package tui

// Copy and layout constants for the onboarding TUI: the key hints, status
// vocabulary, field labels, and the widths the views clamp to. Keeping the
// user-visible strings here means a wording change is one edit in one file
// rather than a hunt through the render functions.

const (
	appName = "qrouton"

	// Body width bounds. bodyWidthInset is View()'s horizontal padding; a
	// lipgloss box renders boxWidthInset wider than its Width, since the border
	// sits outside it.
	bodyWidthInset = 6
	minBodyWidth   = 50
	maxBodyWidth   = 100
	boxWidthInset  = 6

	// repoListWindow is how many repositories the picker shows at once, and
	// repoListLead how many rows stay visible above the cursor when scrolling.
	repoListWindow = 8
	repoListLead   = 6
)

// Field labels on the new-session form.
const (
	labelTicket      = "Ticket URL"
	labelName        = "Name"
	labelDescription = "Description"
	labelOwners      = "GitHub owners"
	labelRepos       = "Repositories"
	labelPrefix      = "Branch prefix"
	labelMode        = "Mode"
)

// Owner refresh status vocabulary, shown per configured owner.
const (
	statusFetching = "fetching…"
	statusUpdated  = "updated"
	statusFailed   = "failed"
)

// Mode names as the form renders them.
const (
	modeLabelRPI       = "RPI"
	modeLabelAssistant = "Assistant"

	modeHintRPI       = "orchestrated Research → Plan → Implement workflow"
	modeHintAssistant = "open-ended coding session · escalate to RPI anytime"
)

// Screen copy.
const (
	loadingBody = "Loading repositories…\n\nFetching configured GitHub owners in the background.\n\nesc back"

	errorTitle       = "Something needs attention"
	errorRetryGitHub = "retry GitHub"
	errorRetryAssemb = "retry assembly"
	errorKeyHints    = "[b] back  [r] "
	errorKeyQuit     = "  [q] quit"

	landingKeyHints = "↑↓ navigate   enter select   d delete   r refresh   q quit"
	runnerKeyHints  = "↑↓ navigate · enter create · esc back"
	deleteKeyHints  = "enter/y delete   esc/n cancel"
	formKeyHints    = "type in the repository list to filter · backspace clears it\n" +
		"↑↓ move · space select/cycle · tab next field · ←→ choice · enter continue · esc back"

	newSessionLabel    = "New session"
	noDescriptionLabel = "No description"
	descriptionTail    = "…"
	emptyFieldLabel    = "—"

	runnerTitle = "Choose a coding agent"

	deleteBody      = "This removes its worktrees and session files. Shared mirrors are kept."
	deleteDirtyBody = "Uncommitted files will be lost in:"
	deleteNoTarget  = "No session selected.\n\n[esc] back"

	assemblyConfigured = "✓ Session configuration"
	assemblyRestoring  = "◌ Restore missing worktrees"
	assemblyFooter     = "Mirrors and worktrees are being assembled…"

	githubStatusPrefix     = "GitHub: "
	githubStatusRefreshing = "refreshing…"
	githubStatusStale      = "cached · refresh failed"
	githubStatusConnected  = "connected"
)

// Markers and glyphs the views draw with.
const (
	glyphCursor    = "▸"
	glyphSelected  = "›"
	glyphPending   = "◌"
	glyphDone      = "✓"
	glyphFailed    = "✗"
	glyphExcluded  = "○"
	glyphActive    = "●"
	glyphReference = "◐"

	// The per-repository clone/fetch bar on the assembly screen. Narrow on
	// purpose: it sits after the repository name and the phase git reports.
	progressBarWidth      = 16
	progressBarFull       = "█"
	progressBarEmpty      = "░"
	progressPercentFormat = " %3d%%"

	roleLabelExcluded  = "excluded"
	roleLabelActive    = "active"
	roleLabelReference = "reference"
)

// Relative-time vocabulary.
const (
	timeUnknown    = "unknown"
	timeJustNow    = "just now"
	timeYesterday  = "yesterday"
	timeDateLayout = "2006-01-02"
)

// Format strings the views lay their columns out with.
const (
	deleteTitleFormat = "Delete %s?"

	repoCountFormat = "%d included · %d active"
	filterPrefix    = "filter · "
	slugPrefix      = "slug · "
	bulletPrefix    = "· "
	updatedPrefix   = " · updated "

	referenceDetailFormat = " → %s · " + roleLabelReference

	// repoInSessionDetail marks a row the escalation picker cannot change: the
	// session already holds it, and it stays exactly as it is. Terse because it
	// shares the detail column with the role and the push time, and the role
	// column to its left already names the role.
	repoInSessionDetail = " " + bulletPrefix + "in session"
	roleColumnFormat    = "%s %-9s"
	repoColumnFormat    = "%-26s %s"
	sessionTitleFormat  = "%-42s %s"

	branchFormat        = "%s/%s"
	branchPreviewFormat = "preview · " + branchFormat + "  (active repos only)"

	repoOwnerCountFormat = " · %d repositories · %d owners"
	workflowLineFormat   = "R %s   P %s   I %s"

	assemblyCreatingPrefix = "Creating "

	minutesAgoFormat = "%dm ago"
	hoursAgoFormat   = "%dh ago"
	daysAgoFormat    = "%d days ago"
)
