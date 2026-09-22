package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func corpusFixture(t *testing.T) (*Service, CorpusRequest) {
	t.Helper()
	s, err := NewService([]Profile{{ID: "personal", Name: "Personal", Root: t.TempDir()}, {ID: "team", Name: "Team", Root: t.TempDir()}}, map[string]string{"kieranajp": "personal", "lifesum": "team"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureImports(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, CorpusRequest{TargetProfile: "personal", Defaults: CorpusDefaults{Author: "Reviewed author", State: "active", WorkstreamFromSession: true}, RepositoryAliases: map[string]string{"qrouton": "github.com/kieranajp/qrouton", "lsx": "github.com/lifesum/lsx"}}
}
func corpusSource(key, name, metadata, body string) CorpusSource {
	return CorpusSource{Source: ImportSource{Key: key, Name: name, Content: []byte("---\n" + metadata + "\n---\n" + body)}, RelativePath: "session/shared/research/" + name}
}
func TestCorpusLegacyUnknownRevisionRoundTrip(t *testing.T) {
	s, request := corpusFixture(t)
	source := corpusSource("one", "R1-2026-07-01-finding.md", "repo: src/qrouton", "# Finding\n\n[code](../../../src/qrouton/main.go:12)\n")
	request.Sources = []CorpusSource{source}
	preview, err := s.PreviewCorpus(context.Background(), request)
	if err != nil || preview.Ready != 1 {
		t.Fatal(preview, err)
	}
	entry, err := s.CorpusEntry(preview.ID, "one")
	if err != nil {
		t.Fatal(err)
	}
	d := entry.Document
	if d.LegacyImport == nil || d.LegacyImport.SourceHash != contentHash(string(source.Source.Content)) || d.Repos[0].Revision != nil || d.Repos[0].Role != "reference" {
		t.Fatal(d)
	}
	if d.ID != "session/R1" || d.Kind != "research" || d.Date != "2026-07-01" || d.Title != "Finding" || d.Workstream != "session" {
		t.Fatal(d)
	}
	if entry.Provenance["author"].Source != "batch_default" || entry.Provenance["session"].Source != "folder" {
		t.Fatal(entry.Provenance)
	}
	if !strings.Contains(entry.Body, "repo://github.com/kieranajp/qrouton/main.go?revision=unknown#L12") {
		t.Fatal(entry.Body)
	}
	if _, err = Parse([]byte(entry.Canonical)); err != nil {
		t.Fatal(err)
	}
	_, err = s.ConfirmImport(context.Background(), preview.ID, []string{"one"}, func(string) ([]byte, error) { return source.Source.Content, nil })
	if err != nil {
		t.Fatal(err)
	}
	waitImport(t, s, ImportWritten)
	read, err := s.Read(Scope{ReadProfiles: []string{"personal"}}, Reference{Profile: "personal", ID: d.ID})
	if err != nil || read.Staleness != "unknown" || read.Document.LegacyImport == nil {
		t.Fatal(read, err)
	}
	bytes, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bytes), "main.go?revision") || strings.Contains(string(bytes), "canonical") {
		t.Fatal("summary includes body")
	}
}
func TestLegacyMarkerDoesNotRelaxOrdinaryOrDeadEndDocuments(t *testing.T) {
	d := fixtureDocument()
	d.Repos[0].Revision = nil
	if _, err := Encode(d); !errors.Is(err, ErrInvalidDocument) {
		t.Fatal(err)
	}
	d.LegacyImport = &LegacyImport{SourceHash: strings.Repeat("a", 64)}
	for _, kind := range []string{"research", "spec", "plan"} {
		d.Kind = kind
		b, err := Encode(d)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Parse(b); err != nil {
			t.Fatal(err)
		}
		bad := strings.Replace(string(b), "source_hash:", "unknown: value\n    source_hash:", 1)
		if _, err = Parse([]byte(bad)); !errors.Is(err, ErrInvalidDocument) {
			t.Fatal(err)
		}
	}
	d.Kind = "dead_end"
	if _, err := Encode(d); !errors.Is(err, ErrInvalidDocument) {
		t.Fatal("dead end accepted without pin", err)
	}
	d.Kind = "research"
	d.LegacyImport.SourceHash = "fake"
	if _, err := Encode(d); !errors.Is(err, ErrInvalidDocument) {
		t.Fatal(err)
	}
}
func TestLegacyWriterRequiresDurableImportEvidence(t *testing.T) {
	s, _ := corpusFixture(t)
	directory, _ := s.importDirectory()
	store, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	d := fixtureDocument()
	d.Repos[0].Path = "github.com/kieranajp/qrouton"
	d.Repos[0].Revision = nil
	d.LegacyImport = &LegacyImport{SourceHash: strings.Repeat("a", 64)}
	b, _ := Encode(d)
	job := queueEntry{ImportJob: ImportJob{ID: "queue-fake", Profile: "personal", ArtifactID: d.ID}, Canonical: b, Hash: contentHash(string(b)), Path: d.ID + ".md", Root: canonicalProfileRoot(s.profiles[0].Root)}
	if err = s.writeCanonical(context.Background(), s.profiles[0], job, store); !errors.Is(err, ErrImportRepair) {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(s.profiles[0].Root, "session", "R1.md")); !os.IsNotExist(err) {
		t.Fatal("forged legacy publication wrote")
	}
}
func TestCorpusRepositoryVariantsAndPrivacy(t *testing.T) {
	cases := []struct {
		name, metadata          string
		ready, repair, excluded int
	}{
		{"scalar", "repo: qrouton", 1, 0, 0}, {"portable", "repository: kieranajp/qrouton @ feat/old", 1, 0, 0}, {"list", "repositories: [src/qrouton]", 1, 0, 0}, {"rich", "repos:\n  - path: src/qrouton\n    role: active", 1, 0, 0},
		{"map", "repositories: {source: qrouton, target: qrouton}", 1, 0, 0}, {"other", "repo: lsx", 0, 0, 1}, {"mixed", "repos: [qrouton, lsx]", 0, 1, 0}, {"unknown", "repos: [qrouton, missing-repo]", 0, 1, 0}, {"bad role", "repos: [{path: qrouton, role: 42}]", 0, 1, 0}, {"bad entry", "repos: [qrouton, {path: 42}]", 0, 1, 0}, {"conflict alias", "repos: [{path: qrouton, name: lsx}]", 0, 1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, request := corpusFixture(t)
			request.Sources = []CorpusSource{corpusSource("one", "R1-2026-07-01-finding.md", tc.metadata, "# Finding\n")}
			request.Overrides = map[string]ImportOverride{"one": {Destination: "personal"}}
			preview, err := s.PreviewCorpus(context.Background(), request)
			if err != nil || preview.Ready != tc.ready || preview.RepairRequired != tc.repair || preview.Excluded != tc.excluded {
				t.Fatal(preview, err)
			}
			if tc.excluded > 0 {
				if _, err = s.ConfirmImport(context.Background(), preview.ID, []string{"one"}, func(string) ([]byte, error) { return request.Sources[0].Source.Content, nil }); !errors.Is(err, ErrImportRepair) {
					t.Fatal("excluded entry confirmed", err)
				}
			}
		})
	}
}
func TestCorpusManifestPinsAndExplicitDefaults(t *testing.T) {
	s, request := corpusFixture(t)
	pin := strings.Repeat("a", 40)
	source := corpusSource("one", "R1-2026-07-01-finding.md", "repo: qrouton\nauthor: Historical author\nstate: abandoned\nworkstream: explicit", "# Finding\n")
	source.Source.Session = &ImportSession{Key: "manifest", Content: []byte("historical manifest"), Slug: "session", Repositories: []Repository{{Path: "github.com/kieranajp/qrouton", Role: "editing", Revision: &pin}}}
	request.Sources = []CorpusSource{source}
	p, err := s.PreviewCorpus(context.Background(), request)
	if err != nil || p.Ready != 1 {
		t.Fatal(p, err)
	}
	entry, _ := s.CorpusEntry(p.ID, "one")
	if entry.Document.Author != "Historical author" || entry.Document.State != "abandoned" || entry.Document.Workstream != "explicit" || *entry.Document.Repos[0].Revision != pin || entry.Document.Repos[0].Role != "editing" {
		t.Fatal(entry)
	}
	request.Sources[0].Source.Session = nil
	request.Sources[0].Source.Content = []byte("---\nrepo: qrouton\nrevision: abcdef0\n---\n# Finding\n")
	calls := 0
	request.ResolveRevision = func(repo, recorded string) (string, error) {
		calls++
		if repo != "github.com/kieranajp/qrouton" || recorded != "abcdef0" {
			t.Fatal(repo, recorded)
		}
		return pin, nil
	}
	p, err = s.PreviewCorpus(context.Background(), request)
	if err != nil || p.Ready != 1 || calls != 1 {
		t.Fatal(p, err, calls)
	}
}
func TestCorpusRejectsConflictsAndPropagatesLinkRepairs(t *testing.T) {
	s, request := corpusFixture(t)
	request.Sources = []CorpusSource{corpusSource("one", "R1-2026-07-01-finding.md", "repo: qrouton", "# One\n"), corpusSource("questions", "R1-2026-07-01-finding-questions.md", "repo: qrouton", "# Questions\n"), corpusSource("two", "R2-2026-07-01-finding.md", "repo: qrouton", "# Two\n\n[one](R1-2026-07-01-finding.md)\n")}
	p, err := s.PreviewCorpus(context.Background(), request)
	if err != nil || p.RepairRequired != 3 || p.Ready != 0 {
		t.Fatal(p, err)
	}
	request.Sources[1].Source.Key = "one"
	if _, err = s.PreviewCorpus(context.Background(), request); !errors.Is(err, ErrImportSource) {
		t.Fatal(err)
	}
	request.Sources = request.Sources[:1]
	request.Sources[0].Source.Content = []byte("---\nschema_version: 999\nrepo: qrouton\n---\n# Future\n")
	p, err = s.PreviewCorpus(context.Background(), request)
	if err != nil || p.RepairRequired != 1 {
		t.Fatal(p, err)
	}
	request.Sources[0] = corpusSource("one", "R1-2026-07-01-finding.md", "repo: src/qrouton", "# Finding\n")
	request.RepositoryAliases["src/qrouton"] = "github.com/lifesum/lsx"
	p, err = s.PreviewCorpus(context.Background(), request)
	if err != nil || p.RepairRequired != 1 {
		t.Fatal(p, err)
	}
}
func TestCorpusLargeLinkedBatchWithExistingInventory(t *testing.T) {
	s, request := corpusFixture(t)
	for i := 0; i < 40; i++ {
		d := fixtureDocument()
		d.ID = fmt.Sprintf("existing/R%d", i)
		d.Session = "existing"
		writeImportDocument(t, s.profiles[0].Root, d.ID+".md", d)
	}
	const count = 300
	for i := 0; i < count; i++ {
		body := fmt.Sprintf("# Finding %d\n", i)
		if i+1 < count {
			body += fmt.Sprintf("\n[next](R%d-2026-07-01-finding.md#finding)\n", i+1)
		}
		request.Sources = append(request.Sources, corpusSource(fmt.Sprint(i), fmt.Sprintf("R%d-2026-07-01-finding.md", i), "repo: qrouton", body))
	}
	started := time.Now()
	p, err := s.PreviewCorpus(context.Background(), request)
	if err != nil || p.Total != count || p.Ready != count {
		t.Fatal(p, err)
	}
	t.Logf("%d linked sources +40 destination documents: %s", count, time.Since(started))
	entry, err := s.CorpusEntry(p.ID, "0")
	if err != nil || !strings.Contains(entry.Body, "R1.md#finding") || len(entry.Dependencies) != 1 {
		t.Fatal(entry, err)
	}
}

