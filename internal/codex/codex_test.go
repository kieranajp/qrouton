package codex

import (
	"os"
	"path/filepath"
	"testing"
)

// home points CODEX_HOME at a directory holding the given config.toml, or at an
// empty one when body is "" — which is the install that has never been configured.
func home(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(homeEnvVar, dir)
	if body != "" {
		if err := os.WriteFile(filepath.Join(dir, configFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestHomePrefersTheEnvironment(t *testing.T) {
	dir := home(t, "")
	if got := Home(); got != dir {
		t.Fatalf("Home() = %q, want %q", got, dir)
	}
}

// Codex accepts agents.max_depth under an [agents] table or as a dotted key at
// top level, and qrouton has to read back whichever the user wrote.
func TestConfiguredMaxDepthReadsBothSpellings(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"absent config", "", DefaultMaxDepth},
		{"empty config", "\n", DefaultMaxDepth},
		{"agents table", "[agents]\nmax_depth = 3\n", 3},
		{"dotted at top level", "agents.max_depth = 4\n", 4},
		{"table with other keys around it", "[other]\nmax_depth = 9\n\n[agents]\nmax_depth = 5\nfoo = 1\n", 5},
		// A dotted key is top-level only: inside a table it names something else.
		{"dotted inside a table", "[other]\nagents.max_depth = 9\n", DefaultMaxDepth},
		{"commented out", "[agents]\n# max_depth = 8\n", DefaultMaxDepth},
		{"trailing comment", "[agents]\nmax_depth = 6 # two levels and one spare\n", 6},
		// An unparseable config leaves the default standing rather than failing
		// a launch.
		{"unparseable value", "[agents]\nmax_depth = deep\n", DefaultMaxDepth},
		{"not toml at all", "]]] broken [[[\n", DefaultMaxDepth},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home(t, tc.body)
			if got := MaxDepth(nil); got != tc.want {
				t.Fatalf("MaxDepth(nil) = %d, want %d", got, tc.want)
			}
		})
	}
}

// Codex's own precedence: a command-line -c beats the file.
func TestArgvOverridesTheConfigFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		argv []string
		want int
	}{
		{"no argv", []string{Binary}, 3},
		{"short flag", []string{Binary, ConfigFlag, "agents.max_depth=5"}, 5},
		{"long flag joined", []string{Binary, "--config=agents.max_depth=6"}, 6},
		{"long flag spaced", []string{Binary, "--config", "agents.max_depth=7"}, 7},
		{"another key entirely", []string{Binary, ConfigFlag, "model=o3"}, 3},
		{"padded value", []string{Binary, ConfigFlag, "agents.max_depth= 8 "}, 8},
		{"unparseable override leaves the file standing", []string{Binary, ConfigFlag, "agents.max_depth=deep"}, 3},
		{"last override wins", []string{Binary, ConfigFlag, "agents.max_depth=5", ConfigFlag, "agents.max_depth=9"}, 9},
		// A flag with nothing after it must not read past the end of argv.
		{"dangling flag", []string{Binary, ConfigFlag}, 3},
		// argv[0] is the binary, never a setting.
		{"argv[0] is not scanned", []string{"agents.max_depth=99"}, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home(t, "[agents]\nmax_depth = 3\n")
			if got := MaxDepth(tc.argv); got != tc.want {
				t.Fatalf("MaxDepth(%v) = %d, want %d", tc.argv, got, tc.want)
			}
		})
	}
}

// The launcher pairs MaxDepthSetting with ConfigFlag to raise the depth, then
// nothing re-reads it — so this round trip is what makes that injection correct.
func TestMaxDepthSettingIsReadBackByMaxDepth(t *testing.T) {
	home(t, "")
	argv := []string{Binary, ConfigFlag, MaxDepthSetting(RequiredMaxDepth)}
	if got := MaxDepth(argv); got != RequiredMaxDepth {
		t.Fatalf("MaxDepth(%v) = %d, want the depth just set (%d)", argv, got, RequiredMaxDepth)
	}
}

// The reason the launcher injects at all: Codex's own default is one level, and
// a lead at one level cannot spawn the specialists it delegates to.
func TestCodexDefaultIsTooShallowForALead(t *testing.T) {
	if DefaultMaxDepth >= RequiredMaxDepth {
		t.Fatalf("default depth %d already meets the required %d, so the injection is pointless",
			DefaultMaxDepth, RequiredMaxDepth)
	}
}
