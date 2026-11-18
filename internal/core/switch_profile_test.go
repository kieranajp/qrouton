package core

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
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

func newSwitchFixture(t *testing.T, runner string, argv ...string) *switchFixture {
	t.Helper()
	sessions := t.TempDir()
	if len(argv) == 0 {
		argv = []string{"/bin/cat"}
	}
	boot := newStubBoot(argv...)
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
func (f *switchFixture) record(t *testing.T) { f.recordID(t, switchID) }

func (f *switchFixture) recordID(t *testing.T, id string) {
	t.Helper()
	if err := session.RecordConversation(f.root, f.write(t, id)); err != nil {
		t.Fatal(err)
	}
}

// write puts a conversation in profile a without naming it in the session.
func (f *switchFixture) write(t *testing.T, id string) session.Conversation {
	t.Helper()
	dir := filepath.Join(f.profiles["a"], "projects", "-octopus")
	if err := os.MkdirAll(filepath.Join(dir, id, "subagents"), 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(dir, id+".jsonl")
	if err := os.WriteFile(transcript, []byte("root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return session.Conversation{ID: id, Transcript: transcript}
}

// copied reports whether profile b holds the conversation id.
func (f *switchFixture) copied(id string) bool {
	_, err := os.Stat(filepath.Join(f.profiles["b"], "projects", "-octopus", id+".jsonl"))
	return err == nil
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
	before, _ := session.ReadConversation(f.root)
	if err := os.WriteFile(filepath.Join(f.profiles["b"], "projects"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.reg.SwitchProfile("octopus", "b"); err == nil {
		t.Fatal("a switch whose copy failed reported success")
	}
	if got := f.manifestProfile(t); got != "a" {
		t.Fatalf("manifest names %q after a failed copy, want a", got)
	}
	if got, _ := session.ReadConversation(f.root); got != before {
		t.Fatalf("the record names %+v after a failed copy, want %+v", got, before)
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

// Catches a switch copying while the runner the supervisor started may still be
// writing the conversation, because only the supervisor was waited for.
func TestSwitchProfileCopiesOnlyOnceTheRunnerHasExited(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runner.pid")
	f := newSwitchFixture(t, "claude", "/bin/sh", "-c", "(trap '' TERM HUP; exec sleep 300) & echo $! > "+marker+"; wait")
	f.record(t)
	f.before.mu.Lock()
	f.before.process.grace = 50 * time.Millisecond
	f.before.mu.Unlock()
	runner := readPID(t, marker)
	t.Cleanup(func() { _ = syscall.Kill(runner, syscall.SIGKILL) })
	var runnerAlive bool
	f.boot.booting = func(AgentRequest) { runnerAlive = executing(t, runner) }
	if err := f.reg.SwitchProfile("octopus", "b"); err != nil {
		t.Fatal(err)
	}
	if runnerAlive {
		t.Fatal("the switch copied and rebooted while the previous runner was still running")
	}
}

// Catches a switch copying the conversation while a runner it could not kill
// may still be writing it.
func TestSwitchProfileWhoseDrainFailsBootsBackWithoutCopying(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "runner.pid")
	f := newSwitchFixture(t, "claude", "/bin/sh", "-c", "(trap '' TERM HUP; exec sleep 300) & echo $! > "+marker+"; wait")
	f.record(t)
	runner := readPID(t, marker)
	t.Cleanup(func() { _ = syscall.Kill(runner, syscall.SIGKILL) })
	f.before.mu.Lock()
	f.before.process.grace = 50 * time.Millisecond
	f.before.process.signal = ignoreSIGKILL
	f.before.mu.Unlock()
	if err := f.reg.SwitchProfile("octopus", "b"); !errors.Is(err, ErrGroupNotDrained) {
		t.Fatalf("SwitchProfile = %v, want %v", err, ErrGroupNotDrained)
	}
	if _, err := os.Stat(filepath.Join(f.profiles["b"], "projects")); !os.IsNotExist(err) {
		t.Fatalf("the conversation was copied: %v", err)
	}
	if agents, _ := f.boot.counts(); agents != 2 || f.manifestProfile(t) != "a" || f.boot.profiles[f.root] != "a" || !f.boot.resumed(t, f.root) {
		t.Fatalf("asked for %d supervisors, last on %q, manifest on %q; want one resumed reboot on a",
			agents, f.boot.profiles[f.root], f.manifestProfile(t))
	}
}

// Catches a switch stopping nothing yet still copying and rewriting the manifest
// under a supervisor another workbench owns.
func TestSwitchProfileRefusesASupervisorFromAnotherWorkbench(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	if err := f.reg.Unload("octopus"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionpaths.AgentPID(f.root), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.reg.SwitchProfile("octopus", "b"); err == nil {
		t.Fatal("a switch went ahead under another workbench's supervisor")
	}
	if agents, _ := f.boot.counts(); agents != 1 || f.manifestProfile(t) != "a" {
		t.Fatalf("asked for %d supervisors and left the manifest on %q", agents, f.manifestProfile(t))
	}
	if _, err := os.Stat(filepath.Join(f.profiles["b"], "projects")); !os.IsNotExist(err) {
		t.Fatalf("the conversation was copied: %v", err)
	}
}

// Catches a switch to the default profile writing anywhere but the folder the
// runner will read once no profile moves it.
func TestSwitchProfileToTheDefaultLandsInTheInheritedFolder(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	home := t.TempDir()
	f.reg.boot.environ = func() []string { return []string{"HOME=" + home} }
	if err := f.reg.SwitchProfile("octopus", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "projects", "-octopus", switchID+".jsonl")); err != nil {
		t.Fatalf("the default profile's folder holds no copy: %v", err)
	}
	if got := f.manifestProfile(t); got != "" {
		t.Fatalf("manifest names %q, want the default", got)
	}
}

// Catches the switch copying one conversation while the replacement resumes
// another, because a writer named a new one after the switch first read it.
func TestSwitchProfileBootsTheConversationItCopied(t *testing.T) {
	const later = "c0ffee00-0000-4000-8000-000000000000"
	for name, tc := range map[string]struct {
		teardown, boot func(t *testing.T, f *switchFixture)
	}{
		"write during teardown": {teardown: func(t *testing.T, f *switchFixture) { f.recordID(t, later) }},
		"root hook landing after the drain": {boot: func(t *testing.T, f *switchFixture) {
			f.before.keepConversation(claudeEventWith(1, f.write(t, later)))
		}},
	} {
		t.Run(name, func(t *testing.T) {
			f := newSwitchFixture(t, "claude")
			f.record(t)
			f.before.agents.begin("claude", 1)
			teardown := f.reg.boot.teardown
			f.reg.boot.teardown = func(state *SessionState) {
				if tc.teardown != nil {
					tc.teardown(t, f)
				}
				teardown(state)
			}
			var resumes string
			var held bool
			f.boot.booting = func(AgentRequest) {
				if tc.boot != nil {
					tc.boot(t, f)
				}
				recorded, _ := session.ReadConversation(f.root)
				resumes, held = recorded.ID, f.copied(recorded.ID)
			}
			if err := f.reg.SwitchProfile("octopus", "b"); err != nil {
				t.Fatal(err)
			}
			if !held {
				t.Fatalf("the replacement resumes %q, which the switch did not copy", resumes)
			}
		})
	}
}

func claudeEventWith(generation uint64, c session.Conversation) workbench.AgentEventRequest {
	return claudeEvent(agentevent.HookUserPromptSubmit, func(e *workbench.AgentEventRequest) {
		e.Generation, e.ConversationID, e.TranscriptPath = generation, c.ID, c.Transcript
	})
}

// Catches the record still pointing into the profile a switch left, which a
// second switch then copies from even though that folder may be gone.
func TestSwitchProfileRecordsTheCopyItCommitted(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	f.profiles["c"] = t.TempDir()
	if err := f.reg.SwitchProfile("octopus", "b"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(f.profiles["b"], "projects", "-octopus", switchID+".jsonl")
	if got, _ := session.ReadConversation(f.root); got.Transcript != want {
		t.Fatalf("the record names %q after the switch, want the copy at %q", got.Transcript, want)
	}
	if err := os.RemoveAll(f.profiles["a"]); err != nil {
		t.Fatal(err)
	}
	if err := f.reg.SwitchProfile("octopus", "c"); err != nil {
		t.Fatalf("a second switch after the first profile's folder went: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.profiles["c"], "projects", "-octopus", switchID+".jsonl")); err != nil {
		t.Fatal(err)
	}
}

// Catches a switch that stops after copying leaving the record on a copy the
// manifest never committed to, or the manifest on a profile the record does not name.
func TestSwitchProfileThatCannotRecordTheCopyBootsBackOnTheOldProfile(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	before, _ := session.ReadConversation(f.root)
	dir := sessionpaths.Dir(f.root)
	teardown := f.reg.boot.teardown
	f.reg.boot.teardown = func(state *SessionState) {
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		teardown(state)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if err := f.reg.SwitchProfile("octopus", "b"); err == nil {
		t.Fatal("a switch that could not record its copy reported success")
	}
	if got, _ := session.ReadConversation(f.root); got != before {
		t.Fatalf("the record names %+v, want it left on %+v", got, before)
	}
	if got := f.manifestProfile(t); got != "a" || f.boot.profiles[f.root] != "a" || !f.boot.resumed(t, f.root) {
		t.Fatalf("manifest on %q, rebooted on %q; want a resumed reboot on a", got, f.boot.profiles[f.root])
	}
}

// Catches a record left on the old profile by an interrupted switch sending the
// next switch to a folder that has gone, when the committed profile holds the copy.
func TestSwitchProfileCopiesFromTheCommittedProfile(t *testing.T) {
	f := newSwitchFixture(t, "claude")
	f.record(t)
	if _, err := launch.CopyConversation("claude", filepath.Join(f.profiles["a"], "projects", "-octopus", switchID+".jsonl"), f.profiles["b"]); err != nil {
		t.Fatal(err)
	}
	if err := session.UpdateManifest(f.root, func(m session.Manifest) (session.Manifest, error) {
		m.Profile = "b"
		return m, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.profiles["a"]); err != nil {
		t.Fatal(err)
	}
	f.profiles["c"] = t.TempDir()
	if err := f.reg.SwitchProfile("octopus", "c"); err != nil {
		t.Fatalf("switch from the committed profile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.profiles["c"], "projects", "-octopus", switchID+".jsonl")); err != nil {
		t.Fatal(err)
	}
}