func TestCorpusReviewedSessionScopeDefaultsDoNotNarrowDeclarations(t *testing.T) {
	s, request := corpusFixture(t)
	request.SessionRepositories = map[string][]Repository{"session": {{Path: "github.com/kieranajp/qrouton", Role: "editing"}}}
	request.Sources = []CorpusSource{corpusSource("one", "R1-2026-07-01-one.md", "", "# One\n"), corpusSource("two", "R2-2026-07-01-two.md", "repo: lsx", "# Two\n"), corpusSource("three", "R3-2026-07-01-three.md", "repos: [qrouton, unresolved]", "# Three\n")}
	p, err := s.PreviewCorpus(context.Background(), request)
	if err != nil || p.Ready != 1 || p.RepairRequired != 1 || p.Excluded != 1 {
		t.Fatal(p, err)
	}
	entry, _ := s.CorpusEntry(p.ID, "one")
	if entry.Provenance["github.com/kieranajp/qrouton"].Source != "batch_default" {
		t.Fatal(entry.Provenance)
	}
	request.Sources = request.Sources[:1]
	request.Sources[0].Source.Session = &ImportSession{Key: "manifest", Content: []byte("team manifest"), Repositories: []Repository{{Path: "github.com/lifesum/lsx", Role: "editing"}}}
	p, err = s.PreviewCorpus(context.Background(), request)
	if err != nil || p.Excluded != 1 || p.Ready != 0 {
		t.Fatal(p, err)
	}
}
func TestCorpusPinnedCanonicalImportPreservesBytes(t *testing.T) {
	s, request := corpusFixture(t)
	d := fixtureDocument()
	d.Repos[0].Path = "github.com/kieranajp/qrouton"
	b, _ := Encode(d)
	request.Sources = []CorpusSource{{Source: ImportSource{Key: "canonical", Name: "R1.md", Content: b}, RelativePath: "session/shared/research/R1.md"}}
	p, err := s.PreviewCorpus(context.Background(), request)
	if err != nil || p.Ready != 1 {
		t.Fatal(p, err)
	}
	entry, _ := s.CorpusEntry(p.ID, "canonical")
	if entry.Canonical != string(b) || entry.Document.LegacyImport != nil {
		t.Fatal("canonical bytes changed")
	}
}

