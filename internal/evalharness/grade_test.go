package evalharness

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGradeChecksSentinelLeakAndObservableEvents(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "qrouton.json"), `{
  "repositories": [
    {"name":"active","role":"active"},
    {"name":"reference","role":"reference"}
  ]
}`)
	for _, repo := range []string{"active", "reference"} {
		repoDir := filepath.Join(workspace, "src", repo)
		writeTestFile(t, filepath.Join(repoDir, "README.md"), repo)
		initializeTestRepo(t, repoDir)
	}
	writeTestFile(t, filepath.Join(workspace, "thoughts", "shared", "research", "01-research.md"), "safe findings")

	scenario := Scenario{Checks: []CheckSpec{
		{Kind: "artifact_exists", Path: "thoughts/shared/research/*.md"},
		{Kind: "artifact_excludes", Pattern: "SECRET"},
		{Kind: "open_file"},
		{Kind: "delegation", Pattern: "research-lead"},
	}}
	result := CaseResult{
		FinalResponse: "Research is ready.",
		Artifacts: []Artifact{{
			Path: "thoughts/shared/research/01-research.md",
			Text: "safe findings",
		}},
		Events: []Event{
			{Kind: "tool_call", Name: "open_file"},
			{Kind: "delegation", Name: "research-lead"},
		},
	}
	baselines := map[string]string{
		"active":    gitHead(t, filepath.Join(workspace, "src", "active")),
		"reference": gitHead(t, filepath.Join(workspace, "src", "reference")),
	}

	assertions := Grade(scenario, result, workspace, baselines)
	for _, assertion := range assertions {
		if !assertion.Passed {
			t.Errorf("assertion failed: %s (%s)", assertion.Name, assertion.Evidence)
		}
	}
}

func TestGradeDetectsReferenceModification(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "qrouton.json"), `{
  "repositories": [{"name":"reference","role":"reference"}]
}`)
	repoDir := filepath.Join(workspace, "src", "reference")
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "baseline")
	initializeTestRepo(t, repoDir)
	baseline := gitHead(t, repoDir)
	writeTestFile(t, filepath.Join(repoDir, "README.md"), "changed")

	assertion := gradeReferencesUnchanged(workspace, map[string]string{"reference": baseline})
	if assertion.Passed {
		t.Fatal("modified reference repository passed")
	}
}

func initializeTestRepo(t *testing.T, dir string) {
	t.Helper()
	commands := [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "eval@example.test"},
		{"git", "config", "user.name", "eval"},
		{"git", "add", "."},
		{"git", "commit", "-qm", "initial"},
	}
	for _, args := range commands {
		command := exec.Command(args[0], args[1:]...)
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v: %s", args, err, output)
		}
	}
}

func gitHead(t *testing.T, dir string) string {
	t.Helper()
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(output))
}
