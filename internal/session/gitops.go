package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func git(args ...string) error {
	out, err := exec.Command(gitBin, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

func gitOK(args ...string) bool { return exec.Command(gitBin, args...).Run() == nil }

// gitLoud runs git wired to the terminal — for clone/fetch, which are slow (progress must be
// visible, not a blank screen) and may prompt (ssh passphrase / host key need stdin).
func gitLoud(args ...string) error {
	cmd := exec.Command(gitBin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w", args, err)
	}
	return nil
}

func mirrorPath(root, org, repo string) string {
	return filepath.Join(root, mirrorsDirName, org, repo+gitDirSuffix)
}

// ensureMirror clones a bare mirror on first use, otherwise fetches. The fetch refspec maps origin's
// heads to refs/remotes/origin/* only, so --prune can never touch session branches under refs/heads/*.
// (A literal --mirror clone's +refs/*:refs/* refspec would prune them — the hazard mirror_test guards.)
func ensureMirror(root, org, repo, url string) error {
	mp := mirrorPath(root, org, repo)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		fmt.Printf(mirroringFormat, org, repo)
		if err := gitLoud(cloneCmd, bareFlag, url, mp); err != nil {
			return err
		}
	}
	// Set the remote-tracking-only refspec on every call, not just right after clone: a bare
	// clone configures no fetch refspec, so an interrupted first run (or a failed config step)
	// would otherwise leave a mirror that never populates refs/remotes/origin/* — worktree
	// creation then fails on every resume and never self-heals. This is idempotent.
	if err := git(dirFlag, mp, configCmd, fetchRefspecKey, fetchRefspec); err != nil {
		return err
	}
	fmt.Printf(fetchingFormat, org, repo)
	return gitLoud(dirFlag, mp, fetchCmd, pruneFlag, remoteName)
}

// addWorktree materialises a worktree at path: on the existing session branch if it exists
// (resume after prune), otherwise on a new branch off the freshly-fetched origin default branch.
func addWorktree(mirror, path, branch, startRef string) error {
	fmt.Printf(checkingOutFormat, filepath.Base(path), branch)
	if err := git(dirFlag, mirror, worktreeCmd, worktreePrune); err != nil {
		return err
	}
	if gitOK(dirFlag, mirror, showRefCmd, verifyFlag, quietLongFlag, localBranchRef+branch) {
		return git(dirFlag, mirror, worktreeCmd, worktreeAdd, path, branch)
	}
	return git(dirFlag, mirror, worktreeCmd, worktreeAdd, branchFlag, branch, path, startRef)
}
