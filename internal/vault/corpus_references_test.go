package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorpusLegacyReferencesAndStableIDs(t *testing.T) {
	s, r := corpusFixture(t)
	a := corpusSource("a", "R1-2026-07-01-result.md", "repo: qrouton\nplan: thoughts/shared/research/R1-2026-07-01-questions.md\nresearch: none — discussed with author", "# Result\n[questions](thoughts/shared/research/R1-2026-07-01-questions.md#why)\n")
	b := corpusSource("b", "R1-2026-07-01-questions.md", "repo: qrouton\nspec: R1-2026-07-01-result.md", "# Questions\n")
	r.Sources = []CorpusSource{a, b}
	p, err := s.PreviewCorpus(context.Background(), r)
	if err != nil || p.Ready != 2 {
		t.Fatal(p, err)
	}
	e, _ := s.CorpusEntry(p.ID, "a")
	if len(e.Document.Lineage) != 0 || e.Document.ID != "session/R1-2026-07-01-result" || !strings.Contains(e.Body, "R1-2026-07-01-questions.md#why") || !strings.Contains(e.Body, corpusLegacyReference) {
		t.Fatal(e)
	}
	r.Sources = []CorpusSource{b}
	p, err = s.PreviewCorpus(context.Background(), r)
	if err != nil || p.Entries[0].ID != "session/R1-2026-07-01-questions" {
		t.Fatal(p, err)
	}
}
func TestCorpusInertUnavailableReferences(t *testing.T) {
	for _, metadata := range []string{"repo: lsx", "repo: qrouton\nstate: invalid"} {
		t.Run(metadata, func(t *testing.T) {
			s, r := corpusFixture(t)
			a := corpusSource("a", "R1-2026-07-01-a.md", "repo: qrouton", "# A\n[authored](R2-2026-07-01-b.md)\n[[R2-2026-07-01-b|wiki authored]]\n[reference][ref]\n[ref]: R2-2026-07-01-b.md\n")
			b := corpusSource("b", "R2-2026-07-01-b.md", metadata, "# Private target title\n")
			r.Sources = []CorpusSource{a, b}
			p, err := s.PreviewCorpus(context.Background(), r)
			if err != nil {
				t.Fatal(err)
			}
			e, _ := s.CorpusEntry(p.ID, "a")
			if e.Disposition != "ready" || len(e.Dependencies) != 0 || strings.Contains(e.Body, "](R2") || strings.Contains(e.Body, "Private target title") || !strings.Contains(e.Body, "authored") {
				t.Fatal(e)
			}
		})
	}
}
func TestCorpusExplicitLineageCycleRemainsRepair(t *testing.T) {
	s, r := corpusFixture(t)
	r.Sources = []CorpusSource{corpusSource("a", "R1-2026-07-01-a.md", "repo: qrouton\nderived_from: thoughts/shared/research/R2-2026-07-01-b.md", "# A\n"), corpusSource("b", "R2-2026-07-01-b.md", "repo: qrouton\nderived_from: R1-2026-07-01-a", "# B\n")}
	p, err := s.PreviewCorpus(context.Background(), r)
	if err != nil || p.RepairRequired != 2 {
		t.Fatal(p, err)
	}
}
func TestCorpusPriorConfirmedIdentitySurvivesRemoval(t *testing.T) {
	s, r := corpusFixture(t)
	source := corpusSource("one", "R1-2026-07-01-finding.md", "repo: qrouton", "# Finding\n")
	r.Sources = []CorpusSource{source}
	p, err := s.PreviewCorpus(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	directory, _ := s.importDirectory()
	store, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	record, err := loadImportRecord(store, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	e := &record.Preview.Entries[0]
	e.Document.Body = e.Body
	e.Document.ID = "session/R1"
	e.Path = "session/R1.md"
	encoded, err := Encode(e.Document)
	if err != nil {
		t.Fatal(err)
	}
	e.Canonical = string(encoded)
	b, err := encodeImportRecord(record, maxImportRecordBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err = replaceRoot(store, p.ID+".json", b); err != nil {
		t.Fatal(err)
	}
	store.Close()
	if _, err = s.ConfirmImport(context.Background(), p.ID, []string{"one"}, func(string) ([]byte, error) { return source.Source.Content, nil }); err != nil {
		t.Fatal(err)
	}
	waitImport(t, s, ImportWritten)
	if err = os.Remove(filepath.Join(s.profiles[0].Root, "session/R1.md")); err != nil {
		t.Fatal(err)
	}
	p, err = s.PreviewCorpus(context.Background(), r)
	if err != nil || p.Ready != 1 || p.Entries[0].ID != "session/R1" || p.Entries[0].Provenance["id"].Source != "prior_import" {
		t.Fatal(p, err)
	}
	copySource := source
	copySource.Source.Key = "copy"
	copySource.Source.Name = "R2-2026-07-01-copy.md"
	copySource.RelativePath = "session/shared/research/R2-2026-07-01-copy.md"
	r.Sources = []CorpusSource{source, copySource}
	p, err = s.PreviewCorpus(context.Background(), r)
	if err != nil || p.Ready != 2 {
		t.Fatal(p, err)
	}
	copyEntry, _ := s.CorpusEntry(p.ID, "copy")
	if copyEntry.Document.ID != "session/R2-2026-07-01-copy" {
		t.Fatal("different source identity reused", copyEntry.Document.ID)
	}

}

func TestCorpusSameProfileTargetPrecedesOtherProfile(t *testing.T) {
	s, r := corpusFixture(t)
	a := corpusSource("a", "R1-2026-07-01-a.md", "repo: qrouton", "# A\n[local](R2-2026-07-01-b.md)\n")
	b := corpusSource("b", "R2-2026-07-01-b.md", "repo: qrouton", "# B\n")
	other := corpusSource("other", "R2-2026-07-01-b.md", "repo: lsx", "# Other\n")
	other.RelativePath = "other/shared/research/" + other.Source.Name
	r.Sources = []CorpusSource{other, a, b}
	p, err := s.PreviewCorpus(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := s.CorpusEntry(p.ID, "a")
	if len(e.Dependencies) != 1 || e.Dependencies[0] != "b" || !strings.Contains(e.Body, "](R2-2026-07-01-b.md)") {
		t.Fatal(e)
	}
}

func TestCorpusAmbiguousSameProfileCannotBecomeInert(t *testing.T) {
	s, r := corpusFixture(t)
	a := corpusSource("a", "R1-2026-07-01-a.md", "repo: qrouton", "# A\n[[Shared title]]\n")
	b := corpusSource("b", "R2-2026-07-01-b.md", "repo: qrouton", "# Shared title\n")
	c := corpusSource("c", "R3-2026-07-01-c.md", "repo: qrouton", "# Shared title\n")
	other := corpusSource("other", "R4-2026-07-01-other.md", "repo: lsx", "# Shared title\n")
	r.Sources = []CorpusSource{other, a, b, c}
	p, err := s.PreviewCorpus(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := s.CorpusEntry(p.ID, "a")
	if e.Disposition != "repair" || !strings.Contains(e.Body, "[[Shared title]]") {
		t.Fatal(e)
	}
}
func TestCorpusPriorImportRestoresDependencies(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(fmt.Sprint(typed), func(t *testing.T) {
			s, r := corpusFixture(t)
			metadata, body := "repo: qrouton", "# A\n[B](R2-2026-07-01-b.md)\n"
			if typed {
				metadata += "\nderived_from: R2-2026-07-01-b.md"
				body = "# A\n"
			}
			a := corpusSource("a", "R1-2026-07-01-a.md", metadata, body)
			b := corpusSource("b", "R2-2026-07-01-b.md", "repo: qrouton", "# B\n")
			r.Sources = []CorpusSource{a, b}
			p, err := s.PreviewCorpus(context.Background(), r)
			if err != nil || p.Ready != 2 {
				t.Fatal(p, err)
			}
			before, _ := s.CorpusEntry(p.ID, "a")
			callback := func(key string) ([]byte, error) {
				switch key {
				case "a":
					return a.Source.Content, nil
				case "b":
					return b.Source.Content, nil
				default:
					return []byte("new manifest"), nil
				}
			}
			if _, err = s.ConfirmImport(context.Background(), p.ID, []string{"a", "b"}, callback); err != nil {
				t.Fatal(err)
			}
			waitImport(t, s, ImportWritten)
			for _, name := range []string{"R1-2026-07-01-a.md", "R2-2026-07-01-b.md"} {
				if err = os.Remove(filepath.Join(s.profiles[0].Root, "session", name)); err != nil {
					t.Fatal(err)
				}
			}
			a.Source.Session = &ImportSession{Key: "new-manifest", Content: []byte("new manifest")}
			r.Sources = []CorpusSource{a, b}
			p, err = s.PreviewCorpus(context.Background(), r)
			if err != nil || p.Ready != 2 {
				t.Fatal(p, err)
			}
			after, _ := s.CorpusEntry(p.ID, "a")
			if before.Canonical != after.Canonical || len(after.Dependencies) != 1 || after.Dependencies[0] != "b" {
				t.Fatal(after)
			}
			if _, err = s.ConfirmImport(context.Background(), p.ID, []string{"a"}, callback); !errors.Is(err, ErrImportRepair) {
				t.Fatal("unselected missing dependency accepted", err)
			}
			r.Sources = []CorpusSource{a}
			p, err = s.PreviewCorpus(context.Background(), r)
			if err != nil || p.RepairRequired != 1 {
				t.Fatal("missing dependency omitted", p, err)
			}
		})
	}
}
