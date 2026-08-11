package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// assemblingMarker is written first during assembly and removed after the
// manifest lands, so a directory it survives in was abandoned mid-assembly
// (e.g. the process was killed) and is safe to reclaim.
const assemblingMarker = ".qrouton-assembling"

// Abandoned reports whether dir is a half-assembled session directory left
// behind by an interrupted run: it carries the assembly marker but no manifest.
func Abandoned(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, manifestName)); err == nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, assemblingMarker))
	return err == nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), slugSeparator), slugSeparator)
}

// ScratchName names a zero-repo scratch session after the directory qrouton
// was invoked from, plus entropy to dodge collisions: running from
// ~/Work/lifesum yields "lifesum-4f3a". A basename that slugifies to nothing
// (e.g. "/") falls back to "scratch-<hex>".
func ScratchName(cwd string) string {
	base := Slugify(filepath.Base(cwd))
	if base == "" {
		base = scratchFallbackName
	}
	return base + slugSeparator + entropySuffix()
}

func entropySuffix() string {
	b := make([]byte, scratchEntropyBytes)
	_, _ = rand.Read(b) // crypto/rand never fails
	return hex.EncodeToString(b)
}

// Create is the role-aware assembly entry point. mode selects the runner's
// starting system prompt. Progress reports the start and outcome of each real
// mirror, worktree, scaffold, and manifest operation.
func Create(cfg *config.Config, name, desc, ticket, prefix string, mode SessionMode, repos []RepoSelection, progress ProgressFunc) (string, error) {
	slug := Slugify(name)
	dir := filepath.Join(cfg.Root, slug)
	if err := os.Mkdir(dir, dirMode); err != nil {
		// Reclaim only directories our own interrupted assembly left behind —
		// never a user's directory that merely shares the slug.
		if !os.IsExist(err) || !Abandoned(dir) {
			return "", err
		}
		if err := os.RemoveAll(dir); err != nil {
			return "", err
		}
		if err := os.Mkdir(dir, dirMode); err != nil {
			return "", err
		}
	}
	manifestComplete := false
	defer func() {
		if !manifestComplete {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, assemblingMarker), nil, fileMode); err != nil {
		return "", err
	}

	m := Manifest{SchemaVersion: manifestSchemaVersion, Name: name, Slug: slug, Description: desc,
		TicketURL: ticket, Mode: mode.effective(), CreatedAt: time.Now()}
	var err error
	if m, err = ComposeRepos(cfg, m, repos, prefix+branchSeparator+slug, progress); err != nil {
		return "", err
	}

	// The durable-artifact scaffold the RPI workflow writes into. Documents
	// live under <root>/thoughts/<slug> and the session reaches them through
	// a relative symlink, so Delete's RemoveAll (which does not follow links)
	// keeps them when the session directory goes.
	emitProgress(progress, Progress{Step: ProgressScaffold, Status: ProgressStarted})
	home := thoughtsHome(cfg.Root, slug)
	for _, d := range scaffoldDirs {
		if err := os.MkdirAll(filepath.Join(home, sessionpaths.SharedDirName, d), dirMode); err != nil {
			emitProgress(progress, Progress{Step: ProgressScaffold, Status: ProgressFailed, Err: err})
			return "", err
		}
	}
	if err := os.Symlink(filepath.Join("..", sessionpaths.ThoughtsDirName, slug),
		filepath.Join(dir, sessionpaths.ThoughtsDirName)); err != nil {
		emitProgress(progress, Progress{Step: ProgressScaffold, Status: ProgressFailed, Err: err})
		return "", err
	}
	emitProgress(progress, Progress{Step: ProgressScaffold, Status: ProgressCompleted})

	emitProgress(progress, Progress{Step: ProgressManifest, Status: ProgressStarted})
	if err := WriteManifest(dir, m); err != nil {
		emitProgress(progress, Progress{Step: ProgressManifest, Status: ProgressFailed, Err: err})
		return "", err
	}
	manifestComplete = true
	_ = os.Remove(filepath.Join(dir, assemblingMarker))
	emitProgress(progress, Progress{Step: ProgressManifest, Status: ProgressCompleted})
	return dir, nil
}

