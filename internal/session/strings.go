package session

// Git plumbing and path fragments the session lifecycle depends on, plus the
// scaffold a fresh session starts with.

import "time"

const (
	// slugSeparator joins and pads slug components; branchSeparator divides a
	// branch prefix from its slug.
	slugSeparator   = "-"
	branchSeparator = "/"

	dirMode         = 0o755
	fileMode        = 0o644
	privateFileMode = 0o600

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

	// tmpSuffix ends the name of a staging file, which is created uniquely so two
	// processes staging at once cannot share one.
	tmpSuffix = ".tmp"
)

// Git subcommands, in the order the lifecycle uses them.
const (
	cloneCmd    = "clone"
	fetchCmd    = "fetch"
	worktreeCmd = "worktree"
	revParseCmd = "rev-parse"

	configCmd      = "config"
	showRefCmd     = "show-ref"
	checkoutCmd    = "checkout"
	symbolicRefCmd = "symbolic-ref"

	worktreeAdd    = "add"
	worktreeRemove = "remove"
	worktreePrune  = "prune"

	bareFlag     = "--bare"
	detachFlag   = "--detach"
	forceFlag    = "--force"
	verifyFlag   = "--verify"
	branchFlag   = "-b"
	quietFlag    = "-q"
	shortFlag    = "--short"
	pruneFlag    = "--prune"
	tagsFlag     = "--tags"
	porcelainArg = "--porcelain"
	statusCmd    = "status"

	quietLongFlag = "--quiet"

	// Measuring a session branch against the branch it was cut from. Two dots
	// asks what one ref has that the other does not; three asks the same since
	// their merge base, so a base branch that has moved on is not counted as
	// the session's work.
	revListCmd         = "rev-list"
	diffCmd            = "diff"
	countFlag          = "--count"
	numstatFlag        = "--numstat"
	headRef            = "HEAD"
	rangeSeparator     = ".."
	mergeBaseSeparator = "..."

	// binaryMarker is --numstat's placeholder for a binary file's line counts.
	binaryMarker = "-"

	// progressFlag is not optional once stderr is a pipe: git reports progress
	// only to a terminal unless asked in so many words.
	progressFlag = "--progress"

	// fetchRefspecKey and fetchRefspec map origin's heads to remote-tracking refs
	// only. A literal --mirror refspec (+refs/*:refs/*) would let --prune delete
	// session branches under refs/heads/*.
	fetchRefspecKey = "remote.origin.fetch"
	fetchRefspec    = "+refs/heads/*:refs/remotes/origin/*"

	// localBranchRef addresses a branch qrouton created in the mirror.
	localBranchRef = "refs/heads/"
)

// Reading git's progress off a pipe. Nothing is printed from here any more:
// mirroring and fetching are slow enough that silence reads as a hang, so they
// report through Progress events and the caller draws them.
const (
	// progressChunkBytes is one read of git's stderr. Its updates are a few
	// dozen bytes each, so this holds several and the newest is the one drawn.
	progressChunkBytes = 4096

	// progressTailLimit caps what is retained to explain a failure — enough for
	// a host-key or auth message, not a whole clone's chatter.
	progressTailLimit = 4096

	// progressEmitInterval rate-limits reports to something an eye can follow;
	// a small repository otherwise produces a few hundred in a fraction of a
	// second. Phase changes and 100% ignore it.
	progressEmitInterval = 100 * time.Millisecond
	progressComplete     = 100
)

// repoStatTimeout bounds one git invocation inside RepoStats, so a wedged git
// cannot hang the caller.
const repoStatTimeout = 5 * time.Second

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
