package mcpserver

// The mode prompt already describes the workbench and when a window is worth
// opening, and each description below owns its own tool's mechanics. What is
// left for the server to say is the one rule neither of them can: the session
// bounds every path this server will accept.

const serverInstructions = "Drive the user's qrouton workbench: most tools here open or read a tab in the session's right pane, beside the conversation and without taking the keyboard, while the repository tools read and grow the workspace itself. All paths and working directories must belong to this session."

const (
	toolOpenFile    = "open_file"
	toolRunCommand  = "run_command"
	toolReadWindow  = "read_window"
	toolShowDiff    = "show_diff"
	toolNotify      = "notify"
	toolCloseWindow = "close_window"
	toolListWindows = "list_windows"
	toolEscalate    = "escalate"
	toolSharePage   = "share_page"
	toolListRepos   = "list_repos"
	toolAddRepos    = "add_repos"
)

const (
	descOpenFile = "Show the user an existing session file. A markdown file is rendered as a formatted pane — headings, task lists, highlighted code, and its source line numbers down the left — and anything else opens in their configured terminal editor at the given line. The keyboard stays with the conversation. A rendered document stays open for reference; an editor's tab closes when the user quits it. Set line (and through, for a range) to mark a passage. The result reports measured visible source-block intervals and only claims scrolling when one intersects the request. Use read_window on editor to verify the current viewport later. Tabs begin in the background except thoughts/ artifacts, which select by default; foreground overrides either default without taking the keyboard."

	descRunCommand = "Run a shell command in a tab instead of your own shell. Ideal for long-running or noisy processes (dev servers, test watchers, builds, log tails) the user should see live: it is interactive, so Ctrl-C there reaches the process. The keyboard stays with the conversation, reusing a name replaces that tab, and a command that succeeds closes it while one that fails leaves it open with its error. Its tab reports whether the command is running, succeeded or failed. Read its output later with read_window. Tabs begin in the background; foreground selects one without taking the keyboard."

	descReadWindow = "Capture the current output of a window opened with run_command or open_file. Markdown results also include the current measured viewport as one-based inclusive source-block intervals, merged in source order; unavailable differs from a measured empty interval list. Set full to include terminal scrollback."

	descShowDiff = "Show a repo's git diff for the user to review, rendered as a formatted pane that stays open until they close it. Give repo as a worktree path within the session (e.g. src/app), or omit it to diff every session repo. Use base to compare against a ref (e.g. the default branch) or staged for index changes. Tabs begin in the background; foreground selects one without taking the keyboard."

	descNotify = "Get the user's attention with an on-screen message and a sound. The message becomes a tab marked as wanting the user, and stays until they dismiss it; it does not take the keyboard. It begins in the background, ordinarily relying on sound and its waiting marker; foreground selects it without taking the keyboard. Use this sparingly — when you finish a long task, need a decision, or are blocked — since the user may have stepped away while work runs."

	descCloseWindow = "Close a window you opened — run_command, open_file, show_diff or notify — by name."

	descListWindows = "List the tabs qrouton is holding for you, by name."

	descSharePage = "Render a session document as a self-contained page carrying qrouton's own styling — its palette, its fonts and the same prose renderer the workbench draws with — so it can be handed to somebody outside this session. Give path, relative to the session root (e.g. thoughts/shared/plans/thing.md). This writes the page and returns its path; it does not send it anywhere. Publish that file yourself, verbatim, with whatever tool you have for it, and give the user the link. The page fetches nothing at runtime, so it survives a strict content-security policy, and it carries no html, head or body tag of its own."

	descListRepos = "List the repositories this session holds, with each one's role (editing repos sit on the session branch and may be changed; reference repos are detached at a pinned commit and are read-only), its branch or pinned revision, and its worktree path relative to the session root. Reads the session manifest and opens no tab."

	descAddRepos = "Add repositories to this session's workspace yourself, rather than asking the user to. Unattended: there is no picker and nobody confirms it. Give repos as a list of {name, role}; name is org/name, or a bare name when it is unambiguous. role is editing (on the session branch, changes allowed) or reference (detached at a pinned commit, read-only) and defaults to reference. The two directions are not symmetric: naming a repository this session already reads with role editing takes it up onto the session branch, while naming one it already edits with role reference changes nothing at all — nothing here ever makes a repository read-only again. A repository already held in the role asked for is left exactly as it is and reported as such. A name that does not resolve fails the whole call before anything is composed. This blocks until the worktrees exist, which for a repository with no local mirror yet means a clone that can take minutes; a tab shows the user what is happening. The workspace picks the new repository up on its own, and your conversation is not restarted."

	descEscalate = "Hand this piece of work off to the full Research → Plan → Implement workflow. Before calling this, write .qrouton/handoff.md with a short brief (what the work is, what's established, what's ruled out, what's still open) — it becomes the system prompt of the fresh orchestrator that replaces you. Give name for the piece of work and, optionally, branch_prefix (one of feat, fix, chore, refactor, docs, test). This opens the repository picker; the user chooses repositories and confirms or cancels there. On confirm, your process is replaced and this call never returns. On cancel, it returns and you continue as the assistant."
)
