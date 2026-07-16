package evalharness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func LoadScenarios(dir, selected string) ([]Scenario, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
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
		if selected == "" || selected == "all" || selected == scenario.ID {
			scenarios = append(scenarios, scenario)
		}
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("scenario %q not found in %s", selected, dir)
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
		return Scenario{}, fmt.Errorf("%s: id, fixture, and turns are required", path)
	}
	if scenario.Version == 0 {
		scenario.Version = ScenarioVersion
	}
	return scenario, nil
}
