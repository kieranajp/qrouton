package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSplitOrgsTrimsDeduplicatesAndDropsEmptyValues(t *testing.T) {
	got := splitOrgs(" lifesum, second-org, lifesum, ,third ")
	want := []string{"lifesum", "second-org", "third"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitOrgs() = %#v, want %#v", got, want)
	}
}

func TestLoadConfigMigratesLegacyClaudeDefault(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("QROUTON_ROOT", t.TempDir())
	dir := filepath.Join(configHome, "qrouton")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"orgs":["acme"],"root":"unused","launch":[["claude"]]}`), 0o644)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Launch != nil {
		t.Fatalf("legacy default was not migrated: %#v", cfg.Launch)
	}
}
