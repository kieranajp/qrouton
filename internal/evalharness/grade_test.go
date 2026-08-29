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

// A framed document and a finished one are the same file; what separates them
// is whether anything sits under the headings.
func TestResearchAnsweredSeparatesFramingFromFindings(t *testing.T) {
	workspace := t.TempDir()
	document := filepath.Join(workspace, "thoughts", "shared", "research", "R1-2026-07-16-retry.md")
	pattern := "thoughts/shared/research/*.md"

	writeTestFile(t, document, strings.Join([]string{
		"# Retry",
		"",
		"## Summary",
		"",
		"What is being investigated, and where to look.",
		"",
		"## How does the client retry?",
		"",
		"> Start in internal/retry.",
		"",
		"## Open Questions",
		"",
	}, "\n"))

	framed := researchAnswered(workspace, pattern)
	if framed.Passed {
		t.Fatal("a document nobody has answered passed as research")
	}
	for _, section := range []string{"How does the client retry?", "Open Questions"} {
		if !strings.Contains(framed.Evidence, section) {
			t.Errorf("evidence %q does not name %q", framed.Evidence, section)
		}
	}

	writeTestFile(t, document, strings.Join([]string{
		"# Retry",
		"",
		"## Summary",
		"",
		"It backs off three times.",
		"",
		"## How does the client retry?",
		"",
		"Three attempts, doubling the wait (`internal/retry/client.go:41`).",
		"",
		"```go",
		"## not a heading",
		"```",
		"",
		"## Open Questions",
		"",
		"None.",
		"",
	}, "\n"))

	if answered := researchAnswered(workspace, pattern); !answered.Passed {
		t.Fatalf("a filled-in document failed: %s", answered.Evidence)
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
	available := Event{Kind: "provider_event", Arguments: []byte(`{"agents":["qrouton-research-lead"]}`)}
	spawned := Event{Kind: "provider_event", Arguments: []byte(`{"subtype":"task_started","subagent_type":"qrouton-research-lead"}`)}

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
		Arguments: []byte(`{"subagent_type":"qrouton-research-lead","prompt":"TICKET-SENTINEL"}`),
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
