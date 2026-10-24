package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func DirtyWorktrees(root string, m Manifest) ([]string, error) {
	var dirty []string
	for _, repo := range m.Repos {
		path := repo.Dir(filepath.Join(root, m.Slug))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		changed, err := worktreeDirty(path)
		if err != nil {
			return nil, fmt.Errorf("check %s/%s for changes: %w", repo.Org, repo.Name, err)
		}
		if changed {
			dirty = append(dirty, repo.Org+"/"+repo.Name)
		}
	}
	return dirty, nil
}

// A checkout can outlive its worktree metadata when a mirror was manually
// removed or corrupted; there is no dirty state left to read then, and a
// session carrying one must still be deletable.
func worktreeDirty(path string) (bool, error) {
	out, err := gitOutput(context.Background(), dirFlag, path, statusCmd, porcelainArg)
	if err != nil {
		if outlivedCheckout(err.Error()) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func outlivedCheckout(output string) bool {
	return slices.ContainsFunc(outlivedCheckoutMessages, func(message string) bool {
		return strings.Contains(output, message)
	})
}

func Delete(root string, m Manifest) error {
	if err := m.validate(); err != nil {
		return err
	}
	dir := filepath.Join(root, m.Slug)
	for _, repo := range m.Repos {
		path := repo.Dir(dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		mirror := mirrorPath(root, repo.Org, repo.Name)
		if err := git(dirFlag, mirror, worktreeCmd, worktreeRemove, forceFlag, path); err != nil {
			// Broken or missing worktree metadata should not strand a session.
			// The user has already confirmed destructive deletion at this point.
			if removeErr := os.RemoveAll(path); removeErr != nil {
				return fmt.Errorf("remove %s/%s worktree after git cleanup failed: %w", repo.Org, repo.Name, removeErr)
			}
			_ = git(dirFlag, mirror, worktreeCmd, worktreePrune)
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove session %q: %w", m.Slug, err)
	}
	return nil
}

// Resumable is the directory slug names under the sessions root, and empty when
// nothing there can be resumed: the manifest is written last, so a directory
// without one is not a session — and a row naming one can outlive it.
func Resumable(root, slug string) string {
	if root == "" || slug == "" {
		return ""
	}
	dir := filepath.Join(root, slug)
	if _, err := os.Stat(sessionpaths.Manifest(dir)); err != nil {
		return ""
	}
	return dir
}

// Uncommitted names the repositories a removal of the session in dir would take
// changes from.
func Uncommitted(root, dir string) ([]string, error) {
	m, err := Load(dir)
	if err != nil {
		return nil, err
	}
	return DirtyWorktrees(root, m)
}

// Remove deletes the session in dir. Delete resolves its target from the
// manifest, so a directory holding another session's manifest is refused rather
// than taking that session's worktrees.
func Remove(root, dir string) error {
	m, err := Load(dir)
	if err != nil {
		return err
	}
	if base := filepath.Base(dir); m.Slug != base {
		return mismatchedManifest(base, m.Slug)
	}
	return Delete(root, m)
}

// RepoStat is how far one repository has moved since it was branched.
type RepoStat struct {
	Org, Name  string
	Role       RepoRole
	Path       string
	Commits    int
	Insertions int
	Deletions  int
	Measured   bool
}

// RepoStats measures each repository against the branch it was cut from. A
// reference repository is pinned, so it has nothing to be ahead of, and one
// naming no branch at all is left unmeasured rather than built into a bad ref.
func RepoStats(ctx context.Context, root string, m Manifest) []RepoStat {
	dir := filepath.Join(root, m.Slug)
	stats := make([]RepoStat, 0, len(m.Repos))
	for _, repo := range m.Repos {
		stat := RepoStat{
			Org: repo.Org, Name: repo.Name, Role: repo.Role.Effective(),
			Path: repo.Dir(dir),
		}
		if stat.Role == RepoRoleEditing && (repo.BaseBranch != "" || repo.DefaultBranch != "") {
			measure(ctx, stat.Path, baseRef(repo.BaseBranch, repo.DefaultBranch), &stat)
		}
		stats = append(stats, stat)
	}
	return stats
}

func measure(ctx context.Context, path, base string, stat *RepoStat) {
	commits, err := countCommits(ctx, path, base)
	if err != nil {
		return
	}
	insertions, deletions, err := countLines(ctx, path, base)
	if err != nil {
		return
	}
	stat.Commits, stat.Insertions, stat.Deletions, stat.Measured = commits, insertions, deletions, true
}

func countCommits(ctx context.Context, path, base string) (int, error) {
	return countRange(ctx, path, base+rangeSeparator+headRef)
}

func countRange(ctx context.Context, path, revisionRange string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, repoStatTimeout)
	defer cancel()
	out, err := gitOutput(ctx, dirFlag, path, revListCmd, countFlag, revisionRange)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(out))
}

// countLines totals the diff since the merge base, so a base branch that has
// moved on does not count as the session's own work.
func countLines(ctx context.Context, path, base string) (int, int, error) {
	ctx, cancel := context.WithTimeout(ctx, repoStatTimeout)
	defer cancel()
	out, err := gitOutput(ctx, dirFlag, path, diffCmd, numstatFlag, base+mergeBaseSeparator+headRef)
	if err != nil {
		return 0, 0, err
	}
	insertions, deletions, ok := sumNumstat(out)
	if !ok {
		return 0, 0, ErrUnreadableNumstat
	}
	return insertions, deletions, nil
}

// sumNumstat totals --numstat's tab-separated columns; binaryMarker replaces
// both counts for a file git cannot diff by line.
func sumNumstat(text string) (int, int, bool) {
	insertions, deletions := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		if line == "" {
			continue
		}
		added, rest, ok := strings.Cut(line, "\t")
		if !ok {
			return 0, 0, false
		}
		removed, _, ok := strings.Cut(rest, "\t")
		if !ok {
			return 0, 0, false
		}
		if added == binaryMarker || removed == binaryMarker {
			continue
		}
		a, err := strconv.Atoi(added)
		if err != nil {
			return 0, 0, false
		}
		d, err := strconv.Atoi(removed)
		if err != nil {
			return 0, 0, false
		}
		insertions += a
		deletions += d
	}
	return insertions, deletions, true
}
