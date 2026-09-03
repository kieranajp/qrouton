package session

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type WorkflowStatus struct {
	Research, Plan, Implement bool
}

// Status infers the user-facing RPI state from durable workflow documents.
func Status(root string, m Manifest) WorkflowStatus {
	dir := sessionpaths.Thoughts(filepath.Join(root, m.Slug))
	research := markdownFiles(filepath.Join(dir, scaffoldResearch))
	plans := markdownFiles(filepath.Join(dir, scaffoldPlans))
	status := WorkflowStatus{Plan: len(plans) > 0}
	for _, path := range research {
		if researched(path) {
			status.Research = true
			break
		}
	}
	for _, path := range plans {
		body, err := os.ReadFile(path)
		if err == nil {
			lower := bytes.ToLower(body)
			if bytes.Contains(lower, []byte(checkedBox)) && !bytes.Contains(lower, []byte(uncheckedBox)) {
				status.Implement = true
				break
			}
		}
	}
	return status
}

// researched requires answered sections, or non-empty prose when there are no sections.
func researched(path string) bool {
	if strings.HasSuffix(strings.ToLower(filepath.Base(path)), legacyQuestionsSuffix) {
		return false
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	sections := markdown.Sections(string(body))
	for _, section := range sections {
		if section.State == markdown.SectionAnswered && !strings.EqualFold(section.Name, summaryHeading) {
			return true
		}
	}
	return len(sections) == 0 && strings.TrimSpace(markdown.Body(string(body))) != ""
}

func markdownFiles(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, markdownGlob))
	return files
}

func DirtyWorktrees(root string, m Manifest) ([]string, error) {
	var dirty []string
	for _, repo := range m.Repos {
		path := filepath.Join(root, m.Slug, repo.WorktreePath)
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
	out, err := exec.Command(gitBin, dirFlag, path, statusCmd, porcelainArg).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), notARepositoryMessage) {
			return false, nil
		}
		return false, fmt.Errorf("%w\n%s", err, out)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

func Delete(root string, m Manifest) error {
	dir := filepath.Join(root, m.Slug)
	for _, repo := range m.Repos {
		path := filepath.Join(dir, repo.WorktreePath)
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
	if _, err := os.Stat(filepath.Join(dir, manifestName)); err != nil {
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

// RepoStat is how far one repository has moved since it was branched. Pushed
// means its remote-tracking session branch contains work beyond the base.
type RepoStat struct {
	Org, Name  string
	Role       RepoRole
	Path       string
	Commits    int
	Insertions int
	Deletions  int
	Measured   bool
	Pushed     bool
}

// RepoStats measures each repository against the branch it was cut from. A
// reference repository is pinned, so it has nothing to be ahead of, and a
// blank default branch is left unmeasured rather than built into a bad ref.
func RepoStats(ctx context.Context, root string, m Manifest) []RepoStat {
	dir := filepath.Join(root, m.Slug)
	stats := make([]RepoStat, 0, len(m.Repos))
	for _, repo := range m.Repos {
		stat := RepoStat{
			Org: repo.Org, Name: repo.Name, Role: repo.Role.Effective(),
			Path: filepath.Join(dir, repo.WorktreePath),
		}
		if stat.Role == RepoRoleEditing && repo.DefaultBranch != "" {
			measure(ctx, stat.Path, remoteRefPrefix+repo.DefaultBranch,
				remoteRefPrefix+repo.Branch, &stat)
		}
		stats = append(stats, stat)
	}
	return stats
}

func measure(ctx context.Context, path, base, branch string, stat *RepoStat) {
	commits, ok := countCommits(ctx, path, base)
	if !ok {
		return
	}
	insertions, deletions, ok := countLines(ctx, path, base)
	if !ok {
		return
	}
	stat.Commits, stat.Insertions, stat.Deletions, stat.Measured = commits, insertions, deletions, true
	stat.Pushed = branch != remoteRefPrefix && countPushedCommits(ctx, path, base, branch) > 0
}

func countCommits(ctx context.Context, path, base string) (int, bool) {
	return countRange(ctx, path, base+rangeSeparator+headRef)
}

func countRange(ctx context.Context, path, revisionRange string) (int, bool) {
	ctx, cancel := context.WithTimeout(ctx, repoStatTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, gitBin, dirFlag, path, revListCmd, countFlag, revisionRange).Output()
	if err != nil {
		return 0, false
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	return count, err == nil
}

func countPushedCommits(ctx context.Context, path, base, branch string) int {
	commits, ok := countRange(ctx, path, base+rangeSeparator+branch)
	if !ok {
		return 0
	}
	return commits
}

// countLines totals the diff since the merge base, so a base branch that has
// moved on does not count as the session's own work.
func countLines(ctx context.Context, path, base string) (int, int, bool) {
	ctx, cancel := context.WithTimeout(ctx, repoStatTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, gitBin, dirFlag, path, diffCmd, numstatFlag, base+mergeBaseSeparator+headRef).Output()
	if err != nil {
		return 0, 0, false
	}
	return sumNumstat(string(out))
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
