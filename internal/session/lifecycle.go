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
		if !strings.Contains(strings.ToLower(filepath.Base(path)), questionsMarker) {
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

func markdownFiles(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, markdownGlob))
	return files
}

// DirtyWorktrees returns repositories with staged, unstaged, or untracked files.
func DirtyWorktrees(root string, m Manifest) ([]string, error) {
	var dirty []string
	for _, repo := range m.Repos {
		path := filepath.Join(root, m.Slug, repo.WorktreePath)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		out, err := exec.Command(gitBin, dirFlag, path, statusCmd, porcelainArg).CombinedOutput()
		if err != nil {
			// A checkout can outlive its worktree metadata when a mirror was
			// manually removed or corrupted. There is no useful dirty-state check
			// left to perform, but the session must still remain deletable.
			if strings.Contains(string(out), notARepositoryMessage) {
				continue
			}
			return nil, fmt.Errorf("check %s/%s for changes: %w\n%s", repo.Org, repo.Name, err, out)
		}
		if len(bytes.TrimSpace(out)) > 0 {
			dirty = append(dirty, repo.Org+"/"+repo.Name)
		}
	}
	return dirty, nil
}

// Delete removes registered worktrees and then the remaining session files.
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

// RepoStat is how far one repository has moved since it was branched.
// Uncommitted work is left out; the commit and diff figures answer the question.
type RepoStat struct {
	Org, Name  string
	Role       RepoRole
	Commits    int
	Insertions int
	Deletions  int
	Measured   bool
}

// RepoStats measures each repository against the branch it was cut from. A
// reference repository is pinned, so it has nothing to be ahead of, and a
// blank default branch is left unmeasured rather than built into a bad ref.
func RepoStats(ctx context.Context, root string, m Manifest) []RepoStat {
	dir := filepath.Join(root, m.Slug)
	stats := make([]RepoStat, 0, len(m.Repos))
	for _, repo := range m.Repos {
		stat := RepoStat{Org: repo.Org, Name: repo.Name, Role: repo.Role.Effective()}
		if stat.Role == RepoRoleEditing && repo.DefaultBranch != "" {
			base := remoteRefPrefix + repo.DefaultBranch
			path := filepath.Join(dir, repo.WorktreePath)
			measure(ctx, path, base, &stat)
		}
		stats = append(stats, stat)
	}
	return stats
}

func measure(ctx context.Context, path, base string, stat *RepoStat) {
	commits, ok := countCommits(ctx, path, base)
	if !ok {
		return
	}
	insertions, deletions, ok := countLines(ctx, path, base)
	if !ok {
		return
	}
	stat.Commits, stat.Insertions, stat.Deletions, stat.Measured = commits, insertions, deletions, true
}

// countCommits counts what the session branch has that its base does not.
func countCommits(ctx context.Context, path, base string) (int, bool) {
	ctx, cancel := context.WithTimeout(ctx, repoStatTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, gitBin, dirFlag, path, revListCmd, countFlag, base+rangeSeparator+headRef).Output()
	if err != nil {
		return 0, false
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	return count, err == nil
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
