package launch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
)

const switchedID = "b8aaffa4-120a-4a6d-bf40-cc703dc147b8"

// claudeTranscript writes a conversation under a profile's project folder the
// way Claude lays it out, with a subagent transcript beside it when withFolder.
func claudeTranscript(t *testing.T, stateDir, project string, withFolder bool) string {
	t.Helper()
	dir := filepath.Join(stateDir, claudeProjectsDirName, project)
	if err := os.MkdirAll(filepath.Join(dir, switchedID, "subagents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if !withFolder {
		if err := os.RemoveAll(filepath.Join(dir, switchedID)); err != nil {
			t.Fatal(err)
		}
	} else if err := os.WriteFile(filepath.Join(dir, switchedID, "subagents", "agent-a1.jsonl"), []byte("child\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(dir, switchedID+".jsonl")
	if err := os.WriteFile(transcript, []byte("root\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return transcript
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Catches qrouton re-encoding the session root itself instead of reusing the
// folder Claude chose, which a resume on the target profile would not find.
func TestCopyConversationLandsUnderTheTranscriptsOwnProjectFolder(t *testing.T) {
	const project = "-Users-me-Work-odd_name-v2"
	transcript := claudeTranscript(t, t.TempDir(), project, true)
	target := t.TempDir()
	if err := CopyConversation(runnerIDClaude, transcript, target); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(target, claudeProjectsDirName, project)
	if got := readFile(t, filepath.Join(dir, switchedID+".jsonl")); got != "root\n" {
		t.Fatalf("copied transcript = %q", got)
	}
	if got := readFile(t, filepath.Join(dir, switchedID, "subagents", "agent-a1.jsonl")); got != "child\n" {
		t.Fatalf("copied subagent transcript = %q", got)
	}
}

// Catches os.CopyFS refusing to overwrite, which fails every switch back to a
// profile that already holds an older copy.
func TestCopyConversationReplacesAnOlderCopy(t *testing.T) {
	transcript := claudeTranscript(t, t.TempDir(), "-s", true)
	target := t.TempDir()
	stale := filepath.Join(target, claudeProjectsDirName, "-s", switchedID, "subagents")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"agent-a1.jsonl": "old\n", "agent-gone.jsonl": "old\n"} {
		if err := os.WriteFile(filepath.Join(stale, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyConversation(runnerIDClaude, transcript, target); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(stale, "agent-a1.jsonl")); got != "child\n" {
		t.Fatalf("subagent transcript after a second copy = %q, want the newer one", got)
	}
	if _, err := os.Stat(filepath.Join(stale, "agent-gone.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("the older copy survived the replacement: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(filepath.Dir(stale)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("project folder holds %d entries after the swap, want the transcript and its folder", len(entries))
	}
}

// Catches a conversation with no subagents or tool results failing to switch.
func TestCopyConversationCopiesATranscriptWithNoFolderAlone(t *testing.T) {
	transcript := claudeTranscript(t, t.TempDir(), "-s", false)
	target := t.TempDir()
	if err := CopyConversation(runnerIDClaude, transcript, target); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(target, claudeProjectsDirName, "-s", switchedID+".jsonl")); got != "root\n" {
		t.Fatalf("copied transcript = %q", got)
	}
}

// Catches a switch to the default profile writing into ~/.claude while the
// runner actually reads the CLAUDE_CONFIG_DIR qrouton inherited.
func TestStateDirOfTheDefaultProfileFollowsTheInheritedEnvironment(t *testing.T) {
	named := config.RunnerProfile{ID: "work", Dir: "/profiles/work"}
	for name, tc := range map[string]struct {
		profile config.RunnerProfile
		environ []string
		want    string
	}{
		"named":             {named, []string{"CLAUDE_CONFIG_DIR=/inherited", "HOME=/home/me"}, "/profiles/work"},
		"default inherited": {config.RunnerProfile{}, []string{"HOME=/home/me", "CLAUDE_CONFIG_DIR=/inherited"}, "/inherited"},
		"default home":      {config.RunnerProfile{}, []string{"HOME=/home/me"}, "/home/me/.claude"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := StateDir(runnerIDClaude, tc.profile, tc.environ)
			if err != nil || got != tc.want {
				t.Fatalf("StateDir = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}

// Catches the rail offering, or the switch attempting, a move for a runner
// whose transcripts qrouton cannot carry.
func TestOnlyClaudeSwitchesProfiles(t *testing.T) {
	for _, spec := range runnerSpecs {
		if got := SwitchesProfiles(spec.ID); got != (spec.ID == runnerIDClaude) {
			t.Errorf("SwitchesProfiles(%q) = %v", spec.ID, got)
		}
	}
	if err := CopyConversation(runnerIDCodex, "/p/-s/x.jsonl", t.TempDir()); err == nil {
		t.Fatal("codex copied a conversation")
	}
}
