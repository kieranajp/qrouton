package vault

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestImportAssetEnumeration(t *testing.T) {
	body := "---\nimage: '[no](metadata.png)'\n---\n![yes](../../assets/a.png \"title\")\n[ref]: ../../assets/b.png\n`![no](inline.png)`\n~~~\n![no](fence.png)\n~~~\n![no](https://example.com/remote.png)\n"
	got := EnumerateImportAssets(body)
	if len(got) != 2 || got[0] != "../../assets/a.png" || got[1] != "../../assets/b.png" {
		t.Fatal(got)
	}
}
func TestImportAssetsImmutablePublication(t *testing.T) {
	s, r := corpusFixture(t)
	source := corpusSource("one", "R1-2026-07-01-finding.md", "repo: qrouton", "# Finding\n![capture](../../assets/capture.png)\n")
	data := testPNG(t)
	r.Sources = []CorpusSource{source}
	r.Assets = []ImportAsset{{Key: "asset", SourceKey: "one", Href: "../../assets/capture.png", Content: data}}
	p, err := s.PreviewCorpus(context.Background(), r)
	if err != nil || p.Ready != 1 || p.Entries[0].AssetCount != 1 {
		t.Fatal(p, err)
	}
	entry, err := s.CorpusEntry(p.ID, "one")
	if err != nil || len(entry.Assets) != 1 {
		t.Fatal(entry, err)
	}
	if !strings.Contains(entry.Body, "assets/"+contentHash(string(data))+".png") {
		t.Fatal(entry.Body)
	}
	if _, err = s.ConfirmImport(context.Background(), p.ID, []string{"one"}, func(key string) ([]byte, error) {
		if key == "asset" {
			return []byte("changed"), nil
		}
		return source.Source.Content, nil
	}); !errors.Is(err, ErrImportStale) {
		t.Fatal(err)
	}
	callback := func(key string) ([]byte, error) {
		if key == "asset" {
			return data, nil
		}
		return source.Source.Content, nil
	}
	if _, err = s.ConfirmImport(context.Background(), p.ID, []string{"one"}, callback); err != nil {
		t.Fatal(err)
	}
	waitImport(t, s, ImportWritten)
	name := filepath.Join(s.profiles[0].Root, filepath.FromSlash(entry.Assets[0].Path))
	actual, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(actual, data) {
		t.Fatal(err)
	}
	p2, err := s.PreviewCorpus(context.Background(), r)
	if err != nil || p2.Ready != 1 {
		t.Fatal(p2, err)
	}
	e2, _ := s.CorpusEntry(p2.ID, "one")
	if e2.Canonical != entry.Canonical {
		t.Fatal("rerun canonical changed")
	}
	before, _ := os.Stat(name)
	if _, err = s.ConfirmImport(context.Background(), p.ID, []string{"one"}, callback); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(name)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("asset rewritten")
	}
}
func TestImportAssetValidationAndCollision(t *testing.T) {
	for _, kind := range []string{"invalid", "escape", "collision", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			s, r := corpusFixture(t)
			href := "../../assets/a.png"
			if kind == "escape" {
				href = "../../../other/assets/a.png"
			}
			source := corpusSource("one", "R1-2026-07-01-finding.md", "repo: qrouton", "# Finding\n![a]("+href+")\n")
			data := testPNG(t)
			if kind == "invalid" {
				data = []byte("not a PNG")
			}
			r.Sources = []CorpusSource{source}
			r.Assets = []ImportAsset{{Key: "asset", SourceKey: "one", Href: href, Content: data}}
			p, err := s.PreviewCorpus(context.Background(), r)
			if kind == "invalid" || kind == "escape" {
				if !errors.Is(err, ErrImportSource) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			entry, _ := s.CorpusEntry(p.ID, "one")
			storePath, _ := s.importDirectory()
			store, err := os.OpenRoot(storePath)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if err := os.MkdirAll(filepath.Join(s.profiles[0].Root, "session"), 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				if err := os.Symlink(t.TempDir(), filepath.Join(s.profiles[0].Root, "session/assets")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.MkdirAll(filepath.Join(s.profiles[0].Root, "session/assets"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(s.profiles[0].Root, entry.Assets[0].Path), []byte("occupied"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			job := queueEntry{Path: entry.Path, Assets: entry.Assets, Root: canonicalProfileRoot(s.profiles[0].Root), Canonical: []byte(entry.Canonical), Hash: contentHash(entry.Canonical), Preview: p.ID, Source: "one", ImportJob: ImportJob{ArtifactID: entry.Document.ID, Profile: "personal"}}
			if err := s.writeCanonical(context.Background(), s.profiles[0], job, store); err == nil {
				t.Fatal("collision accepted")
			}
			if _, err := os.Stat(filepath.Join(s.profiles[0].Root, entry.Path)); !os.IsNotExist(err) {
				t.Fatal("Markdown published before asset")
			}
			if kind == "collision" {
				b, _ := os.ReadFile(filepath.Join(s.profiles[0].Root, entry.Assets[0].Path))
				if string(b) != "occupied" {
					t.Fatal("overwritten")
				}
			}
		})
	}
}

func TestImportAssetsRetryFromArchive(t *testing.T) {
	s, r := corpusFixture(t)
	source := corpusSource("one", "R1-2026-07-01-finding.md", "repo: qrouton", "# Finding\n![capture](../../assets/capture.png)\n")
	data := testPNG(t)
	r.Sources = []CorpusSource{source}
	r.Assets = []ImportAsset{{Key: "asset", SourceKey: "one", Href: "../../assets/capture.png", Content: data}}
	p, err := s.PreviewCorpus(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := s.CorpusEntry(p.ID, "one")
	root := s.profiles[0].Root
	if err = os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ConfirmImport(context.Background(), p.ID, []string{"one"}, func(key string) ([]byte, error) {
		if key == "asset" {
			return data, nil
		}
		return source.Source.Content, nil
	}); err != nil {
		t.Fatal(err)
	}
	state := s.importState
	s.Close()
	if err = os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	next, err := NewService(s.profiles, s.mappings)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	if err = next.ConfigureImports(state); err != nil {
		t.Fatal(err)
	}
	waitImport(t, next, ImportWritten)
	bytesOnDisk, err := os.ReadFile(filepath.Join(root, entry.Assets[0].Path))
	if err != nil || !bytes.Equal(bytesOnDisk, data) {
		t.Fatal(err)
	}
}
