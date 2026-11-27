package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func profiledSession(t *testing.T, root, slug, runner, profile string) string {
	t.Helper()
	dir := sessionDir(t, root, slug)
	if err := session.WriteManifest(dir, session.Manifest{Slug: slug, Mode: session.ModeAssistant,
		Runner: runner, Profile: profile}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBootResumesOnTheManifestsProfile(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	dir := profiledSession(t, root, "octopus", "claude", "work")

	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	boot.mu.Lock()
	runner, profile := boot.runners[dir], boot.profiles[dir]
	boot.mu.Unlock()
	if runner != "claude" || profile != "work" {
		t.Fatalf("booted %q on profile %q, want claude on the manifest's work", runner, profile)
	}
	if !boot.resumed(t, dir) {
		t.Fatal("the session booted without resuming its conversation")
	}
	if state := reg.BySlug("octopus"); state == nil || state.profile != "work" {
		t.Fatalf("session state does not keep the profile: %+v", state)
	}
}

func TestARefusedProfileLeavesTheSessionUnbooted(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	profiledSession(t, root, "octopus", "claude", "gone")
	cfg := &config.Config{Profiles: map[string][]config.RunnerProfile{
		"claude": {{ID: "work", Label: "Work", Dir: filepath.Join(t.TempDir(), "work")}},
	}}
	reg.boot.agent = func(req AgentRequest) (AgentCommand, error) {
		r, err := launch.WithProfile(cfg, launch.Runner{ID: req.RunnerID}, req.ProfileID)
		if err != nil {
			return AgentCommand{}, err
		}
		return AgentCommand{Argv: []string{"/bin/cat"}, RunnerID: r.ID, ProfileID: r.Profile.ID}, nil
	}

	err := reg.Show("octopus")
	if !errors.Is(err, launch.ErrProfileUnavailable) {
		t.Fatalf("Show = %v, want %v", err, launch.ErrProfileUnavailable)
	}
	if !strings.Contains(err.Error(), "claude") || !strings.Contains(err.Error(), "gone") {
		t.Fatalf("%q does not name the runner and the missing profile", err)
	}
	if reg.BySlug("octopus") != nil {
		t.Fatal("a session whose profile is gone was booted")
	}
}

// Breaks if a manifest that exists but will not load boots the workbench's runner
// on the default account instead of refusing.
func TestAnUnreadableManifestLeavesTheSessionUnbooted(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	manifest := sessionpaths.Manifest(profiledSession(t, root, "octopus", "claude", "work"))
	if err := os.WriteFile(manifest, []byte("{\"runner\": "), 0o644); err != nil {
		t.Fatal(err)
	}

	err := reg.Show("octopus")
	if !errors.Is(err, ErrManifestUnreadable) || !strings.Contains(err.Error(), manifest) {
		t.Fatalf("Show = %v, want %v naming %s", err, ErrManifestUnreadable, manifest)
	}
	if agents, _ := boot.counts(); agents != 0 || reg.BySlug("octopus") != nil {
		t.Fatalf("a session whose manifest will not load asked for %d supervisors", agents)
	}
}

// Breaks if a resume whose manifest vanished after discovery boots the
// workbench's runner on the default account.
func TestAResumeWhoseManifestVanishedLeavesTheSessionUnbooted(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	profiledSession(t, root, "octopus", "claude", "work")
	dir := reg.boot.root("octopus")
	if dir == "" {
		t.Fatal("the session was not discovered")
	}
	if err := os.Remove(sessionpaths.Manifest(dir)); err != nil {
		t.Fatal(err)
	}

	_, err := reg.start(dir, "", "", true)
	if !errors.Is(err, ErrManifestUnreadable) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("start = %v, want %v for a missing file", err, ErrManifestUnreadable)
	}
	if agents, _ := boot.counts(); agents != 0 || reg.BySlug("octopus") != nil {
		t.Fatalf("a resume with no manifest asked for %d supervisors", agents)
	}
}

// Breaks if Reload boots on the workbench's default account rather than the
// manifest's profile.
func TestReloadResumesOnTheManifestsProfile(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	dir := profiledSession(t, root, "octopus", "codex", "personal")
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	boot.mu.Lock()
	delete(boot.runners, dir)
	delete(boot.profiles, dir)
	boot.mu.Unlock()

	if err := reg.Reload("octopus"); err != nil {
		t.Fatal(err)
	}
	boot.mu.Lock()
	runner, profile := boot.runners[dir], boot.profiles[dir]
	boot.mu.Unlock()
	if runner != "codex" || profile != "personal" {
		t.Fatalf("Reload booted %q on profile %q, want codex on the manifest's personal", runner, profile)
	}
	if !boot.resumed(t, dir) {
		t.Fatal("the reloaded session started a fresh conversation")
	}
	if state := reg.BySlug("octopus"); state == nil || state.profile != "personal" {
		t.Fatalf("the reloaded session does not keep the profile: %+v", state)
	}
}

var claudeAndCodex = []assembly.Runner{
	{ID: "claude", Label: "Claude Code", Installed: true},
	{ID: "codex", Label: "Codex CLI", Installed: true},
	{ID: "opencode", Label: "OpenCode", Installed: true},
}

// testProfiles is the Agents tab over a config.json of its own, under a home
// directory of its own.
func testProfiles(t *testing.T, cfg *config.Config) (*Profiles, *stubBoot) {
	t.Helper()
	profiles, boot, _ := testProfilesClient(t, cfg)
	return profiles, boot
}

func testProfilesClient(t *testing.T, cfg *config.Config) (*Profiles, *stubBoot, *fakeClient) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if cfg.Root == "" {
		cfg.Root = t.TempDir()
	}
	writeConfigFile(t, cfg)
	boot := newStubBoot("/bin/cat")
	reg, windows, client := testSessions(t, cfg.Root, boot)
	runners := func() ([]assembly.Runner, error) {
		return assembly.WithProfiles(claudeAndCodex, cfg.Snapshot().Profiles), nil
	}
	return newProfiles(cfg, nil, runners, boot, reg, windows, client), boot, client
}

