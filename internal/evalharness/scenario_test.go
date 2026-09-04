package evalharness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadScenariosSelectsAndDefaultsVersion(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "one.json"), `{
  "id": "one",
  "fixture": "fixture-one",
  "turns": ["hello"]
}`)
	writeTestFile(t, filepath.Join(dir, "two.json"), `{
  "id": "two",
  "version": 3,
  "fixture": "fixture-two",
  "turns": ["hello"]
}`)

	scenarios, err := LoadScenarios(dir, "one")
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 1 || scenarios[0].ID != "one" {
		t.Fatalf("unexpected scenarios: %#v", scenarios)
	}
	if scenarios[0].Version != ScenarioVersion {
		t.Fatalf("version = %d, want %d", scenarios[0].Version, ScenarioVersion)
	}
}

func TestLoadScenariosRejectsIncompleteDefinition(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "bad.json"), `{"id":"bad"}`)

	if _, err := LoadScenarios(dir, "all"); err == nil {
		t.Fatal("expected incomplete scenario error")
	}
}

// An unknown check kind is inert rather than loud: it grades as one failed
// assertion whose name nobody wrote, so a typo in a scenario reads as a prompt
// regression. Every declared kind is dispatched here against an empty run.
func TestShippedScenarioCheckKindsAreDispatched(t *testing.T) {
	scenarios, err := LoadScenarios(filepath.Join("..", "..", "eval", "scenarios"), scenarioAll)
	if err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	for _, scenario := range scenarios {
		if len(scenario.Checks) == 0 {
			t.Errorf("%s: scenario grades nothing deterministically", scenario.ID)
		}
		for _, check := range scenario.Checks {
			assertion := gradeCheck(check, CaseResult{}, workspace)
			if strings.HasPrefix(assertion.Name, assertUnknownCheck) {
				t.Errorf("%s: no grader for check kind %q", scenario.ID, check.Kind)
			}
			// A delegation check with no pattern matches every agent there is.
			if check.Kind == checkDelegation || check.Kind == checkFirstDelegation {
				if check.Pattern == "" {
					t.Errorf("%s: %s names no agent", scenario.ID, check.Kind)
				}
			}
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
