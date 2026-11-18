package session

import (
	"os"
	"slices"
	"testing"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// A second add before any runner takes the queue must not discard the first.
func TestASecondNoticeKeepsTheFirst(t *testing.T) {
	dir := t.TempDir()
	for _, notice := range []string{"added org/svc", "added org/web\nacross two lines"} {
		if err := QueueAgentNotice(dir, notice); err != nil {
			t.Fatal(err)
		}
	}
	got, err := TakeAgentNotices(dir)
	if want := []string{"added org/svc", "added org/web\nacross two lines"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("notices = %q, %v; want %q", got, err, want)
	}
	if again, err := TakeAgentNotices(dir); err != nil || len(again) != 0 {
		t.Fatalf("second take = %q, %v; want nothing", again, err)
	}
}

// A notice queued by a qrouton from before the queue was JSON lines is plain
// text, and must still reach the agent intact.
func TestALegacyNoticeFileIsTakenWhole(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(sessionpaths.Dir(dir), dirMode); err != nil {
		t.Fatal(err)
	}
	legacy := "qrouton: session repositories changed — added org/svc.\nRe-read qrouton.json."
	if err := os.WriteFile(sessionpaths.AgentNotice(dir), []byte(legacy), privateFileMode); err != nil {
		t.Fatal(err)
	}
	got, err := TakeAgentNotices(dir)
	if err != nil || !slices.Equal(got, []string{legacy}) {
		t.Fatalf("notices = %q, %v; want the legacy file whole", got, err)
	}
}
