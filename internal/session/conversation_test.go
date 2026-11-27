package session

import (
	"os"
	"strconv"
	"testing"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

const testConversationID = "b8aaffa4-120a-4a6d-bf40-cc703dc147b8"

func testConversation() Conversation {
	return Conversation{ID: testConversationID, Transcript: "/home/user/.claude/projects/-tmp-s/" + testConversationID + ".jsonl"}
}

// Catches a record that is written but cannot be read back as the same pair.
func TestConversationRoundTrips(t *testing.T) {
	root := t.TempDir()
	if err := RecordConversation(root, testConversation()); err != nil {
		t.Fatal(err)
	}
	got, ok := ReadConversation(root)
	if !ok || got != testConversation() {
		t.Fatalf("ReadConversation = %+v, %v, want %+v", got, ok, testConversation())
	}
}

// Catches a session with no record resuming some conversation anyway.
func TestConversationMissingReadsAsNone(t *testing.T) {
	if got, ok := ReadConversation(t.TempDir()); ok {
		t.Fatalf("ReadConversation on an empty session = %+v", got)
	}
}

// Catches a hand-edited record smuggling a flag into argv or a path outside the
// profile into the switch's copy.
func TestConversationRejectsAnIDOrTranscriptARunnerNeverWrites(t *testing.T) {
	good := testConversation()
	for name, c := range map[string]Conversation{
		"separator in id":         {ID: "a/b", Transcript: "/p/a/b.jsonl"},
		"dot-dot id":              {ID: "..", Transcript: "/p/...jsonl"},
		"flag id":                 {ID: "--help", Transcript: "/p/--help.jsonl"},
		"relative transcript":     {ID: good.ID, Transcript: "projects/x/" + good.ID + ".jsonl"},
		"unclean transcript":      {ID: good.ID, Transcript: "/p/x/../" + good.ID + ".jsonl"},
		"another id's transcript": {ID: good.ID, Transcript: "/p/x/other.jsonl"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(sessionpaths.Dir(root), dirMode); err != nil {
				t.Fatal(err)
			}
			raw := `{"id":` + strconv.Quote(c.ID) + `,"transcript":` + strconv.Quote(c.Transcript) + `}`
			if err := os.WriteFile(sessionpaths.Conversation(root), []byte(raw), fileMode); err != nil {
				t.Fatal(err)
			}
			if got, ok := ReadConversation(root); ok {
				t.Fatalf("ReadConversation accepted %+v", got)
			}
			if err := RecordConversation(root, c); err == nil {
				t.Fatalf("RecordConversation accepted %+v", c)
			}
		})
	}
}

// Catches a fresh launch failing because there was nothing to clear.
func TestConversationClearTwiceSucceeds(t *testing.T) {
	root := t.TempDir()
	if err := RecordConversation(root, testConversation()); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := ClearConversation(root); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(sessionpaths.Conversation(root)); !os.IsNotExist(err) {
		t.Fatalf("record still on disk after clear: %v", err)
	}
}
