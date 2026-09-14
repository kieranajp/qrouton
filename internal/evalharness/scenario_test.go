package evalharness

import (
	"fmt"
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
			if problem := checkSpecProblem(scenario, check); problem != "" {
				t.Errorf("%s: %s", scenario.ID, problem)
			}
		}
	}
}

// checkSpecProblem reports why a check could never grade what its name claims.
// A delegation check with no pattern matches every agent there is, and a
// turn-scoped one aimed past the turns a scenario sends grades an empty stream.
func checkSpecProblem(scenario Scenario, check CheckSpec) string {
	switch check.Kind {
	case checkDelegation, checkFirstDelegation, checkTurnDelegation:
		if check.Pattern == "" {
			return fmt.Sprintf("%s names no agent", check.Kind)
		}
	}
	if check.Kind == checkTurnDelegation && (check.Turn < 1 || check.Turn > len(scenario.Turns)) {
		return fmt.Sprintf("%s grades turn %d of %d", check.Kind, check.Turn, len(scenario.Turns))
	}
	return ""
}

func TestCheckSpecProblemRejectsAnUngradableSpec(t *testing.T) {
	scenario := Scenario{ID: "two-turns", Turns: []string{"first", "second"}}
	cases := map[string]struct {
		check   CheckSpec
		problem string
	}{
		"turn-scoped check with no turn": {
			check:   CheckSpec{Kind: checkTurnDelegation, Pattern: "lead"},
			problem: "turn_delegation grades turn 0 of 2",
		},
		"turn-scoped check past the last turn": {
			check:   CheckSpec{Kind: checkTurnDelegation, Pattern: "lead", Turn: 3},
			problem: "turn_delegation grades turn 3 of 2",
		},
		"turn-scoped check naming no agent": {
			check:   CheckSpec{Kind: checkTurnDelegation, Turn: 2},
			problem: "turn_delegation names no agent",
		},
		"whole-run check naming no agent": {
			check:   CheckSpec{Kind: checkDelegation},
			problem: "delegation names no agent",
		},
		"usable turn-scoped check": {
			check: CheckSpec{Kind: checkTurnDelegation, Pattern: "lead", Turn: 2},
		},
		"check with no turn to grade": {
			check: CheckSpec{Kind: checkRepoUnchanged, Repo: "account-service"},
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if problem := checkSpecProblem(scenario, testCase.check); problem != testCase.problem {
				t.Errorf("problem = %q, want %q", problem, testCase.problem)
			}
		})
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
