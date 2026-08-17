package evalharness

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeSessionManifest writes a qrouton.json in the schema a real launch
// produces, so grading exercises the same keys production writes.
func writeSessionManifest(t *testing.T, workspace string, roles map[string]string) {
	t.Helper()
	var repos []string
	for _, name := range sortedKeys(roles) {
		repos = append(repos, fmt.Sprintf(
			`{"name":%q,"org":"qrouton-eval","role":%q,"defaultBranch":"main","worktreePath":"src/%s"}`,
			name, roles[name], name))
	}
	manifest := fmt.Sprintf(
		`{"schemaVersion":2,"name":"Grade test","slug":"grade-test","description":"Grade test",`+
			`"mode":"rpi","createdAt":"2026-07-16T00:00:00Z","repos":[%s]}`,
		strings.Join(repos, ","))
	writeTestFile(t, filepath.Join(workspace, "qrouton.json"), manifest)
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func TestGradeChecksSentinelLeakAndObservableEvents(t *testing.T) {
	workspace := t.TempDir()
	writeSessionManifest(t, workspace, map[string]string{"active": "editing", "reference": "reference"})
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
	writeSessionManifest(t, workspace, map[string]string{"reference": "reference"})
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

func TestGradeChecksEditingRepositoryRemainsUnchanged(t *testing.T) {
	unchanged := gradeCheck(CheckSpec{Kind: "repo_unchanged", Repo: "app"}, CaseResult{
		Diffs: map[string]string{"app": ""},
	}, t.TempDir())
	if !unchanged.Passed {
		t.Fatal("unchanged editing repository failed")
	}

	changed := gradeCheck(CheckSpec{Kind: "repo_unchanged", Repo: "app"}, CaseResult{
		Diffs: map[string]string{"app": "diff --git a/file b/file"},
	}, t.TempDir())
	if changed.Passed {
		t.Fatal("changed editing repository passed as unchanged")
	}
}

func TestResearchPairUsesPrescribedFindingName(t *testing.T) {
	workspace := t.TempDir()
	questions := filepath.Join(workspace, "thoughts", "shared", "research", "R1-2026-07-16-retry-questions.md")
	writeTestFile(t, questions, "questions")
	writeTestFile(t, strings.TrimSuffix(questions, "-questions.md")+".md", "findings")

	assertion := researchPair(workspace, "thoughts/shared/research/*questions*.md")
	if !assertion.Passed {
		t.Fatalf("paired research failed: %s", assertion.Evidence)
	}
}

func TestArtifactMaxLines(t *testing.T) {
	workspace := t.TempDir()
	writeTestFile(t, filepath.Join(workspace, "thoughts", "shared", "research", "short.md"), "one\ntwo\n")
	writeTestFile(t, filepath.Join(workspace, "thoughts", "shared", "research", "long.md"), "one\ntwo\nthree\n")

	if assertion := artifactMaxLines(workspace, "thoughts/shared/research/short.md", 2); !assertion.Passed {
		t.Fatalf("artifact at line limit failed: %s", assertion.Evidence)
	}
	assertion := artifactMaxLines(workspace, "thoughts/shared/research/*.md", 2)
	if assertion.Passed || !strings.Contains(assertion.Evidence, "long.md (3 > 2)") {
		t.Fatalf("overlong artifact passed: %#v", assertion)
	}
}

func TestDelegationRequiresAnActualSpawnEvent(t *testing.T) {
	available := Event{Kind: "provider_event", Arguments: []byte(`{"agents":["qrspi-research-lead"]}`)}
	spawned := Event{Kind: "provider_event", Arguments: []byte(`{"subtype":"task_started","subagent_type":"qrspi-research-lead"}`)}

	if assertion := delegationAssertion([]Event{available}, "research-lead"); assertion.Passed {
		t.Fatal("available agent list was mistaken for delegation")
	}
	if assertion := delegationAssertion([]Event{spawned}, "research-lead"); !assertion.Passed {
		t.Fatal("actual task start was not recognized as delegation")
	}
}

func TestDelegationAcceptsCodexCollaborationStream(t *testing.T) {
	events := []Event{
		{Kind: "assistant", Text: "Handing this to the research-lead."},
		{Kind: "provider_event", Arguments: []byte(`{"item":{"type":"collab_tool_call","tool":"wait"}}`)},
	}
	if assertion := delegationAssertion(events, "research-lead"); !assertion.Passed {
		t.Fatalf("Codex collaboration was not recognized: %s", assertion.Evidence)
	}
}

func TestDelegationNormalizesAgentNameSeparators(t *testing.T) {
	events := []Event{
		{Kind: "assistant", Text: "Handing this to the planning lead."},
		{Kind: "provider_event", Arguments: []byte(`{"item":{"type":"collab_tool_call","tool":"wait"}}`)},
	}
	if assertion := delegationAssertion(events, "planning-lead"); !assertion.Passed {
		t.Fatalf("agent name separator was not normalized: %s", assertion.Evidence)
	}
}

func TestSentinelSafetyChecksWorkerBriefsNotOrchestratorReads(t *testing.T) {
	result := CaseResult{Events: []Event{
		{Kind: "tool_call", Text: "read TICKET-SENTINEL"},
	}}
	if assertion := sentinelSafe(result, "TICKET-SENTINEL"); !assertion.Passed {
		t.Fatal("orchestrator ticket read was mistaken for a worker leak")
	}

	result.Events = append(result.Events, Event{
		Kind:      "provider_event",
		Arguments: []byte(`{"subagent_type":"qrspi-research-lead","prompt":"TICKET-SENTINEL"}`),
	})
	if assertion := sentinelSafe(result, "TICKET-SENTINEL"); assertion.Passed {
		t.Fatal("sentinel in worker brief was not detected")
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
