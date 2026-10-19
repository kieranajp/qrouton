package evalharness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

func TestLoadScenariosRejectsAnUngradableCheck(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "bad.json"), `{
  "id": "bad",
  "fixture": "fixture-bad",
  "turns": ["hello"],
  "checks": [{"kind": "turn_delegation", "pattern": "lead", "turn": 2}]
}`)

	if _, err := LoadScenarios(dir, "all"); !errors.Is(err, ErrCheckUngradable) {
		t.Fatalf("err = %v, want %v", err, ErrCheckUngradable)
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
		}
	}
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
		"response_contains with an empty pattern": {
			check:   CheckSpec{Kind: checkResponseContains},
			problem: "response_contains has an empty pattern",
		},
		"artifact_contains with an empty pattern": {
			check:   CheckSpec{Kind: checkArtifactContains, Path: "thoughts/shared/plans/*.md"},
			problem: "artifact_contains has an empty pattern",
		},
		"artifact_excludes with an empty pattern": {
			check:   CheckSpec{Kind: checkArtifactExcludes},
			problem: "artifact_excludes has an empty pattern",
		},
		"sentinel_safe with an empty pattern": {
			check:   CheckSpec{Kind: checkSentinelSafe},
			problem: "sentinel_safe has an empty pattern",
		},
		"response_matches with an empty pattern": {
			check:   CheckSpec{Kind: checkResponseMatches},
			problem: "response_matches has an empty pattern",
		},
		"response_matches that does not compile": {
			check:   CheckSpec{Kind: checkResponseMatches, Pattern: "("},
			problem: "response_matches pattern does not compile: error parsing regexp: missing closing ): `(`",
		},
		"response_excludes with nothing to exclude": {
			check:   CheckSpec{Kind: checkResponseExcludes},
			problem: "response_excludes has an empty pattern",
		},
		"response_excludes holding an empty entry": {
			check:   CheckSpec{Kind: checkResponseExcludes, Any: []string{"legacy cache plan", ""}},
			problem: "response_excludes has an empty pattern",
		},
		"usable response_excludes": {
			check: CheckSpec{Kind: checkResponseExcludes, Any: []string{"legacy cache plan"}},
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

var researchBudgetPattern = regexp.MustCompile(`about (\d+) lines for research`)

// researchCapsBelow names every research line cap tighter than the length the
// prompts ask research to run to, which fails a document the prompt allows.
func researchCapsBelow(scenarios []Scenario, budget int) []string {
	var tight []string
	for _, scenario := range scenarios {
		for _, check := range scenario.Checks {
			if check.Kind == checkArtifactMaxLines && strings.Contains(check.Path, researchPathSegment) && check.MaxLines < budget {
				tight = append(tight, fmt.Sprintf("%s: %s caps at %d", scenario.ID, check.Path, check.MaxLines))
			}
		}
	}
	return tight
}

func TestResearchLineCapsHonourThePromptBudget(t *testing.T) {
	discipline, err := os.ReadFile(filepath.Join("..", "..", "prompts", "artifact-discipline.md"))
	if err != nil {
		t.Fatal(err)
	}
	match := researchBudgetPattern.FindSubmatch(discipline)
	if match == nil {
		t.Fatal("the artifact discipline prompt states no research length budget")
	}
	budget, err := strconv.Atoi(string(match[1]))
	if err != nil {
		t.Fatal(err)
	}
	scenarios, err := LoadScenarios(filepath.Join("..", "..", "eval", "scenarios"), scenarioAll)
	if err != nil {
		t.Fatal(err)
	}
	for _, problem := range researchCapsBelow(scenarios, budget) {
		t.Errorf("%s, below the %d-line research budget", problem, budget)
	}
}