func savedProfiles(t *testing.T) map[string][]config.RunnerProfile {
	t.Helper()
	onDisk, err := config.ReadFile()
	if err != nil {
		t.Fatal(err)
	}
	return onDisk.Profiles
}

func TestAddDerivesTheIDAndCreatesThePrivateFolder(t *testing.T) {
	cfg := &config.Config{}
	profiles, _ := testProfiles(t, cfg)
	home, _ := os.UserHomeDir()
	if got := profiles.Suggest("claude", "Lifesum Work"); got != (ProfileSuggestion{ID: "lifesum-work", Dir: "~/.claude-lifesum-work"}) {
		t.Fatalf("Suggest = %+v", got)
	}

	view, err := profiles.Add(ProfileInput{Runner: "claude", Label: " Lifesum Work "})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".claude-lifesum-work")
	want := config.RunnerProfile{ID: "lifesum-work", Label: "Lifesum Work", Dir: dir}
	if got, ok := cfg.Profile("claude", "lifesum-work"); !ok || got != want {
		t.Fatalf("live profile = %+v, %v; want %+v", got, ok, want)
	}
	if got := savedProfiles(t)["claude"]; !slices.Equal(got, []config.RunnerProfile{want}) {
		t.Fatalf("config.json holds %+v, want %+v", got, want)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("profile folder = %v, %v; want a directory at 0700", info, err)
	}
	if len(view.Runners) != 2 || view.Runners[0].ID != "claude" || len(view.Runners[0].Profiles) != 1 {
		t.Fatalf("view after Add = %+v, want claude holding the profile and codex beside it", view)
	}

	if _, err := profiles.Add(ProfileInput{Runner: "claude", Label: "lifesum work"}); !errors.Is(err, ErrProfileTaken) {
		t.Fatalf("adding a taken id = %v, want %v", err, ErrProfileTaken)
	}
	if _, err := profiles.Add(ProfileInput{Runner: "claude", Label: "Other", Dir: "relative/dir"}); !errors.Is(err, ErrProfileFolder) {
		t.Fatalf("adding a relative folder = %v, want %v", err, ErrProfileFolder)
	}
	if _, err := profiles.Add(ProfileInput{Runner: "opencode", Label: "Other"}); !errors.Is(err, launch.ErrProfilesUnsupported) {
		t.Fatalf("adding an OpenCode profile = %v, want %v", err, launch.ErrProfilesUnsupported)
	}
}

