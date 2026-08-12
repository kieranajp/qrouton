package desktop

import (
	"context"
	"testing"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func recordedWindows(t *testing.T, dir string) []session.WindowRecord {
	t.Helper()
	manifest, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return manifest.Windows
}

// Decision 13: qrouton.json records which windows were open. The workbench's own
// windows stay out of it — a resume opens the conversation and the shell without
// being told — and the session ending is not the user closing them.
func TestTheManifestRecordsTheAgentsWindows(t *testing.T) {
	r := newFakeRenderer()
	opts, boot := testOptions(t)
	opts.Shell = func(dir string) []string { return []string{"/bin/cat", dir} }
	if err := session.WriteManifest(opts.SessionRoot, session.Manifest{Slug: "recorded", Name: "Recorded"}); err != nil {
		t.Fatal(err)
	}
	_, term, windows := testWorkbench(t, r, r.Emit)

	done := startWorkbench(t, r, term, windows, opts)
	conversation := <-r.opened

	socket := boot.socket(t, opts.SessionRoot)
	host, err := (workbench.Handle{Socket: socket, SessionRoot: opts.SessionRoot}).WindowHost()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: opts.SessionRoot, Command: []string{"/bin/cat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	<-r.opened

	waitFor(t, "the window to be recorded", func() bool { return len(recordedWindows(t, opts.SessionRoot)) == 1 })
	record := recordedWindows(t, opts.SessionRoot)[0]
	if record.Label != "▶ dev" || record.Kind != string(workbench.KindTerminal) || record.Cwd != opts.SessionRoot {
		t.Fatalf("recorded window = %+v", record)
	}
	if len(record.Command) != 1 || record.Command[0] != "/bin/cat" {
		t.Fatalf("recorded command = %v, which is what a replay would put in the input line", record.Command)
	}

	if err := host.Close(ctx, id); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the record to shrink", func() bool { return len(recordedWindows(t, opts.SessionRoot)) == 0 })

	if _, err := host.Open(ctx, workbench.WindowOptions{
		Kind: workbench.KindDocument, Label: "◆ diff", Content: "@@ -1 +1 @@",
	}); err != nil {
		t.Fatal(err)
	}
	<-r.opened
	waitFor(t, "the document to be recorded", func() bool { return len(recordedWindows(t, opts.SessionRoot)) == 1 })

	conversation.OnClose()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := recordedWindows(t, opts.SessionRoot); len(got) != 1 {
		t.Fatalf("ending the session rewrote the record as %+v", got)
	}
}

// A workbench that opens before onboarding has chosen a session has nowhere to
// write, and must not treat that as a failure.
func TestRecordingWaitsForASession(t *testing.T) {
	r := newFakeRenderer()
	reg := newSessions()
	owner := reg.add("", []string{"/bin/cat"}, nil)
	reg.reveal(owner)
	windows := newWindows(r, r.Emit, false, reg)
	record := &windowRecorder{windows: windows}
	windows.observe(record.save)

	if _, err := windows.openWindow(owner, workbench.WindowOptions{
		Kind: workbench.KindTerminal, Label: "▶ dev", Cwd: t.TempDir(), Command: []string{"/bin/cat"},
	}); err != nil {
		t.Fatal(err)
	}
	<-r.opened
	t.Cleanup(windows.stopAll)

	dir := t.TempDir()
	if err := reg.adopt(dir, false); err != nil {
		t.Fatal(err)
	}
	record.save(owner)
	if _, err := session.Load(dir); err == nil {
		t.Fatal("recording invented a manifest for a directory that is not a session")
	}
}
