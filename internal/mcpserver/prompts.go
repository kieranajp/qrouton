package mcpserver

// The MCP surface as the agent reads it: the server instructions, and each
// tool's name and description. These are prompt text, not code — the agent's
// only guide to when a pane is the right move — so they live together where
// they can be read and revised as a set.

const serverInstructions = "Drive the user's qrouton workspace. Panes you open are floating, pinned, and leave focus on the agent, so the user can watch them while chatting. Use open_file to show a document (especially after creating one); run_command to run long-lived or noisy work (dev servers, watchers, builds, logs) in a visible pane instead of your own shell; read_pane to inspect that output; show_diff to display a repo's changes for review; notify to get the user's attention when you finish or need them; close_pane/list_panes to manage them. All paths and working directories must belong to this session."

// Tool names, as the agent calls them and as qrouton reports them back in its
// own messages.
const (
	toolOpenFile   = "open_file"
	toolRunCommand = "run_command"
	toolReadPane   = "read_pane"
	toolShowDiff   = "show_diff"
	toolNotify     = "notify"
	toolClosePane  = "close_pane"
	toolListPanes  = "list_panes"
)

// Tool descriptions.
const (
	descOpenFile = "Open an existing session file in the user's configured terminal editor pane. The pane stays open for reference while the user keeps chatting with the agent. Use this after creating a document when showing it to the user is helpful."

	descRunCommand = "Run a shell command in a visible workspace pane instead of your own shell. Ideal for long-running or noisy processes (dev servers, test watchers, builds, log tails) the user should see live. The pane is floating and pinned, focus stays on the agent, and reusing a name replaces that pane. Read its output later with read_pane."

	descReadPane = "Capture the current output of a pane opened with run_command (or open_file) and return it as text. Use this to check on a command you started — for example to confirm a dev server booted or to read a test run's failures. Set full to include the scrollback."

	descShowDiff = "Show a repo's git diff in a workspace pane for the user to review. Give repo as a worktree path within the session (e.g. src/app), or omit it to diff every session repo. Use base to compare against a ref (e.g. the default branch) or staged for index changes."

	descNotify = "Get the user's attention with an on-screen toast, the terminal bell, and a sound. Use this sparingly — when you finish a long task, need a decision, or are blocked — since the user may have stepped away while work runs."

	descClosePane = "Close a pane previously opened with run_command or open_file, by name."

	descListPanes = "List the panes qrouton is currently managing for you, by name."
)
