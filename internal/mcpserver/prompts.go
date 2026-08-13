package mcpserver

// The MCP surface as the agent reads it: the server instructions, and each
// tool's name and description. These are prompt text, not code — the agent's
// only guide to when a window is the right move — so they live together where
// they can be read and revised as a set.

const serverInstructions = "Drive the user's qrouton workbench. Every tool surface is a tab in the session's right pane beside the conversation. Opening one leaves the keyboard where it is, so the user can watch while you work and keep chatting. A tab whose command exits cleanly goes with it; one whose command fails stays so the error remains readable, and either way the tab says whether its process is running, succeeded or failed. Use open_file to show a document (especially after creating one); run_command to run long-lived or noisy work (dev servers, watchers, builds, log tails) where the user can see it instead of in your own shell — that tab is interactive, so Ctrl-C there reaches the process; read_window to inspect its output; show_diff to display a repo's changes for review; notify to get the user's attention when you finish or need them; close_window/list_windows to manage what you have open; escalate to hand a piece of work off to the full RPI workflow once you've drafted a brief. Escalation uses the repository picker in the workbench. All paths and working directories must belong to this session."

// Tool names, as the agent calls them and as qrouton reports them back in its
// own messages.
const (
	toolOpenFile    = "open_file"
	toolRunCommand  = "run_command"
	toolReadWindow  = "read_window"
	toolShowDiff    = "show_diff"
	toolNotify      = "notify"
	toolCloseWindow = "close_window"
	toolListWindows = "list_windows"
	toolEscalate    = "escalate"
)

// Tool descriptions.
const (
	descOpenFile = "Show the user an existing session file. A markdown file is rendered as a formatted pane — headings, task lists, highlighted code, and its source line numbers down the left — and anything else opens in their configured terminal editor at the given line. Either way it becomes the selected tab, so it is not left behind another one, and the keyboard stays with the conversation. A rendered document stays open for reference; an editor's tab closes when the user quits it. Use this after writing a document when showing it to the user is helpful. Set line (and through, for a range) to point at a particular passage of a long document: the pane opens scrolled to it with those lines marked, which beats asking the user to go hunting for the section you mean."

	descRunCommand = "Run a shell command in a tab instead of your own shell. Ideal for long-running or noisy processes (dev servers, test watchers, builds, log tails) the user should see live: it is interactive, so Ctrl-C there reaches the process. The keyboard stays with the conversation, reusing a name replaces that tab, and a command that succeeds closes it while one that fails leaves it open with its error. Its tab reports whether the command is running, succeeded or failed. Read its output later with read_window."

	descReadWindow = "Capture the current output of a window opened with run_command (or open_file) and return it as text. Use this to check on a command you started — for example to confirm a dev server booted or to read a test run's failures. Set full to include the scrollback."

	descShowDiff = "Show a repo's git diff for the user to review, rendered as a formatted pane that stays open until they close it. Give repo as a worktree path within the session (e.g. src/app), or omit it to diff every session repo. Use base to compare against a ref (e.g. the default branch) or staged for index changes."

	descNotify = "Get the user's attention with an on-screen message and a sound. The message becomes a tab marked as wanting the user, and stays until they dismiss it; it does not take the keyboard. Use this sparingly — when you finish a long task, need a decision, or are blocked — since the user may have stepped away while work runs."

	descCloseWindow = "Close a window you opened — run_command, open_file, show_diff or notify — by name."

	descListWindows = "List the tabs qrouton is holding for you, by name."

	descEscalate = "Hand this piece of work off to the full Research → Plan → Implement workflow. Before calling this, write .qrouton/handoff.md with a short brief (what the work is, what's established, what's ruled out, what's still open) — it becomes the system prompt of the fresh orchestrator that replaces you. Give name for the piece of work and, optionally, branch_prefix (one of feat, fix, chore, refactor, docs, test). This opens the repository picker; the user chooses repositories and confirms or cancels there. On confirm, your process is replaced and this call never returns. On cancel, it returns and you continue as the assistant."
)
