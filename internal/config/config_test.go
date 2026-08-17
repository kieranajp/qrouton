package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if cfg.Welcomed {
		t.Fatal("a config nobody has written claims first run has been through")
	}
	if _, err := os.Stat(Path()); !os.IsNotExist(err) {
		t.Fatal("Load wrote a config file; nothing was configured yet")
	}
}

// A hand-written config predating the marker sees the first-run flow once.
func TestLoadReadsAConfigWithoutTheWelcomedMarkerAsNotWelcomed(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("QROUTON_ROOT", t.TempDir())
	dir := filepath.Join(configHome, appDirName)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, configFileName),
		[]byte(`{"orgs":["acme"],"root":"unused"}`), fileMode); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Welcomed {
		t.Fatal("a config with no welcomed key reads as welcomed")
	}
}

func TestSaveWritesTheWelcomedMarkerAndOmitsItWhenUnset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	if err := Save(&Config{Root: "/sessions", Welcomed: true}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"welcomed": true`) {
		t.Fatalf("saved config carries no welcomed marker: %s", b)
	}

	if err := Save(&Config{Root: "/sessions"}); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"welcomed"`) {
		t.Fatalf("saved config claims a marker nobody set: %s", b)
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

func TestLegacyWindowsPreferenceIsIgnoredAndOmittedOnSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("QROUTON_ROOT", filepath.Join(dir, "sessions"))
	configDir := filepath.Join(dir, appDirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"orgs":["acme"],"root":"unused","windows":"float","editor":["vi"]}`
	if err := os.WriteFile(filepath.Join(configDir, configFileName), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Orgs, []string{"acme"}) || !reflect.DeepEqual(cfg.Editor, []string{"vi"}) {
		t.Fatalf("legacy config lost ordinary fields: %+v", cfg)
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(filepath.Join(configDir, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), `"windows"`) {
		t.Fatalf("saved config retained the retired preference: %s", saved)
	}
}

// A successor inheriting an override would have the answer just written to
// config.json overridden the moment it read it.
func TestWithoutOverridesDropsOnlyTheRuntimeOverrides(t *testing.T) {
	env := []string{"PATH=/usr/bin", rootEnvVar + "=/old/sessions", "HOME=/home/me",
		orgsEnvVar + "=acme,other", "QROUTON_ROOTLESS=keep"}
	want := []string{"PATH=/usr/bin", "HOME=/home/me", "QROUTON_ROOTLESS=keep"}
	if got := WithoutOverrides(env); !reflect.DeepEqual(got, want) {
		t.Fatalf("WithoutOverrides() = %#v, want %#v", got, want)
	}

	plain := []string{"PATH=/usr/bin", "HOME=/home/me"}
	if got := WithoutOverrides(plain); !reflect.DeepEqual(got, plain) {
		t.Fatalf("WithoutOverrides() = %#v, want it unchanged", got)
	}
}

func TestExpandHomeResolvesTildeAndLeavesAnAbsolutePathAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := ExpandHome("~"), home; got != want {
		t.Fatalf("ExpandHome(%q) = %q, want %q", "~", got, want)
	}
	if got, want := ExpandHome("~/work"), filepath.Join(home, "work"); got != want {
		t.Fatalf("ExpandHome(%q) = %q, want %q", "~/work", got, want)
	}
	if got, want := ExpandHome("/srv/sessions"), "/srv/sessions"; got != want {
		t.Fatalf("ExpandHome(%q) = %q, want %q", "/srv/sessions", got, want)
	}
}