// thoughtsHome is where a session's documents actually live: under the root,
// outside the session directory, so deleting the session keeps them.
func thoughtsHome(root, slug string) string {
	return filepath.Join(root, sessionpaths.ThoughtsDirName, slug)
}

// materialise mirrors and checks out one selected repository for the session
// at dir, returning its manifest entry — the shared per-repo body of Create
// and ComposeRepos. branch names the session branch an active repository is
// cut on; worktreePath is the checkout's location relative to dir.
func materialise(cfg *config.Config, dir string, sel RepoSelection, branch, worktreePath string, progress ProgressFunc) (ManifestRepo, error) {
	r := sel.Repo
	role := sel.Role
	if role == "" {
		role = RepoRoleActive
	}
	if role != RepoRoleActive && role != RepoRoleReference {
		return ManifestRepo{}, invalidRole(role, r.Org, r.Name)
	}
	url := sshURL(r.Org, r)
	emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressStarted, Repo: &r, Role: role})
	// git's clone/fetch progress, forwarded per repository so several
	// assembling at once each draw their own bar instead of interleaving lines.
	var onProgress func(string, int)
	if progress != nil {
		onProgress = func(phase string, percent int) {
			progress(Progress{Step: ProgressMirror, Status: ProgressAdvanced, Repo: &r, Role: role,
				Phase: phase, Percent: percent})
		}
	}
	if err := ensureMirror(cfg.Root, r.Org, r.Name, url, onProgress); err != nil {
		emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
		return ManifestRepo{}, err
	}
	emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressCompleted, Repo: &r, Role: role})
	mr := ManifestRepo{Name: r.Name, Org: r.Org, Role: role,
		DefaultBranch: r.DefaultBranch, WorktreePath: worktreePath, SSHURL: url}
	mirror := mirrorPath(cfg.Root, r.Org, r.Name)
	emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressStarted, Repo: &r, Role: role})
	if role == RepoRoleReference {
		revision, err := resolveRevision(mirror, remoteRefPrefix+r.DefaultBranch)
		if err != nil {
			emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
			return ManifestRepo{}, err
		}
		mr.Revision = revision
		if err := addDetachedWorktree(mirror, filepath.Join(dir, worktreePath), revision); err != nil {
			emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
			return ManifestRepo{}, err
		}
	} else {
		mr.Branch = branch
		if err := addWorktree(mirror, filepath.Join(dir, worktreePath), branch,
			remoteRefPrefix+r.DefaultBranch); err != nil {
			emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
			return ManifestRepo{}, err
		}
	}
	emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressCompleted, Repo: &r, Role: role})
	return mr, nil
}

// ComposeRepos materialises the selected repositories into m — mirrors,
// worktrees, manifest entries — without writing the manifest, so a caller can
// fold repos, mode, and an escalation outcome into one atomic write. branch is
// the session branch active repositories are cut on. A repository sharing its
// name with another (in the batch, or already in m) gets an org-qualified
// worktree path.
func ComposeRepos(cfg *config.Config, m Manifest, sels []RepoSelection, branch string, progress ProgressFunc) (Manifest, error) {
	dir := filepath.Join(cfg.Root, m.Slug)
	if err := os.MkdirAll(sessionpaths.Src(dir), dirMode); err != nil {
		return m, err
	}
	nameCounts := make(map[string]int, len(m.Repos)+len(sels))
	for _, r := range m.Repos {
		nameCounts[r.Name]++
	}
	for _, sel := range sels {
		nameCounts[sel.Repo.Name]++
	}
	for _, sel := range sels {
		// A repository already in the session is left exactly as it stands —
		// same worktree, same branch, same uncommitted work. Escalating a
		// session that has been worked in must not disturb what it was working
		// on, and the collision handling below cannot tell a repository from
		// itself: it counts names, so it would dutifully org-qualify a second
		// checkout of the same repo and clone it alongside the first, on a
		// different branch.
		if hasRepo(m.Repos, sel.Repo.Org, sel.Repo.Name) {
			continue
		}
		worktreePath := filepath.Join(sessionpaths.SrcDirName, sel.Repo.Name)
		if nameCounts[sel.Repo.Name] > 1 {
			worktreePath = filepath.Join(sessionpaths.SrcDirName, Slugify(sel.Repo.Org+slugSeparator+sel.Repo.Name))
		}
		mr, err := materialise(cfg, dir, sel, branch, worktreePath, progress)
		if err != nil {
			return m, err
		}
		m.Repos = append(m.Repos, mr)
	}
	return m, nil
}

