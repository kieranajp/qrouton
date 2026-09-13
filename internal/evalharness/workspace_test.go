package evalharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/gittest"
)

func TestCollectDiffsIncludesCommittedStagedAndUnstagedWork(t *testing.T) {
	workspace := t.TempDir()
	repo := filepath.Join(workspace, srcDirName, "app")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(repo, "app.txt"), "baseline\n")
	baselines, err := initializeRepositories(workspace)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"committed", "staged", "unstaged"} {
		writeTestFile(t, filepath.Join(repo, stage+".txt"), stage+" change\n")
		if stage != "unstaged" {
			gittest.Run(t, repo, "add", ".")
		}
		if stage == "committed" {
			gittest.Run(t, repo, "commit", "-m", "runner change")
		}
	}
	writeTestFile(t, filepath.Join(repo, "app.txt"), "working tree change\n")
	diff := collectDiffs(workspace, baselines)["app"]
	for _, want := range []string{"committed change", "staged change", "working tree change", "unstaged.txt"} {
		if !strings.Contains(diff, want) {
			t.Fatalf("diff omitted %q:\n%s", want, diff)
		}
	}
}
