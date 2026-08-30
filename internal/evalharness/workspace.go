package evalharness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func initializeRepositories(workspace string) (map[string]string, error) {
	root := filepath.Join(workspace, srcDirName)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	baselines := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repo := filepath.Join(root, entry.Name())
		commands := [][]string{
			{gitBin, gitInitCmd, gitQuietFlag},
			{gitBin, gitConfigCmd, gitUserEmailKey, fixtureUserEmail},
			{gitBin, gitConfigCmd, gitUserNameKey, fixtureUserName},
			{gitBin, gitAddCmd, gitAllPathspec},
			{gitBin, gitCommitCmd, gitQuietMsgFlag, fixtureCommitMessage},
		}
		for _, command := range commands {
			cmd := exec.Command(command[0], command[1:]...)
			cmd.Dir = repo
			if output, err := cmd.CombinedOutput(); err != nil {
				return nil, fmt.Errorf("%s in %s: %w: %s", strings.Join(command, " "), repo, err, output)
			}
		}
		head, err := commandOutput(context.Background(), repo, gitBin, gitRevParseCmd, gitHeadRef)
		if err != nil {
			return nil, fmt.Errorf("resolve baseline in %s: %w: %s", repo, err, head)
		}
		baselines[entry.Name()] = head
	}
	return baselines, nil
}

func collectArtifacts(workspace string) ([]Artifact, error) {
	thoughts := filepath.Join(workspace, thoughtsDirName, sharedDirName)
	var artifacts []Artifact
	err := filepath.WalkDir(thoughts, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		artifacts = append(artifacts, Artifact{
			Path:   filepath.ToSlash(rel),
			SHA256: hex.EncodeToString(digest[:]),
			Text:   string(content),
		})
		return nil
	})
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Path < artifacts[j].Path
	})
	return artifacts, err
}

// collectDiffs deliberately ignores the case context: it runs after the turns,
// where the per-case timeout may already have expired, and a timed-out case is
// exactly the one whose diffs the report and judges must still see.
func collectDiffs(workspace string, baselines map[string]string) map[string]string {
	diffs := make(map[string]string, len(baselines))
	for repo := range baselines {
		repoDir := repoDir(workspace, repo)
		ctx := context.Background()
		diff, err := commandOutput(ctx, repoDir, gitBin, gitDiffCmd, gitNoExtDiffFlag, gitHeadRef)
		if err != nil {
			// Fail loud: an error string trips repo_unchanged instead of
			// letting a broken diff pass as "no changes".
			diff = fmt.Sprintf(diffFailedFormat, err, diff)
		}
		diffs[repo] = diff
		untracked, err := commandOutput(ctx, repoDir, gitBin, gitLsFilesCmd, gitOthersFlag, gitExcludeStdFlag)
		if err == nil && untracked != "" {
			diffs[repo] += fmt.Sprintf(untrackedFormat, untracked)
		}
	}
	return diffs
}

func commandOutput(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func makeTreeReadOnly(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o755)
		}
		return os.Chmod(path, 0o444)
	})
}
