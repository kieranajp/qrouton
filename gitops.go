package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func git(args ...string) error {
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return nil
}

func gitOK(args ...string) bool { return exec.Command("git", args...).Run() == nil }

// gitLoud runs git wired to the terminal — for clone/fetch, which are slow (progress must be
// visible, not a blank screen) and may prompt (ssh passphrase / host key need stdin).
func gitLoud(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w", args, err)
	}
	return nil
}

func mirrorPath(root, org, repo string) string {
	return filepath.Join(root, ".mirrors", org, repo+".git")
}

// ensureMirror clones a bare mirror on first use, otherwise fetches. The fetch refspec maps origin's
// heads to refs/remotes/origin/* only, so --prune can never touch session branches under refs/heads/*.
// (A literal --mirror clone's +refs/*:refs/* refspec would prune them — the hazard mirror_test guards.)
func ensureMirror(root, org, repo, url string) error {
	mp := mirrorPath(root, org, repo)
	if _, err := os.Stat(mp); os.IsNotExist(err) {
		fmt.Printf("qrouton: mirroring %s/%s (first use, may take a while)…\n", org, repo)
		if err := gitLoud("clone", "--bare", url, mp); err != nil {
			return err
		}
		if err := git("-C", mp, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
			return err
		}
	}
	fmt.Printf("qrouton: fetching %s/%s…\n", org, repo)
	return gitLoud("-C", mp, "fetch", "--prune", "origin")
}

// addWorktree materialises a worktree at path: on the existing session branch if it exists
// (resume after prune), otherwise on a new branch off the freshly-fetched origin default branch.
func addWorktree(mirror, path, branch, startRef string) error {
	fmt.Printf("qrouton: checking out %s on %s…\n", filepath.Base(path), branch)
	if err := git("-C", mirror, "worktree", "prune"); err != nil {
		return err
	}
	if gitOK("-C", mirror, "show-ref", "--verify", "--quiet", "refs/heads/"+branch) {
		return git("-C", mirror, "worktree", "add", path, branch)
	}
	return git("-C", mirror, "worktree", "add", "-b", branch, path, startRef)
}
