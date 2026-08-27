package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
)

// RepoRef names one repository a session already holds.
type RepoRef struct {
	Org, Name string
}

// UpgradeRepos moves the named reference checkouts onto the session branch, cut
// from each repository's own remote default branch as every other editing
// repository's is — so a repo taken up shares its siblings' base rather than a
// revision only it was pinned to. Every ref is checked before any is touched, and
// the manifest is left to the caller to rewrite once these are on disk.
func UpgradeRepos(cfg *config.Config, m Manifest, refs []RepoRef, branch string, progress ProgressFunc) error {
	if len(refs) == 0 {
		return nil
	}
	dir := filepath.Join(cfg.Root, m.Slug)
	pending := make([]ManifestRepo, 0, len(refs))
	for _, ref := range refs {
		r, todo, err := upgradable(m, dir, ref, branch)
		if err != nil {
			return err
		}
		if todo {
			pending = append(pending, r)
		}
	}
	for _, r := range pending {
		if err := upgradeRepo(cfg, dir, r, branch, progress); err != nil {
			return err
		}
	}
	return nil
}

// ApplyUpgrades rewrites the entries taken up: editing, on the session branch, and
// no longer pinned. It touches no disk, so it can run against a manifest freshly
// loaded after the checkouts have moved.
func ApplyUpgrades(m Manifest, refs []RepoRef, branch string) (Manifest, error) {
	if len(refs) == 0 {
		return m, nil
	}
	m.Repos = slices.Clone(m.Repos)
	for _, ref := range refs {
		i := indexOfRepo(m.Repos, ref.Org, ref.Name)
		if i < 0 {
			return m, refuseUpgrade(ErrNotHeld, ref.Org, ref.Name)
		}
		m.Repos[i].Role, m.Repos[i].Branch, m.Repos[i].Revision = RepoRoleEditing, branch, ""
	}
	return m, nil
}

// upgradable answers whether ref can be taken up, and whether there is anything
// left to do. A checkout already on the branch is an upgrade whose manifest write
// never landed, which the next attempt has to be able to finish.
func upgradable(m Manifest, dir string, ref RepoRef, branch string) (ManifestRepo, bool, error) {
	i := indexOfRepo(m.Repos, ref.Org, ref.Name)
	if i < 0 {
		return ManifestRepo{}, false, refuseUpgrade(ErrNotHeld, ref.Org, ref.Name)
	}
	r := m.Repos[i]
	if r.Role.Effective() != RepoRoleReference {
		return r, false, refuseUpgrade(ErrNotReference, r.Org, r.Name)
	}
	if r.SSHURL == "" {
		return r, false, refuseUpgrade(ErrNoCloneURL, r.Org, r.Name)
	}
	wt := filepath.Join(dir, r.WorktreePath)
	if _, err := os.Stat(wt); err != nil {
		return r, true, nil
	}
	on, err := currentBranch(wt)
	if err != nil {
		return r, false, err
	}
	if on == branch {
		return r, false, nil
	}
	// Commits in a detached checkout exist nowhere else, and a branch cut from the
	// default branch's tip would leave them unreachable.
	if r.Revision == "" {
		return r, false, refuseUpgrade(ErrNoPinnedRevision, r.Org, r.Name)
	}
	head, err := resolveRevision(wt, headRef)
	if err != nil {
		return r, false, err
	}
	if head != r.Revision {
		return r, false, refuseUpgrade(ErrReferenceMoved, r.Org, r.Name)
	}
	return r, true, nil
}

func upgradeRepo(cfg *config.Config, dir string, r ManifestRepo, branch string, progress ProgressFunc) error {
	repo := github.Repo{Name: r.Name, Org: r.Org, DefaultBranch: r.DefaultBranch, SSHURL: r.SSHURL}
	var onProgress func(string, int)
	if progress != nil {
		onProgress = func(phase string, percent int) {
			progress(Progress{Step: ProgressMirror, Status: ProgressAdvanced, Repo: &repo,
				Role: RepoRoleEditing, Phase: phase, Percent: percent})
		}
	}
	// The mirror is already there; this is the fetch that brings the default
	// branch's tip within reach of the new session branch.
	emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressStarted, Repo: &repo, Role: RepoRoleEditing})
	if err := ensureMirror(cfg.Root, r.Org, r.Name, r.SSHURL, onProgress); err != nil {
		emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressFailed, Repo: &repo, Role: RepoRoleEditing, Err: err})
		return err
	}
	emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressCompleted, Repo: &repo, Role: RepoRoleEditing})

	emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressStarted, Repo: &repo, Role: RepoRoleEditing})
	mirror := mirrorPath(cfg.Root, r.Org, r.Name)
	wt := filepath.Join(dir, r.WorktreePath)
	if err := branchWorktree(mirror, wt, r, branch); err != nil {
		emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressFailed, Repo: &repo, Role: RepoRoleEditing, Err: err})
		return err
	}
	emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressCompleted, Repo: &repo, Role: RepoRoleEditing})
	return nil
}

// branchWorktree switches an existing checkout in place rather than replacing it,
// which keeps whatever the directory holds that git does not track — an .env, a
// node_modules — and refuses rather than clobbering uncommitted work.
func branchWorktree(mirror, wt string, r ManifestRepo, branch string) error {
	startRef := remoteRefPrefix + r.DefaultBranch
	if _, err := os.Stat(wt); err != nil {
		return addWorktree(mirror, wt, branch, startRef)
	}
	var err error
	if mirrorHasBranch(mirror, branch) {
		err = git(dirFlag, wt, checkoutCmd, quietFlag, branch)
	} else {
		err = git(dirFlag, wt, checkoutCmd, quietFlag, branchFlag, branch, startRef)
	}
	if err == nil {
		return nil
	}
	// The refusal a user can actually cause, said in a sentence: git's own names
	// every file it would overwrite, and the footer holds one line.
	if dirty, dirtyErr := worktreeDirty(wt); dirtyErr == nil && dirty {
		return refuseUpgrade(ErrCheckoutHasWork, r.Org, r.Name)
	}
	return err
}

// currentBranch is the branch a worktree is on, empty when detached — which
// symbolic-ref reports by exiting non-zero and saying nothing.
func currentBranch(wt string) (string, error) {
	out, err := exec.Command(gitBin, dirFlag, wt, symbolicRefCmd, shortFlag, quietFlag, headRef).CombinedOutput()
	if err != nil {
		if len(strings.TrimSpace(string(out))) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("read %s HEAD: %w\n%s", wt, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// indexOfRepo locates a repository by owner and name together, case-insensitively.
func indexOfRepo(repos []ManifestRepo, org, name string) int {
	return slices.IndexFunc(repos, func(r ManifestRepo) bool {
		return strings.EqualFold(r.Org, org) && strings.EqualFold(r.Name, name)
	})
}