// Catches the save path trimming a picked folder into its sibling.
func TestAddKeepsThePickedFolderExactly(t *testing.T) {
	profiles, _ := testProfiles(t, &config.Config{})
	base := t.TempDir()
	spaced := filepath.Join(base, "work ")
	for _, dir := range []string{filepath.Join(base, "work"), spaced} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	view, err := profiles.Add(ProfileInput{Runner: "claude", Label: "Spaced", Dir: spaced})
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Runners[0].Profiles; len(got) != 1 || got[0].Dir != spaced {
		t.Fatalf("claude's profiles = %+v, want the folder %q byte for byte", got, spaced)
	}
	if _, err := profiles.Add(ProfileInput{Runner: "claude", Label: "Blank", Dir: "  "}); !errors.Is(err, ErrProfileFolder) {
		t.Fatalf("adding a blank folder = %v, want %v", err, ErrProfileFolder)
	}
}

func TestRenameKeepsTheID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	cfg := &config.Config{Profiles: map[string][]config.RunnerProfile{"codex": {{ID: "work", Label: "Work", Dir: dir}}}}
	profiles, _ := testProfiles(t, cfg)

	if _, err := profiles.Rename("codex", "work", "Day job"); err != nil {
		t.Fatal(err)
	}
	want := config.RunnerProfile{ID: "work", Label: "Day job", Dir: dir}
	if got, _ := cfg.Profile("codex", "work"); got != want {
		t.Fatalf("live profile = %+v, want %+v", got, want)
	}
	if got := savedProfiles(t)["codex"]; !slices.Equal(got, []config.RunnerProfile{want}) {
		t.Fatalf("config.json holds %+v, want %+v", got, want)
	}
	if _, err := profiles.Rename("codex", "gone", "Other"); !errors.Is(err, launch.ErrProfileUnavailable) {
		t.Fatalf("renaming a missing profile = %v, want %v", err, launch.ErrProfileUnavailable)
	}
}

func TestDeleteLeavesTheFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: t.TempDir(), Profiles: map[string][]config.RunnerProfile{
		"claude": {{ID: "work", Label: "Work", Dir: dir}},
	}}
	profiles, _ := testProfiles(t, cfg)
	profiledSession(t, cfg.Root, "octopus", "claude", "work")
	profiledSession(t, cfg.Root, "squid", "claude", "work")
	profiledSession(t, cfg.Root, "cuttlefish", "codex", "work")

	view, err := profiles.View()
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Runners[0].Profiles; len(got) != 1 || got[0].Sessions != 2 {
		t.Fatalf("claude's profiles = %+v, want work pinned by two sessions", got)
	}
	if _, err := profiles.Delete("claude", "work"); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Profile("claude", "work"); ok || cfg.Snapshot().Profiles != nil {
		t.Fatalf("live profiles after Delete = %+v", cfg.Snapshot().Profiles)
	}
	if got := savedProfiles(t); got != nil {
		t.Fatalf("config.json still holds %+v", got)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Delete removed the folder holding the login: %v", err)
	}
}

func TestSignInNeedsAShownSession(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	cfg := &config.Config{Profiles: map[string][]config.RunnerProfile{"claude": {{ID: "work", Label: "Lifesum", Dir: dir}}}}
	profiles, boot := testProfiles(t, cfg)
	var asked config.RunnerProfile
	boot.signIn = func(runnerID string, profile config.RunnerProfile) ([]string, error) {
		asked = profile
		return []string{"/bin/echo", runnerID}, nil
	}

	if _, err := profiles.SignIn("claude", "work"); !errors.Is(err, ErrNoSessionShown) {
		t.Fatalf("SignIn with no session on screen = %v, want %v", err, ErrNoSessionShown)
	}
	if view, _ := profiles.View(); view.SessionShown {
		t.Fatal("the view says a session is shown when none is")
	}

	sessionDir(t, cfg.Root, "octopus")
	if err := profiles.sessions.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	id, err := profiles.SignIn("claude", "work")
	if err != nil {
		t.Fatal(err)
	}
	window, ok := profiles.windows.window(id)
	if !ok || window.session != profiles.sessions.Current() {
		t.Fatalf("the sign-in tab %q is not in the shown session's pane", id)
	}
	if window.opts.Label != "Sign in · Claude Code · Lifesum" || !slices.Equal(window.opts.Command, []string{"/bin/echo", "claude"}) {
		t.Fatalf("sign-in tab = %+v", window.opts)
	}
	if asked != cfg.Snapshot().Profiles["claude"][0] {
		t.Fatalf("signed in on %+v, want the work profile", asked)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("SignIn left the profile folder %v, %v; want it created at 0700", info, err)
	}
}