// MergeRepos appends repositories missing from a freshly loaded manifest.
func MergeRepos(m Manifest, repos []ManifestRepo) Manifest {
	for _, repo := range repos {
		if !hasRepo(m.Repos, repo.Org, repo.Name) {
			m.Repos = append(m.Repos, repo)
		}
	}
	return m
}

// hasRepo reports whether the session already holds this repository. Identity is
// owner and name together, case-insensitively — the same reckoning the ad-hoc
// path uses when it dedupes command-line arguments.
func hasRepo(repos []ManifestRepo, org, name string) bool {
	for _, r := range repos {
		if strings.EqualFold(r.Org, org) && strings.EqualFold(r.Name, name) {
			return true
		}
	}
	return false
}

func emitProgress(progress ProgressFunc, event Progress) {
	if progress != nil {
		progress(event)
	}
}

// EnsureWorktrees re-materialises any pruned worktrees on resume (fresh fetch
// either way). progress reports the fetch — and, if a mirror has been deleted,
// a full re-clone — so a slow resume is not silent.
func EnsureWorktrees(cfg *config.Config, m Manifest, progress ProgressFunc) error {
	dir := filepath.Join(cfg.Root, m.Slug)
	for _, r := range m.Repos {
		if r.SSHURL == "" {
			return fmt.Errorf("%s: %w: %s/%s", manifestName, ErrNoCloneURL, r.Org, r.Name)
		}
		repo := github.Repo{Name: r.Name, Org: r.Org, DefaultBranch: r.DefaultBranch, SSHURL: r.SSHURL}
		var onProgress func(string, int)
		if progress != nil {
			onProgress = func(phase string, percent int) {
				progress(Progress{Step: ProgressMirror, Status: ProgressAdvanced, Repo: &repo,
					Role: r.Role, Phase: phase, Percent: percent})
			}
		}
		emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressStarted, Repo: &repo, Role: r.Role})
		if err := ensureMirror(cfg.Root, r.Org, r.Name, r.SSHURL, onProgress); err != nil {
			emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressFailed, Repo: &repo, Role: r.Role, Err: err})
			return err
		}
		emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressCompleted, Repo: &repo, Role: r.Role})
		wt := filepath.Join(dir, r.WorktreePath)
		if _, err := os.Stat(wt); err == nil {
			continue
		}
		mirror := mirrorPath(cfg.Root, r.Org, r.Name)
		var err error
		if r.Role == RepoRoleReference {
			if r.Revision == "" {
				return fmt.Errorf("%w: %s/%s", ErrNoPinnedRevision, r.Org, r.Name)
			}
			err = addDetachedWorktree(mirror, wt, r.Revision)
		} else {
			err = addWorktree(mirror, wt, r.Branch, remoteRefPrefix+r.DefaultBranch)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveRevision(mirror, ref string) (string, error) {
	out, err := exec.Command(gitBin, dirFlag, mirror, revParseCmd, verifyFlag, ref+commitRefSuffix).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w\n%s", ref, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func addDetachedWorktree(mirror, path, revision string) error {
	if err := git(dirFlag, mirror, worktreeCmd, worktreePrune); err != nil {
		return err
	}
	return git(dirFlag, mirror, worktreeCmd, worktreeAdd, detachFlag, path, revision)
}

func sshURL(org string, r github.Repo) string {
	if r.SSHURL != "" {
		return r.SSHURL
	}
	return fmt.Sprintf(sshURLFormat, org, r.Name)
}
