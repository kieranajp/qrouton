package session

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type WorkflowStatus struct {
	Research, Plan, Implement bool
}

// Status infers the user-facing RPI state from durable workflow documents.
func Status(root string, m Manifest) WorkflowStatus {
	dir := filepath.Join(root, m.Slug, "thoughts", "shared")
	research := markdownFiles(filepath.Join(dir, "research"))
	plans := markdownFiles(filepath.Join(dir, "plans"))
	status := WorkflowStatus{Plan: len(plans) > 0}
	for _, path := range research {
		if !strings.Contains(strings.ToLower(filepath.Base(path)), "question") {
			status.Research = true
			break
		}
	}
	for _, path := range plans {
		body, err := os.ReadFile(path)
		if err == nil {
			lower := bytes.ToLower(body)
			if bytes.Contains(lower, []byte("- [x]")) && !bytes.Contains(lower, []byte("- [ ]")) {
				status.Implement = true
				break
			}
		}
	}
	return status
}

func markdownFiles(dir string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, "*.md"))
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
		out, err := exec.Command("git", "-C", path, "status", "--porcelain").CombinedOutput()
		if err != nil {
			// A checkout can outlive its worktree metadata when a mirror was
			// manually removed or corrupted. There is no useful dirty-state check
			// left to perform, but the session must still remain deletable.
			if strings.Contains(string(out), "not a git repository") {
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
		if err := git("-C", mirror, "worktree", "remove", "--force", path); err != nil {
			// Broken or missing worktree metadata should not strand a session.
			// The user has already confirmed destructive deletion at this point.
			if removeErr := os.RemoveAll(path); removeErr != nil {
				return fmt.Errorf("remove %s/%s worktree after git cleanup failed: %w", repo.Org, repo.Name, removeErr)
			}
			_ = git("-C", mirror, "worktree", "prune")
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove session %q: %w", m.Slug, err)
	}
	return nil
}