func TestStatusProbesTheDefaultAndEveryNamedProfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	cfg := &config.Config{Profiles: map[string][]config.RunnerProfile{"claude": {{ID: "work", Label: "Work", Dir: dir}}}}
	profiles, boot := testProfiles(t, cfg)
	boot.probe = func(_ context.Context, runnerID string, profile config.RunnerProfile) launch.SignIn {
		if runnerID == "claude" && profile.ID == "work" {
			return launch.SignIn{State: launch.SignInSignedIn, Email: "k@example.com"}
		}
		return launch.SignIn{State: launch.SignInSignedOut}
	}
	got, err := profiles.Status()
	if err != nil {
		t.Fatal(err)
	}
	want := []ProfileStatus{
		{Runner: "claude", State: launch.SignInSignedOut},
		{Runner: "claude", Profile: "work", State: launch.SignInSignedIn, Email: "k@example.com"},
		{Runner: "codex", State: launch.SignInSignedOut},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Status = %+v, want %+v", got, want)
	}
}

func TestViewWarnsOfACredentialInTheEnvironment(t *testing.T) {
	profiles, _ := testProfiles(t, &config.Config{})
	profiles.environ = func() []string { return []string{"ANTHROPIC_API_KEY=sk-test"} }
	view, err := profiles.View()
	if err != nil {
		t.Fatal(err)
	}
	claude, codex := view.Runners[0], view.Runners[1]
	if !slices.Equal(claude.Credentials, []string{"ANTHROPIC_API_KEY"}) || !strings.Contains(claude.Warning, "ANTHROPIC_API_KEY") {
		t.Fatalf("claude = %+v, want a warning naming ANTHROPIC_API_KEY", claude)
	}
	if len(codex.Credentials) != 0 || codex.Warning != "" {
		t.Fatalf("codex = %+v, want no warning", codex)
	}
}

