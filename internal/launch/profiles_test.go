package launch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/codex"
	"github.com/kieranajp/qrouton/internal/config"
)

func profiled(id, dir string) Runner {
	return Runner{ID: id, Command: []string{id}, Profile: config.RunnerProfile{ID: "work", Label: "Work", Dir: dir}}
}

func envValues(env []string, key string) []string {
	var values []string
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, key+"="); ok {
			values = append(values, value)
		}
	}
	return values
}

func launchEnv(t *testing.T, r Runner) []string {
	t.Helper()
	_, env, err := runnerLaunch(r, "/bin/qrouton", sessionWithOwnHome(t), EditorCommand{}, testHandle(), 1, false, "")
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestNamedClaudeProfileSetsConfigDir(t *testing.T) {
	t.Setenv(claudeConfigDirEnvVar, t.TempDir())
	dir := filepath.Join(t.TempDir(), ".claude-work")
	got := envValues(launchEnv(t, profiled(runnerIDClaude, dir)), claudeConfigDirEnvVar)
	if !slices.Equal(got, []string{dir}) {
		t.Fatalf("%s = %q, want only the profile's %q", claudeConfigDirEnvVar, got, dir)
	}
}

func TestNamedCodexProfileSetsCodexHome(t *testing.T) {
	t.Setenv(codex.HomeEnvVar, t.TempDir())
	dir := filepath.Join(t.TempDir(), ".codex-work")
	got := envValues(launchEnv(t, profiled(runnerIDCodex, dir)), codex.HomeEnvVar)
	if !slices.Equal(got, []string{dir}) {
		t.Fatalf("%s = %q, want only the profile's %q", codex.HomeEnvVar, got, dir)
	}
}

func TestDefaultProfileLeavesInheritedVariableAlone(t *testing.T) {
	for id, key := range map[string]string{runnerIDClaude: claudeConfigDirEnvVar, runnerIDCodex: codex.HomeEnvVar} {
		inherited := t.TempDir()
		t.Setenv(key, inherited)
		got := envValues(launchEnv(t, Runner{ID: id, Command: []string{id}}), key)
		if !slices.Equal(got, []string{inherited}) {
			t.Errorf("%s default profile %s = %q, want the inherited %q", id, key, got, inherited)
		}
	}
}

func TestProfileFolderIsCreatedPrivateBeforeLaunch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing", ".claude-work")
	launchEnv(t, profiled(runnerIDClaude, dir))
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("profile folder was not created: %v", err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("profile folder mode = %v, want a 0700 directory", info.Mode())
	}
}

func TestCodexDepthReadsTheProfilesConfig(t *testing.T) {
	deep := "[agents]\nmax_depth = 5\n"
	writeDepth := func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(deep), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	inherited, profile := t.TempDir(), t.TempDir()
	writeDepth(inherited)
	t.Setenv(codex.HomeEnvVar, inherited)
	argv, _, err := runnerLaunch(profiled(runnerIDCodex, profile), "/bin/qrouton", t.TempDir(), EditorCommand{}, testHandle(), 1, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := codex.MaxDepth(profile, argv); got != codex.RequiredMaxDepth {
		t.Fatalf("a shallow profile runs at depth %d, want %d\nargv: %v", got, codex.RequiredMaxDepth, argv)
	}

	inherited, profile = t.TempDir(), t.TempDir()
	writeDepth(profile)
	t.Setenv(codex.HomeEnvVar, inherited)
	argv, _, err = runnerLaunch(profiled(runnerIDCodex, profile), "/bin/qrouton", t.TempDir(), EditorCommand{}, testHandle(), 1, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := codex.MaxDepth(profile, argv); got != 5 {
		t.Fatalf("a deep profile runs at depth %d, want its own 5\nargv: %v", got, argv)
	}
}

func TestRunnersRejectProfilesForOpenCodeAndAgy(t *testing.T) {
	old := findExecutable
	t.Cleanup(func() { findExecutable = old })
	findExecutable = func(name string) (string, error) { return "/bin/" + name, nil }
	work := []config.RunnerProfile{{ID: "work", Label: "Work", Dir: "/tmp/work"}}

	for _, id := range []string{runnerIDOpenCode, runnerIDAgy, "team-agent"} {
		_, err := Runners(&config.Config{Profiles: map[string][]config.RunnerProfile{id: work}})
		if !errors.Is(err, ErrProfilesUnsupported) || !strings.Contains(err.Error(), id) {
			t.Errorf("profiles for %s: %v, want %v naming it", id, err, ErrProfilesUnsupported)
		}
	}
	if _, err := Runners(&config.Config{Profiles: map[string][]config.RunnerProfile{
		runnerIDClaude: work, runnerIDCodex: work,
	}}); err != nil {
		t.Fatalf("profiles for claude and codex: %v", err)
	}
}

func TestWithProfileRefusesAMissingProfile(t *testing.T) {
	work := config.RunnerProfile{ID: "work", Label: "Work", Dir: "/tmp/work"}
	cfg := &config.Config{Profiles: map[string][]config.RunnerProfile{runnerIDClaude: {work}}}
	claude := Runner{ID: runnerIDClaude, Command: []string{runnerIDClaude}}

	for _, tc := range []struct{ runner, profile string }{{runnerIDClaude, "gone"}, {runnerIDCodex, "work"}} {
		_, err := WithProfile(cfg, Runner{ID: tc.runner}, tc.profile)
		if !errors.Is(err, ErrProfileUnavailable) {
			t.Fatalf("%s %s: %v, want %v", tc.runner, tc.profile, err, ErrProfileUnavailable)
		}
		if !strings.Contains(err.Error(), tc.runner) || !strings.Contains(err.Error(), tc.profile) {
			t.Fatalf("%q does not name the runner %q and profile %q", err, tc.runner, tc.profile)
		}
	}

	if got, err := WithProfile(cfg, claude, ""); err != nil || got.Profile != (config.RunnerProfile{}) {
		t.Fatalf("default profile = %+v, %v", got.Profile, err)
	}
	if got, err := WithProfile(cfg, claude, "work"); err != nil || got.Profile != work {
		t.Fatalf("named profile = %+v, %v", got.Profile, err)
	}
}

func TestSuperviseArgvCarriesTheProfile(t *testing.T) {
	argv := superviseArgv("/bin/qrouton", "/sessions/s", profiled(runnerIDClaude, "/tmp/work"), testHandle(), EditorCommand{}, true)
	if i := flagValue(argv, profileFlag); i == 0 || argv[i] != "work" {
		t.Fatalf("supervisor argv %q does not carry the profile", argv)
	}
	argv = superviseArgv("/bin/qrouton", "/sessions/s", Runner{ID: runnerIDClaude}, testHandle(), EditorCommand{}, true)
	if slices.Contains(argv, profileFlag) {
		t.Fatalf("default profile's supervisor argv %q names a profile", argv)
	}
}
