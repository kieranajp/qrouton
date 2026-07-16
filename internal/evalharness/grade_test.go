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