func TestSettingsSaveKeepsProfilesAddedSinceLoad(t *testing.T) {
	cfg := &config.Config{Orgs: []string{"acme"}}
	profiles, _ := testProfiles(t, cfg)
	settings := testSettings(t, cfg, nil, nil, nil)
	loaded := settings.Load()

	if _, err := profiles.Add(ProfileInput{Runner: "claude", Label: "Lifesum"}); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.Save(SettingsInput{Orgs: loaded.Orgs, Root: loaded.Root, Editor: "vim"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Profile("claude", "lifesum"); !ok {
		t.Fatal("a settings save dropped a profile added since the form loaded from the live config")
	}
	if got := savedProfiles(t)["claude"]; len(got) != 1 || got[0].ID != "lifesum" {
		t.Fatalf("a settings save dropped a profile added since the form loaded from config.json: %+v", got)
	}
}

// threeProbes is the Agents tab over claude's default, a named claude profile
// and codex's default, with a short probe deadline.
func threeProbes(t *testing.T, deadline time.Duration) (*Profiles, *stubBoot) {
	t.Helper()
	cfg := &config.Config{Profiles: map[string][]config.RunnerProfile{
		"claude": {{ID: "work", Label: "Work", Dir: filepath.Join(t.TempDir(), "work")}},
	}}
	profiles, boot := testProfiles(t, cfg)
	profiles.probeTimeout = deadline
	return profiles, boot
}

func TestStatusReadsAProbeStillRunningAtTheDeadlineAsUnknown(t *testing.T) {
	const deadline = 100 * time.Millisecond
	profiles, boot := threeProbes(t, deadline)
	released := make(chan struct{})
	t.Cleanup(func() { close(released) })
	boot.probe = func(context.Context, string, config.RunnerProfile) launch.SignIn {
		select {
		case <-released:
		case <-time.After(20 * deadline):
		}
		return launch.SignIn{State: launch.SignInSignedIn}
	}

	started := time.Now()
	got, err := profiles.Status()
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 10*deadline {
		t.Fatalf("Status waited %s on probes that ignore the %s deadline", elapsed, deadline)
	}
	if len(got) != 3 {
		t.Fatalf("Status = %+v, want three probes", got)
	}
	for _, status := range got {
		if status.State != launch.SignInUnknown {
			t.Fatalf("a probe still running at the deadline read %+v, want unknown", status)
		}
	}
}

func TestStatusProbesConcurrentlyWithinOneDeadline(t *testing.T) {
	const deadline, each = 300 * time.Millisecond, 150 * time.Millisecond
	profiles, boot := threeProbes(t, deadline)
	boot.probe = func(ctx context.Context, _ string, _ config.RunnerProfile) launch.SignIn {
		select {
		case <-time.After(each):
			return launch.SignIn{State: launch.SignInSignedIn}
		case <-ctx.Done():
			return launch.SignIn{State: launch.SignInUnknown}
		}
	}

	got, err := profiles.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range got {
		if status.State != launch.SignInSignedIn {
			t.Fatalf("three %s probes under a %s deadline read %+v: they ran one after another", each, deadline, got)
		}
	}
}

func TestViewMarksPinnedCountsItCouldNotReadAsUncounted(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(root, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: root, Profiles: map[string][]config.RunnerProfile{
		"claude": {{ID: "work", Label: "Work", Dir: filepath.Join(t.TempDir(), "work")}},
	}}
	profiles, _ := testProfiles(t, cfg)
	view, err := profiles.View()
	if err != nil {
		t.Fatal(err)
	}
	if got := view.Runners[0].Profiles[0]; !got.Uncounted {
		t.Fatalf("a sessions root that cannot be read gave %+v, which reads as no pinned sessions", got)
	}
}

// The runner list repeats a runner once per named profile; the tab must not.
func TestViewAndStatusListEachRunnerOnceWhenItHasProfiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "work")
	cfg := &config.Config{Profiles: map[string][]config.RunnerProfile{"claude": {
		{ID: "work", Label: "Work", Dir: dir},
		{ID: "lifesum", Label: "Lifesum", Dir: dir + "-lifesum"},
	}}}
	profiles, boot := testProfiles(t, cfg)
	var probes atomic.Int32
	boot.probe = func(context.Context, string, config.RunnerProfile) launch.SignIn {
		probes.Add(1)
		return launch.SignIn{State: launch.SignInSignedOut}
	}
	view, err := profiles.View()
	if err != nil {
		t.Fatal(err)
	}
	var groups []string
	for _, g := range view.Runners {
		groups = append(groups, fmt.Sprintf("%s:%d", g.ID, len(g.Profiles)))
	}
	if want := []string{"claude:2", "codex:0"}; !slices.Equal(groups, want) {
		t.Fatalf("View groups = %q, want %q", groups, want)
	}
	got, err := profiles.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || probes.Load() != 4 {
		t.Fatalf("Status answered %d rows from %d probes, want 4 of each: %+v", len(got), probes.Load(), got)
	}
}

func TestViewOffersTheFolderPickerOnlyWhereTheClientHasOne(t *testing.T) {
	profiles, _, client := testProfilesClient(t, &config.Config{})
	if view, _ := profiles.View(); view.CanChooseFolder {
		t.Fatal("the view offers a folder picker the client cannot show")
	}
	client.choose = func(bool) (string, error) { return "/picked", nil }
	if view, _ := profiles.View(); !view.CanChooseFolder {
		t.Fatal("the view hides the folder picker the client has")
	}
}

func TestChooseFolderPassesTheClientsAnswerAndACancelThrough(t *testing.T) {
	profiles, _, client := testProfilesClient(t, &config.Config{})
	answers := []string{"/Volumes/work/claude", ""}
	client.choose = func(bool) (string, error) {
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}
	if got, err := profiles.ChooseFolder(); err != nil || got != "/Volumes/work/claude" {
		t.Fatalf("ChooseFolder = %q, %v; want the picked folder", got, err)
	}
	if got, err := profiles.ChooseFolder(); err != nil || got != "" {
		t.Fatalf("a cancelled ChooseFolder = %q, %v; want the empty string", got, err)
	}
	client.choose = nil
	if _, err := profiles.ChooseFolder(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ChooseFolder without a picker = %v, want %v", err, ErrUnavailable)
	}
}
