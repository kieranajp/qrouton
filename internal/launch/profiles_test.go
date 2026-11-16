package launch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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

type statusCall struct {
	argv, env []string
}

func fakeStatus(t *testing.T, out string, exit int) *statusCall {
	t.Helper()
	call := &statusCall{}
	real := runStatus
	t.Cleanup(func() { runStatus = real })
	runStatus = func(_ context.Context, argv, env []string) ([]byte, int, error) {
		call.argv, call.env = argv, env
		return []byte(out), exit, nil
	}
	return call
}

func installed(r Runner) Runner {
	r.Path = "/opt/bin/" + r.ID
	return r
}

func TestClaudeProbeReadsLoggedInAndEmail(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".claude-work")
	call := fakeStatus(t, `{"loggedIn": true, "authMethod": "claude.ai", "email": "k@example.com"}`, 0)
	got := ProbeSignIn(context.Background(), installed(profiled(runnerIDClaude, dir)))
	if got != (SignIn{State: SignInSignedIn, Email: "k@example.com"}) {
		t.Fatalf("signed-in probe = %+v", got)
	}
	if want := []string{"/opt/bin/claude", "auth", "status", "--json"}; !slices.Equal(call.argv, want) {
		t.Fatalf("probe ran %q, want %q", call.argv, want)
	}
	if got := envValues(call.env, claudeConfigDirEnvVar); !slices.Equal(got, []string{dir}) {
		t.Fatalf("probe ran with %s = %q, want the profile's %q", claudeConfigDirEnvVar, got, dir)
	}

	fakeStatus(t, `{"loggedIn": false}`, 1)
	if got := ProbeSignIn(context.Background(), installed(profiled(runnerIDClaude, dir))); got != (SignIn{State: SignInSignedOut}) {
		t.Fatalf("signed-out probe, which exits 1, = %+v", got)
	}
	fakeStatus(t, "not json", 0)
	if got := ProbeSignIn(context.Background(), installed(profiled(runnerIDClaude, dir))); got.State != SignInUnknown {
		t.Fatalf("unreadable probe = %+v, want unknown", got)
	}
}

func TestCodexProbeReadsTheExitCode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".codex-work")
	for exit, want := range map[int]SignInState{0: SignInSignedIn, 1: SignInSignedOut, 2: SignInUnknown} {
		call := fakeStatus(t, "Logged in using ChatGPT", exit)
		got := ProbeSignIn(context.Background(), installed(profiled(runnerIDCodex, dir)))
		if got != (SignIn{State: want}) {
			t.Errorf("exit %d = %+v, want %s and no email", exit, got, want)
		}
		if wantArgv := []string{"/opt/bin/codex", "login", "status"}; !slices.Equal(call.argv, wantArgv) {
			t.Fatalf("probe ran %q, want %q", call.argv, wantArgv)
		}
		if got := envValues(call.env, codex.HomeEnvVar); !slices.Equal(got, []string{dir}) {
			t.Fatalf("probe ran with %s = %q, want the profile's %q", codex.HomeEnvVar, got, dir)
		}
	}
}

func TestProbeTimeoutReadsUnknown(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexec sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	got := ProbeSignIn(ctx, Runner{ID: runnerIDClaude, Command: []string{bin}, Path: bin})
	if got.State != SignInUnknown {
		t.Fatalf("a probe that outlived its deadline = %+v, want unknown", got)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the probe waited %s past its deadline", elapsed)
	}
}

func TestSignInCommandSetsTheProfileVariable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	for _, tc := range []struct {
		runner Runner
		want   []string
	}{
		{installed(profiled(runnerIDClaude, dir)), []string{"env", "CLAUDE_CONFIG_DIR=" + dir, "/opt/bin/claude", "auth", "login"}},
		{installed(profiled(runnerIDCodex, dir)), []string{"env", "CODEX_HOME=" + dir, "/opt/bin/codex", "login"}},
		{installed(Runner{ID: runnerIDClaude}), []string{"/opt/bin/claude", "auth", "login"}},
	} {
		got, err := SignInCommand(tc.runner)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("SignInCommand(%s on %q) = %q, %v; want %q", tc.runner.ID, tc.runner.Profile.ID, got, err, tc.want)
		}
	}
	if _, err := SignInCommand(installed(Runner{ID: runnerIDOpenCode})); !errors.Is(err, ErrProfilesUnsupported) {
		t.Fatalf("OpenCode sign-in = %v, want %v", err, ErrProfilesUnsupported)
	}
	if _, err := SignInCommand(Runner{ID: runnerIDClaude}); !errors.Is(err, ErrRunnerUnavailable) {
		t.Fatalf("an uninstalled runner's sign-in = %v, want %v", err, ErrRunnerUnavailable)
	}
}

func TestCredentialsSetNamesOnlyNonEmptyVariables(t *testing.T) {
	environ := []string{"ANTHROPIC_API_KEY=sk", "CLAUDE_CODE_OAUTH_TOKEN=", "CODEX_API_KEY=x", "PATH=/bin"}
	if got := CredentialsSet(runnerIDClaude, environ); !slices.Equal(got, []string{"ANTHROPIC_API_KEY"}) {
		t.Fatalf("claude credentials = %q", got)
	}
	if got := CredentialsSet(runnerIDCodex, environ); !slices.Equal(got, []string{"CODEX_API_KEY"}) {
		t.Fatalf("codex credentials = %q", got)
	}
}
