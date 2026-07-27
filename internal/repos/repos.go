package repos

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/paneui"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// Status redraws the session's per-repo branch and dirty state forever. It is
// manifest-driven, so it knows roles and exact worktree paths — a detached
// reference renders as its pinned revision instead of a blank branch.
func Status(root string) error {
	for {
		fmt.Print(paneui.Frame(statusLines(root)))
		time.Sleep(refreshInterval)
	}
}

func statusLines(root string) []string {
	lines := []string{paneui.Title(paneTitle)}
	b, err := os.ReadFile(sessionpaths.Manifest(root))
	var m session.Manifest
	if err != nil || json.Unmarshal(b, &m) != nil {
		return append(lines, paneui.Muted(noManifestLabel))
	}
	if len(m.Repos) == 0 {
		// A scratch session: point at the way repositories arrive.
		return append(lines, paneui.Muted(emptyStateLabel), paneui.Muted(emptyStateHint))
	}
	for _, r := range m.Repos {
		lines = append(lines, repoLine(root, r))
	}
	return lines
}

func repoLine(root string, r session.ManifestRepo) string {
	name := r.WorktreePath
	wt := filepath.Join(root, r.WorktreePath)
	if _, err := os.Stat(wt); err != nil {
		return fmt.Sprintf(repoStateFormat, paneui.Bold(name), paneui.Muted(stateMissing))
	}
	ref, err := gitOutput(wt, "branch", "--show-current")
	if err == nil && ref == "" {
		// detached HEAD — a pinned reference checkout
		var revision string
		if revision, err = gitOutput(wt, "rev-parse", "--short", "HEAD"); err == nil {
			ref = detachedPrefix + revision
		}
	}
	if err != nil {
		return fmt.Sprintf(repoStateFormat, paneui.Bold(name), paneui.Muted(stateUnavailable))
	}
	state := stateClean
	if status, err := gitOutput(wt, "status", "--porcelain"); err != nil {
		state = stateUnavailable
	} else if dirty := countLines(status); dirty > 0 {
		state = fmt.Sprintf(changedFormat, dirty)
	}
	if r.Role == session.RepoRoleReference {
		state = referencePrefix + state
	}
	return fmt.Sprintf(repoLineFormat, paneui.Bold(name), ref, state)
}

func gitOutput(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return strings.TrimSpace(string(out)), err
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
