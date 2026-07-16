package repos

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/agents"
	"github.com/kieranajp/qrouton/internal/session"
)

// Status redraws the session's per-repo branch and dirty state every 3s, forever.
// It replaces the generated status.sh pane: manifest-driven, so it knows roles
// and exact worktree paths (a detached reference renders as its pinned revision
// instead of a blank branch), and drawn in place via agents.Frame so the pane
// never flashes blank between ticks.
func Status(root string) error {
	for {
		fmt.Print(agents.Frame(statusLines(root)))
		time.Sleep(3 * time.Second)
	}
}

func statusLines(root string) []string {
	lines := []string{"\033[1mrepos\033[0m"}
	b, err := os.ReadFile(filepath.Join(root, "qrouton.json"))
	var m session.Manifest
	if err != nil || json.Unmarshal(b, &m) != nil || len(m.Repos) == 0 {
		return append(lines, "\033[2mNo session manifest\033[0m")
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
		return fmt.Sprintf("\033[1m%s\033[0m  \033[2mmissing — resume to restore\033[0m", name)
	}
	ref, err := gitOutput(wt, "branch", "--show-current")
	if err == nil && ref == "" {
		// detached HEAD — a pinned reference checkout
		ref, err = gitOutput(wt, "rev-parse", "--short", "HEAD")
		ref = "@ " + ref
	}
	if err != nil {
		return fmt.Sprintf("\033[1m%s\033[0m  \033[2munavailable\033[0m", name)
	}
	state := "clean"
	if status, err := gitOutput(wt, "status", "--porcelain"); err != nil {
		state = "unavailable"
	} else if dirty := countLines(status); dirty > 0 {
		state = fmt.Sprintf("%d changed", dirty)
	}
	if r.Role == session.RepoRoleReference {
		state = "reference · " + state
	}
	return fmt.Sprintf("\033[1m%s\033[0m  %s · %s", name, ref, state)
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
