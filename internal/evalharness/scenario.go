package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
)

func LoadScenarios(dir, selected string) ([]Scenario, error) {
	paths, err := filepath.Glob(filepath.Join(dir, scenarioGlob))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	var scenarios []Scenario
	for _, path := range paths {
		scenario, err := loadScenario(path)
		if err != nil {
			return nil, err
		}
		if selected == "" || selected == scenarioAll || selected == scenario.ID {
			scenarios = append(scenarios, scenario)
		}
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("%w: %q in %s", ErrScenarioNotFound, selected, dir)
	}
	return scenarios, nil
}

func loadScenario(path string) (Scenario, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}

	var scenario Scenario
	if err := json.Unmarshal(content, &scenario); err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", path, err)
	}
	if scenario.ID == "" || scenario.Fixture == "" || len(scenario.Turns) == 0 {
		return Scenario{}, fmt.Errorf("%s: %w", path, ErrScenarioIncomplete)
	}
	for _, check := range scenario.Checks {
		if problem := checkSpecProblem(scenario, check); problem != "" {
			return Scenario{}, fmt.Errorf("%s: %w: %s", path, ErrCheckUngradable, problem)
		}
	}
	if scenario.Version == 0 {
		scenario.Version = ScenarioVersion
	}
	return scenario, nil
}

// checkSpecProblem reports why a check could never grade what its name claims.
// An empty pattern matches every response, agent or artifact there is, and a
// turn-scoped check aimed past the turns a scenario sends grades an empty stream.
func checkSpecProblem(scenario Scenario, check CheckSpec) string {
	switch check.Kind {
	case checkDelegation, checkFirstDelegation, checkTurnDelegation:
		if check.Pattern == "" {
			return fmt.Sprintf("%s names no agent", check.Kind)
		}
	case checkResponseContains, checkArtifactContains, checkArtifactExcludes, checkSentinelSafe:
		if check.Pattern == "" {
			return fmt.Sprintf("%s has an empty pattern", check.Kind)
		}
	case checkResponseMatches:
		if check.Pattern == "" {
			return fmt.Sprintf("%s has an empty pattern", check.Kind)
		}
		if _, err := regexp.Compile(check.Pattern); err != nil {
			return fmt.Sprintf("%s pattern does not compile: %v", check.Kind, err)
		}
	case checkResponseExcludes:
		if check.Pattern == "" && len(check.Any) == 0 || slices.Contains(check.Any, "") {
			return fmt.Sprintf("%s has an empty pattern", check.Kind)
		}
	}
	if check.Kind == checkTurnDelegation && (check.Turn < 1 || check.Turn > len(scenario.Turns)) {
		return fmt.Sprintf("%s grades turn %d of %d", check.Kind, check.Turn, len(scenario.Turns))
	}
	return ""
}
