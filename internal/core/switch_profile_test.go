package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
)

const switchID = "b8aaffa4-120a-4a6d-bf40-cc703dc147b8"

// switchFixture is a live claude session on profile a, with a recorded
// conversation in a's folder and an empty profile b beside it.
type switchFixture struct {
	reg      *Sessions
	boot     *stubBoot
	root     string
	profiles map[string]string
	before   *SessionState
}

func newSwitchFixture(t *testing.T, runner string) *switchFixture {
	t.Helper()
	sessions := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, r := testSessions(t, sessions, boot)
	f := &switchFixture{reg: reg, boot: boot, root: sessionDir(t, sessions, "octopus"),
		profiles: map[string]string{"a": t.TempDir(), "b": t.TempDir()}}
	reg.boot.profile = func(runnerID, id string) (config.RunnerProfile, bool) {
		dir, ok := f.profiles[id]
		return config.RunnerProfile{ID: id, Dir: dir}, ok && runnerID == runner
	}
	if err := session.UpdateManifest(f.root, func(m session.Manifest) (session.Manifest, error) {
		m.Runner, m.Profile = runner, "a"
		return m, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("octopus"); err != nil {
		t.Fatal(err)
	}
	f.before = reg.Current()
	if err := newTerm(reg, r.Emit).Start(f.before.terminal, 80, 24); err != nil {
		t.Fatal(err)
	}
	return f
}

// record writes the conversation into profile a and names it in the session.
func (f *switchFixture) record(t *testing.T) {
	t.Helper()
	dir := filepath.Join(f.profiles["a"], "projects", "-octopus")
	if err := os.MkdirAll(filepath.Join(dir, switchID, "subagents"), 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(dir, switchID+".jsonl")
	if err := os.WriteFile(transcript, []byte("root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := session.RecordConversation(f.root, session.Conversation{ID: switchID, Transcript: transcript}); err != nil {
		t.Fatal(err)
	}
}

func (f *switchFixture) manifestProfile(t *testing.T) string {
	t.Helper()
	m, err := session.Load(f.root)
	if err != nil {
		t.Fatal(err)
	}
	return m.Profile
}

// untouched fails unless the session is still the one booted before the switch,
// still running, and no supervisor was asked for since.
func (f *switchFixture) untouched(t *testing.T) {
	t.Helper()
	if agents, _ := f.boot.counts(); agents != 1 {
		t.Fatalf("asked for %d supervisors, want only the first", agents)
	}
	if f.reg.BySlug("octopus") != f.before || !f.before.alive() {
		t.Fatal("the session was stopped")
	}
}

// Catches a switch that boots before the manifest or the copy is in place, which
// resumes on the old account or on a profile without the conversation.
func TestSwitchProfileBootsOnceOnTheTargetWithTheConversationCopied(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	var bootedProfile string
	var copied bool
	f.boot.booting = func(req AgentRequest) {
		bootedProfile = f.manifestProfile(t)
		_, err := os.Stat(filepath.Join(f.profiles["b"], "projects", "-octopus", switchID+".jsonl"))
		copied = err == nil
	}
	if err := f.reg.SwitchProfile("octopus", "b"); err != nil {
		t.Fatal(err)
	}
	if agents, _ := f.boot.counts(); agents != 2 {
		t.Fatalf("asked for %d supervisors, want the first and one replacement", agents)
	}
	if !f.boot.resumed(t, f.root) || f.boot.profiles[f.root] != "b" {
		t.Fatalf("replacement booted with resume %v on profile %q, want a resume on b", f.boot.resumed(t, f.root), f.boot.profiles[f.root])
	}
	if bootedProfile != "b" || !copied {
		t.Fatalf("at boot the manifest named %q and the copy was in place: %v", bootedProfile, copied)
	}
	if _, err := os.Stat(filepath.Join(f.profiles["b"], "projects", "-octopus", switchID, "subagents")); err != nil {
		t.Fatalf("the conversation's folder did not move: %v", err)
	}
}

// Catches a failed copy leaving the session dead, or moved onto a profile that
// does not hold its conversation.
func TestSwitchProfileThatCannotCopyBootsBackOnTheOldProfile(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	if err := os.WriteFile(filepath.Join(f.profiles["b"], "projects"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.reg.SwitchProfile("octopus", "b"); err == nil {
		t.Fatal("a switch whose copy failed reported success")
	}
	if got := f.manifestProfile(t); got != "a" {
		t.Fatalf("manifest names %q after a failed copy, want a", got)
	}
	if agents, _ := f.boot.counts(); agents != 2 || f.boot.profiles[f.root] != "a" || !f.boot.resumed(t, f.root) {
		t.Fatalf("asked for %d supervisors, last on %q; want one resumed reboot on a", agents, f.boot.profiles[f.root])
	}
}

// Catches a refusal that stops the session first and leaves it down.
func TestSwitchProfileRefusesBeforeStoppingAnything(t *testing.T) {
	for name, tc := range map[string]struct {
		runner, target string
		record         bool
		want           error
	}{
		"codex session":   {"codex", "b", true, ErrProfileUnswitchable},
		"unknown profile": {"claude", "zzz", true, launch.ErrProfileUnavailable},
		"no conversation": {"claude", "b", false, ErrNoConversation},
	} {
		t.Run(name, func(t *testing.T) {
			f := newSwitchFixture(t, tc.runner)
			if tc.record {
				f.record(t)
			}
			if err := f.reg.SwitchProfile("octopus", tc.target); !errors.Is(err, tc.want) {
				t.Fatalf("SwitchProfile = %v, want %v", err, tc.want)
			}
			f.untouched(t)
			if got := f.manifestProfile(t); got != "a" {
				t.Fatalf("manifest names %q after a refusal", got)
			}
		})
	}
}

// Catches picking the profile already in use restarting the session.
func TestSwitchProfileToTheCurrentProfileDoesNothing(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	if err := f.reg.SwitchProfile("octopus", "a"); err != nil {
		t.Fatal(err)
	}
	f.untouched(t)
}
