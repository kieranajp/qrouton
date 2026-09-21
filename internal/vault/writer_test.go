package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestImmutableWriteRejectsSpecialFilesAndPreservesExisting(t *testing.T) {
	directory, outside := t.TempDir(), t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err = os.WriteFile(filepath.Join(outside, "untouched"), []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Join(outside, "untouched"), filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}
	if err = syscall.Mkfifo(filepath.Join(directory, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"link", "pipe", "../escape"} {
		if err = writeOnce(root, name, []byte("incoming")); err == nil {
			t.Fatal("unsafe write", name)
		}
	}
	if err = writeOnce(root, "file", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err = writeOnce(root, "file", []byte("second")); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(directory, "file"))
	if string(b) != "first" {
		t.Fatal("overwrite")
	}
	b, _ = os.ReadFile(filepath.Join(outside, "untouched"))
	if string(b) != "outside" {
		t.Fatal("escape")
	}
}
func TestPublicationPolicyIsSessionScopedAndBatchPreflighted(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, state)
	request := importRequest(t, "Evidence\n")
	request.Sources[0].Session = &ImportSession{Key: "allowed", Content: []byte("allowed manifest")}
	second := fixtureDocument()
	second.ID = "session/R2"
	b, _ := Encode(second)
	request.Sources = append(request.Sources, ImportSource{Key: "blocked", Name: "R2.md", Content: b, Session: &ImportSession{Key: "blocked-session", Content: []byte("blocked manifest")}})
	p := previewAccepted(t, s, request)
	if err := s.SetPublicationDisabled(context.Background(), "blocked-session", true); err != nil {
		t.Fatal(err)
	}
	sources := map[string][]byte{"source": request.Sources[0].Content, "allowed": request.Sources[0].Session.Content, "blocked": b, "blocked-session": request.Sources[1].Session.Content}
	report, err := s.ConfirmImport(context.Background(), p.ID, []string{"source", "blocked"}, func(key string) ([]byte, error) { return sources[key], nil })
	if !errors.Is(err, ErrPublicationDisabled) || len(report.Entries) != 0 {
		t.Fatal(report, err)
	}
	report, err = s.ImportStatus()
	if err != nil || len(report.Entries) != 0 {
		t.Fatal("preflight enqueued partial batch", report, err)
	}
	report, err = s.ConfirmImport(context.Background(), p.ID, []string{"source"}, func(key string) ([]byte, error) { return sources[key], nil })
	if err != nil || len(report.Entries) != 1 {
		t.Fatal(report, err)
	}
	waitImport(t, s, ImportWritten)
}
func TestPublicationNamespaceSymlinkIsNotFollowed(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, t.TempDir())
	request := importRequest(t, "Evidence\n")
	p := previewAccepted(t, s, request)
	if err := os.Symlink(outside, filepath.Join(root, "session")); err != nil {
		t.Fatal(err)
	}
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportUnavailable)
	names, err := os.ReadDir(outside)
	if err != nil || len(names) != 0 {
		t.Fatal("namespace escaped", names, err)
	}
}
