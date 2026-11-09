package evalharness

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/gittest"
)

func TestManifestCheckoutIsUsedForInitializationTestsDiffsAndReferences(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, manifestName), `{"repos":[{"name":"app","role":"reference","worktreePath":"src/owner/app"}]}`)
	repo := filepath.Join(workspace, "src", "owner", "app")
	writeTestFile(t, filepath.Join(repo, "go.mod"), "module example.com/fixture\n\ngo 1.27\n")
	writeTestFile(t, filepath.Join(repo, "app_test.go"), "package app\nimport \"testing\"\nfunc TestApp(t *testing.T) {}\n")
	decoy := filepath.Join(workspace, "src", "app")
	writeTestFile(t, filepath.Join(decoy, "package.json"), `{"scripts":{"test":"exit 1"}}`)
	baselines, err := initializeRepositories(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(decoy, ".git")); !os.IsNotExist(err) {
		t.Fatalf("flat decoy was initialized: %v", err)
	}
	if got := testsPass(workspace, "app"); !got.Passed {
		t.Fatalf("nested checkout tests failed: %s", got.Evidence)
	}
	if got := gradeReferencesUnchanged(workspace, baselines); !got.Passed {
		t.Fatalf("clean nested reference failed: %s", got.Evidence)
	}
	writeTestFile(t, filepath.Join(repo, "go.mod"), "module example.com/changed\n\ngo 1.27\n")
	if got := collectDiffs(workspace, baselines)["app"]; !strings.Contains(got, "example.com/changed") {
		t.Fatalf("nested diff omitted change: %s", got)
	}
	if got := gradeReferencesUnchanged(workspace, baselines); got.Passed {
		t.Fatal("modified nested reference passed")
	}
}

func TestManifestCheckoutFallsBackOnlyForMissingWorktreePath(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, manifestName), `{"repos":[{"name":"app"}]}`)
	got, err := repoDir(workspace, "app")
	if err != nil || got != filepath.Join(workspace, "src", "app") {
		t.Fatalf("legacy checkout = %q, %v", got, err)
	}
	if _, err := repoDir(workspace, "absent"); !errors.Is(err, ErrCheckoutMissing) {
		t.Fatalf("missing checkout error = %v", err)
	}
}

func TestDuplicateCheckoutNamesFailBeforeInitialization(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, manifestName), `{"repos":[{"name":"app","worktreePath":"src/one/app"},{"name":"app","worktreePath":"src/two/app"}]}`)
	if _, err := initializeRepositories(workspace); !errors.Is(err, ErrCheckoutAmbiguous) {
		t.Fatalf("duplicate checkout error = %v", err)
	}
}

func TestManifestRejectsMalformedCanonicalMetadata(t *testing.T) {
	workspace := t.TempDir()
	for _, content := range []string{`{"createdAt":"not-a-date"}`, `{"repos":[{"role":"unknown"}]}`, `{"repos":[{"worktreePath":"../outside"}]}`, `{"repos":[{"name":"../outside"}]}`, `{"repos":[{"name":""}]}`} {
		writeTestFile(t, filepath.Join(workspace, manifestName), content)
		if _, err := readManifest(filepath.Join(workspace, manifestName)); err == nil {
			t.Fatalf("malformed manifest was accepted: %s", content)
		}
	}
}

func TestTestGradingTimeoutDoesNotWaitForInheritedOutputPipes(t *testing.T) {
	workspace := t.TempDir()
	writeSessionManifest(t, workspace, map[string]string{"app": "editing"})
	writeTestFile(t, filepath.Join(workspace, "src", "app", "package.json"), `{"scripts":{"test":"sh test.sh"}}`)
	writeTestFile(t, filepath.Join(workspace, "src", "app", "test.sh"), "printf 'started\\n'\nsleep 10 &\nwait\n")
	started := time.Now()
	got := testsPassWithin(workspace, "app", time.Second)
	if got.Passed || !strings.Contains(got.Evidence, "started") || !strings.Contains(got.Evidence, "test run exceeded") {
		t.Fatalf("cancelled grading = %+v", got)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("grading waited for inherited output pipe: %s", elapsed)
	}
}

func TestCollectDiffsIncludesCommittedStagedAndUnstagedWork(t *testing.T) {
	workspace := t.TempDir()
	writeSessionManifest(t, workspace, map[string]string{"app": "editing"})
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
