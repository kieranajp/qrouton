package vault

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func importService(t *testing.T, profiles []Profile, state string) *Service {
	t.Helper()
	s, err := NewService(profiles, map[string]string{"team": profiles[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureImports(state); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func importRequest(t *testing.T, body string) ImportRequest {
	t.Helper()
	d := fixtureDocument()
	d.Body = body
	b, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	return ImportRequest{Sources: []ImportSource{{Key: "source", Name: "R1.md", Content: b}}}
}
func previewAccepted(t *testing.T, s *Service, request ImportRequest) ImportPreview {
	t.Helper()
	p, err := s.PreviewImport(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if p.Accepted != len(request.Sources) {
		t.Fatalf("repairs: %+v", p.Entries)
	}
	return p
}
func confirmPreview(t *testing.T, s *Service, p ImportPreview, request ImportRequest) ImportReport {
	t.Helper()
	content := map[string][]byte{}
	keys := []string{}
	for _, source := range request.Sources {
		content[source.Key] = source.Content
		keys = append(keys, source.Key)
		if source.Session != nil {
			content[source.Session.Key] = source.Session.Content
		}
	}
	report, err := s.ConfirmImport(context.Background(), p.ID, keys, func(key string) ([]byte, error) { return content[key], nil })
	if err != nil {
		t.Fatal(err)
	}
	return report
}
func waitImport(t *testing.T, s *Service, state string) ImportReport {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, err := s.ImportStatus()
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Entries) > 0 {
			all := true
			for _, j := range r.Entries {
				all = all && j.State == state
			}
			if all {
				return r
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	r, err := s.ImportStatus()
	t.Fatalf("wanted %s: %+v %v", state, r, err)
	return r
}
func TestLegacyPreviewFixtureConversion(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Key, Name, Body string
		Accepted        bool
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	root, state := t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, state)
	session := &ImportSession{Key: "manifest", Content: []byte("selected historical manifest"), Slug: "session", Workstream: "work", Repositories: fixtureDocument().Repos}
	request := ImportRequest{}
	for _, row := range rows {
		request.Sources = append(request.Sources, ImportSource{Key: row.Key, Name: row.Name, Content: []byte(row.Body), Session: session})
	}
	preview, err := s.PreviewImport(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Accepted != 2 || preview.RepairRequired != 3 || preview.Excluded != 0 {
		t.Fatalf("conversion counts: %+v", preview)
	}
	t.Logf("representative fixture conversion: accepted=%d repair=%d excluded=%d total=%d", preview.Accepted, preview.RepairRequired, preview.Excluded, len(rows))
	for _, entry := range preview.Entries {
		for _, row := range rows {
			if entry.Key == row.Key && (len(entry.Repairs) == 0) != row.Accepted {
				t.Fatalf("%s: %+v", row.Key, entry.Repairs)
			}
		}
	}
	store, err := os.OpenRoot(filepath.Join(state, importDirectoryName))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	record, err := loadImportRecord(store, preview.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(record.Sources[0].Content) != rows[0].Body || record.Hashes[rows[0].Key] != contentHash(rows[0].Body) {
		t.Fatal("source archive changed")
	}
	for _, entry := range preview.Entries {
		if entry.Key == "complete" && (entry.Unsupported["extra"] == "" || len(entry.Warnings) != 1) {
			t.Fatal("unsupported metadata lost")
		}
	}
}
func TestImportSourceAndManifestRevalidation(t *testing.T) {
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: t.TempDir()}}, t.TempDir())
	request := importRequest(t, "Evidence.\n")
	request.Sources[0].Session = &ImportSession{Key: "manifest", Content: []byte("original manifest")}
	p := previewAccepted(t, s, request)
	for _, stale := range []string{"source", "manifest"} {
		_, err := s.ConfirmImport(context.Background(), p.ID, []string{"source"}, func(key string) ([]byte, error) {
			if key == stale {
				return []byte("changed"), nil
			}
			if key == "source" {
				return request.Sources[0].Content, nil
			}
			return request.Sources[0].Session.Content, nil
		})
		if !errors.Is(err, ErrImportStale) {
			t.Fatal(err)
		}
	}
	r, err := s.ImportStatus()
	if err != nil || len(r.Entries) != 0 {
		t.Fatal(r, err)
	}
}
func TestImportIdempotentAndSourceUnchanged(t *testing.T) {
	root, state, sourceRoot := t.TempDir(), t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, state)
	request := importRequest(t, "Evidence.\n")
	sourcePath := filepath.Join(sourceRoot, "R1.md")
	original := request.Sources[0].Content
	if err := os.WriteFile(sourcePath, original, 0600); err != nil {
		t.Fatal(err)
	}
	request.Sources[0].Origin = sourcePath
	p := previewAccepted(t, s, request)
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportWritten)
	target := filepath.Join(root, "session", "R1.md")
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(target)
	if err != nil || string(b) != p.Entries[0].Canonical {
		t.Fatal(err)
	}
	p = previewAccepted(t, s, request)
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportWritten)
	after, _ := os.Stat(target)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("idempotent import rewrote canonical file")
	}
	sourceAfter, _ := os.ReadFile(sourcePath)
	if string(sourceAfter) != string(original) {
		t.Fatal("source changed")
	}
}
func TestImportRestartPauseAndFixedDestination(t *testing.T) {
	ctx := context.Background()
	parent, state := t.TempDir(), t.TempDir()
	root := filepath.Join(parent, "offline")
	other := t.TempDir()
	profiles := []Profile{{ID: "main", Name: "Main", Root: root}, {ID: "other", Name: "Other", Root: other}}
	s := importService(t, profiles, state)
	request := importRequest(t, "Evidence.\n")
	request.Sources[0].Session = &ImportSession{Key: "session-key", Content: []byte("manifest")}
	p := previewAccepted(t, s, request)
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportUnavailable)
	if err := s.SetPublicationDisabled(ctx, "session-key", true); err != nil {
		t.Fatal(err)
	}
	waitImport(t, s, ImportPaused)
	s.Close()
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(profiles, map[string]string{"team": "other"})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if err = restarted.ConfigureImports(state); err != nil {
		t.Fatal(err)
	}
	waitImport(t, restarted, ImportPaused)
	if _, err = os.Stat(filepath.Join(root, "session", "R1.md")); !os.IsNotExist(err) {
		t.Fatal("paused write occurred")
	}
	if err = restarted.SetPublicationDisabled(ctx, "session-key", false); err != nil {
		t.Fatal(err)
	}
	waitImport(t, restarted, ImportWritten)
	if _, err = os.Stat(filepath.Join(root, "session", "R1.md")); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(other, "session", "R1.md")); !os.IsNotExist(err) {
		t.Fatal("mapping rerouted fixed job")
	}
}
func TestImportDestinationChangeRequiresRevalidation(t *testing.T) {
	state := t.TempDir()
	old := filepath.Join(t.TempDir(), "offline")
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: old}}, state)
	request := importRequest(t, "Evidence.\n")
	p := previewAccepted(t, s, request)
	confirmPreview(t, s, p, request)
	waitImport(t, s, ImportUnavailable)
	s.Close()
	next := t.TempDir()
	restarted := importService(t, []Profile{{ID: "main", Name: "Main", Root: next}}, state)
	r := waitImport(t, restarted, ImportDestinationChanged)
	if _, err := os.Stat(filepath.Join(next, "session", "R1.md")); !os.IsNotExist(err) {
		t.Fatal("write before revalidation")
	}
	if err := restarted.RevalidateImportDestination(context.Background(), r.Entries[0].ID); err != nil {
		t.Fatal(err)
	}
	waitImport(t, restarted, ImportWritten)
}
func TestImportsIndependentOfIndexDependencies(t *testing.T) {
	for _, mode := range []string{"nil-provider", "failed-initialization", "cache-symlink", "missing-model"} {
		t.Run(mode, func(t *testing.T) {
			root, state := t.TempDir(), t.TempDir()
			options := IndexOptions{StateDir: state, InstallationID: "installation"}
			if mode == "failed-initialization" {
				options.Failure = "state initialization failed"
			}
			if mode == "cache-symlink" {
				if err := os.Symlink(t.TempDir(), filepath.Join(root, ".qrouton")); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "missing-model" || mode == "cache-symlink" {
				options.Provider = &importMissingProvider{}
			}
			s, err := NewIndexedService([]Profile{{ID: "main", Name: "Main", Root: root}}, map[string]string{"team": "main"}, options)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			request := importRequest(t, "Evidence.\n")
			p := previewAccepted(t, s, request)
			confirmPreview(t, s, p, request)
			waitImport(t, s, ImportWritten)
		})
	}
}

type importMissingProvider struct{}

func (*importMissingProvider) Probe(context.Context) (Fingerprint, error) {
	return Fingerprint{}, ErrModelMissing
}
func (*importMissingProvider) Embed(context.Context, Fingerprint, []string) ([][]float32, error) {
	panic("missing provider embedded")
}
func TestConfigureImportsConcurrentClose(t *testing.T) {
	for n := 0; n < 10; n++ {
		s, err := NewService([]Profile{{ID: "main", Name: "Main", Root: t.TempDir()}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		state := t.TempDir()
		var group sync.WaitGroup
		for i := 0; i < 5; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				err := s.ConfigureImports(state)
				if err != nil && !errors.Is(err, ErrUnavailable) {
					t.Error(err)
				}
			}()
		}
		group.Add(1)
		go func() { defer group.Done(); s.Close() }()
		group.Wait()
		s.Close()
		if err := s.ConfigureImports(state); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
	}
}
func TestImportCollisionPreservesBothAndDurableIdentity(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, state)
	request := importRequest(t, "Incoming\n")
	p := previewAccepted(t, s, request)
	d := fixtureDocument()
	d.Body = "Other bytes\n"
	other, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(root, "session"), 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "session", "R1.md")
	if err = os.WriteFile(target, other, 0600); err != nil {
		t.Fatal(err)
	}
	confirmPreview(t, s, p, request)
	r := waitImport(t, s, ImportConflict)
	b, _ := os.ReadFile(target)
	if string(b) != string(other) {
		t.Fatal("existing overwritten")
	}
	siblings, _ := filepath.Glob(filepath.Join(root, "session", "conflict-*.md"))
	if len(siblings) != 1 {
		t.Fatal(siblings)
	}
	incoming, _ := os.ReadFile(siblings[0])
	if string(incoming) != p.Entries[0].Canonical {
		t.Fatal("incoming lost")
	}
	evidence, _ := os.ReadFile(filepath.Join(state, importDirectoryName, conflictPrefix+r.Entries[0].ID+".json"))
	var e conflictEvidence
	if json.Unmarshal(evidence, &e) != nil || len(e.Existing) != 1 {
		t.Fatal(string(evidence))
	}
	if err = os.Remove(siblings[0]); err != nil {
		t.Fatal(err)
	}
	s.Close()
	restarted := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, state)
	_, err = restarted.Read(Scope{ReadProfiles: []string{"main"}}, Reference{Profile: "main", ID: d.ID})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("ledger conflict lost: %v", err)
	}
	status := restarted.Status(Scope{ReadProfiles: []string{"main"}})
	if status.Profiles[0].Conflicts != 1 || status.Profiles[0].Documents != 0 {
		t.Fatal(status)
	}
}
func TestImportTargetOccupiedByAnotherIdentity(t *testing.T) {
	root, state := t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}}, state)
	request := importRequest(t, "Incoming\n")
	p := previewAccepted(t, s, request)
	d := fixtureDocument()
	d.ID = "session/R2"
	other, _ := Encode(d)
	if err := os.Mkdir(filepath.Join(root, "session"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "session", "R1.md"), other, 0600); err != nil {
		t.Fatal(err)
	}
	confirmPreview(t, s, p, request)
	r := waitImport(t, s, ImportRepairRequired)
	if _, err := os.Stat(filepath.Join(state, importDirectoryName, conflictPrefix+r.Entries[0].ID+".json")); err != nil {
		t.Fatal(err)
	}
	siblings, _ := filepath.Glob(filepath.Join(root, "session", "*conflict*"))
	if len(siblings) != 0 {
		t.Fatal(siblings)
	}
}
func TestImportRequiredHistoricalMetadataAndCrossVault(t *testing.T) {
	root, other := t.TempDir(), t.TempDir()
	s := importService(t, []Profile{{ID: "main", Name: "Main", Root: root}, {ID: "other", Name: "Other", Root: other}}, t.TempDir())
	request := importRequest(t, "Evidence\n")
	request.Sources[0].Content = []byte(strings.Replace(string(request.Sources[0].Content), strings.Repeat("a", 40), "HEAD", 1))
	p, err := s.PreviewImport(context.Background(), request)
	if err != nil || p.RepairRequired != 1 {
		t.Fatal(p, err)
	}
	request = importRequest(t, "Evidence\n")
	request.Sources[0].Origin = filepath.Join(root, "source.md")
	request.Overrides = map[string]ImportOverride{"source": {Destination: "other"}}
	p, err = s.PreviewImport(context.Background(), request)
	if err != nil || p.RepairRequired != 1 {
		t.Fatal(p, err)
	}
}
