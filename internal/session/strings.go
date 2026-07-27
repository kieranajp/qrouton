package session

// Git plumbing and path fragments the session lifecycle depends on, plus the
// scaffold a fresh session starts with.

const (
	// slugSeparator joins and pads slug components; branchSeparator divides a
	// branch prefix from its slug.
	slugSeparator   = "-"
	branchSeparator = "/"

	dirMode  = 0o755
	fileMode = 0o644

	gitBin = "git"

	// dirFlag runs a git command against a directory without cd-ing to it.
	dirFlag = "-C"

	// remoteName is the only remote a mirror has: qrouton clones from the
	// canonical origin and never adds another.
	remoteName = "origin"

	// remoteRefPrefix addresses a branch as the mirror sees it.
	remoteRefPrefix = remoteName + "/"

	// mirrorsDirName holds the shared bare mirrors, one per repository, beneath
	// the workspace root rather than inside any session.
	mirrorsDirName = ".mirrors"

	// gitDirSuffix is the extension a bare clone carries.
	gitDirSuffix = ".git"

	// sshURLFormat is the clone URL qrouton assumes for a GitHub repository when
	// the API did not supply one.
	sshURLFormat = "git@github.com:%s/%s" + gitDirSuffix

	// commitRefSuffix resolves a ref to the commit it points at, so a tag or an
	// annotated object still pins a revision.
	commitRefSuffix = "^{commit}"

	// notARepositoryMessage appears in git's output when a checkout has outlived
	// its worktree metadata.
	notARepositoryMessage = "not a git repository"

	// manifestTmpSuffix names WriteManifest's staging file, renamed over the
	// manifest so its writes are atomic.
	manifestTmpSuffix = ".tmp"
)

// Git subcommands, in the order the lifecycle uses them.
const (
	cloneCmd    = "clone"
	fetchCmd    = "fetch"
	worktreeCmd = "worktree"
	revParseCmd = "rev-parse"

	configCmd  = "config"
	showRefCmd = "show-ref"

	worktreeAdd    = "add"
	worktreeRemove = "remove"
	worktreePrune  = "prune"

	bareFlag     = "--bare"
	detachFlag   = "--detach"
	forceFlag    = "--force"
	verifyFlag   = "--verify"
	branchFlag   = "-b"
	quietFlag    = "-q"
	pruneFlag    = "--prune"
	tagsFlag     = "--tags"
	porcelainArg = "--porcelain"
	statusCmd    = "status"

	quietLongFlag = "--quiet"

	// fetchRefspecKey and fetchRefspec map origin's heads to remote-tracking refs
	// only. A literal --mirror refspec (+refs/*:refs/*) would let --prune delete
	// session branches under refs/heads/*.
	fetchRefspecKey = "remote.origin.fetch"
	fetchRefspec    = "+refs/heads/*:refs/remotes/origin/*"

	// localBranchRef addresses a branch qrouton created in the mirror.
	localBranchRef = "refs/heads/"
)

// Progress messages printed while assembling a session. Mirroring and fetching
// are slow enough that silence reads as a hang.
const (
	mirroringFormat   = "qrouton: mirroring %s/%s (first use, may take a while)…\n"
	fetchingFormat    = "qrouton: fetching %s/%s…\n"
	checkingOutFormat = "qrouton: checking out %s on %s…\n"
)

// Scratch sessions (a bare `qrouton`) are named after the invoking directory
// plus entropy, falling back when the basename slugifies to nothing.
const (
	scratchFallbackName = "scratch"
	scratchEntropyBytes = 2 // 4 hex characters
)

// The durable-artifact directories every session starts with, under
// thoughts/shared — a symlink into <root>/thoughts/<slug>/shared, so documents
// outlive the session directory. Status reads the first two to infer workflow
// state.
const (
	scaffoldResearch = "research"
	scaffoldPlans    = "plans"
	scaffoldSpecs    = "specs"
)

var scaffoldDirs = []string{scaffoldResearch, scaffoldPlans, scaffoldSpecs}

// Markers the workflow-status reader looks for in a plan document: a plan whose
// checkboxes are all ticked counts as implemented.
const (
	questionsMarker = "question"
	checkedBox      = "- [x]"
	uncheckedBox    = "- [ ]"

	markdownGlob = "*.md"
)
