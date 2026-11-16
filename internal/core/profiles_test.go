package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
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
