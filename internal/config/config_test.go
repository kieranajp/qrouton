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

// A zero-repo session needs neither a configured root nor GitHub owners, so
// Load must succeed with no config file at all and without prompting. If this
// regresses, `qrouton <dir>` stops being a no-configuration entry point.
func TestLoadWithoutConfigFileDefaultsInsteadOfPrompting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	t.Setenv("QROUTON_ROOT", root)

	cfg, err := Load()
	if err != nil {
		t.Fatal("Load failed with no config file:", err)
	}
	if len(cfg.Orgs) != 0 {
		t.Fatalf("orgs invented from nowhere: %#v", cfg.Orgs)
	}
	if cfg.Root != root {
		t.Fatalf("root = %q, want %q", cfg.Root, root)
	}
	if _, err := os.Stat(Path()); !os.IsNotExist(err) {
		t.Fatal("Load wrote a config file; nothing was configured yet")
	}
}

// With no root anywhere, sessions still get somewhere to live.
func TestLoadDefaultsRootWhenUnset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("QROUTON_ROOT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Root == "" || cfg.Root == defaultRoot {
		t.Fatalf("root not resolved to an absolute default: %q", cfg.Root)
	}
	if _, err := os.Stat(cfg.Root); err != nil {
		t.Fatal("default root not created:", err)
	}
}

func TestLoadReadsLaunchOverridesVerbatim(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("QROUTON_ROOT", t.TempDir())
	dir := filepath.Join(configHome, "qrouton")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "config.json"),
		[]byte(`{"orgs":["acme"],"root":"unused","launch":{"claude":["claude","--verbose"]}}`), 0o644)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"claude": {"claude", "--verbose"}}
	if !reflect.DeepEqual(cfg.Launch, want) {
		t.Fatalf("launch = %#v, want %#v", cfg.Launch, want)
	}
}

func TestWindowsPreferenceRoundTrips(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("QROUTON_ROOT", filepath.Join(dir, "sessions"))

	if err := Save(&Config{Root: filepath.Join(dir, "sessions"), Windows: WindowsDock}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Windows != WindowsDock || !cfg.Dock() {
		t.Fatalf("windows = %q, Dock() = %v", cfg.Windows, cfg.Dock())
	}
	for _, value := range []string{"", WindowsFloat, "tiled"} {
		if (&Config{Windows: value}).Dock() {
			t.Errorf("Windows=%q docked; anything but %q floats", value, WindowsDock)
		}
	}
}