func TestCorpusArchivesSharedManifestOnce(t *testing.T) {
	s, request := corpusFixture(t)
	manifest := &ImportSession{Key: "shared-manifest", Content: []byte(strings.Repeat("manifest-evidence", 1000)), Repositories: []Repository{{Path: "github.com/kieranajp/qrouton", Role: "editing"}}}
	for i := 0; i < 3; i++ {
		source := corpusSource(fmt.Sprint(i), fmt.Sprintf("R%d-2026-07-01-doc.md", i), "", "# Finding\n")
		source.Source.Session = manifest
		request.Sources = append(request.Sources, source)
	}
	preview, err := s.PreviewCorpus(context.Background(), request)
	if err != nil || preview.Ready != 3 {
		t.Fatal(preview, err)
	}
	directory, _ := s.importDirectory()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	b, err := readRegular(root, preview.ID+".json", maxImportRecordBytes)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw["sources"]), "manifest-evidence") {
		t.Fatal("manifest repeated in sources")
	}
	record, err := loadImportRecord(root, preview.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Sessions) != 1 || len(record.SourceSessions) != 3 {
		t.Fatal(record.Sessions)
	}
	for _, source := range record.Sources {
		if source.Session == nil || string(source.Session.Content) != string(manifest.Content) {
			t.Fatal("manifest restore failed")
		}
	}
}
