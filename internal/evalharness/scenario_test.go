package evalharness

import (
	"os"
	"path/filepath"
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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
