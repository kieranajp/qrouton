package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Several processes write the manifest at once, and a poller reads it between
// them: none of those writers may let Load observe a torn file, and none may
// leave a staging file behind.
func TestWriteManifestConcurrentWritesNeverTearOrLeaveTempFiles(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent"}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var readErr error
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for ctx.Err() == nil {
			if _, err := Load(dir); err != nil {
				readErr = err
				return
			}
		}
	}()

	var writers sync.WaitGroup
	for i := range 20 {
		writers.Add(1)
		go func(i int) {
			defer writers.Done()
			m := Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent", Name: strings.Repeat("x", i*97)}
			if err := WriteManifest(dir, m); err != nil {
				t.Errorf("WriteManifest: %v", err)
			}
		}(i)
	}
	writers.Wait()
	cancel()
	readers.Wait()

	if readErr != nil {
		t.Fatalf("a concurrent reader saw a torn manifest: %v", readErr)
	}
	if _, err := Load(dir); err != nil {
		t.Fatalf("final manifest does not parse: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != manifestName {
		t.Fatalf("directory holds %v, want only %s", entries, manifestName)
	}
}

// Three processes edit a live session's manifest, each loading it, changing one
// field and replacing it. Two doing that at once must both leave their change
// behind: the later one has to see the earlier one's, not the snapshot it would
// have loaded before it.
func TestUpdateManifestConcurrentMutationsBothSurvive(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent"}); err != nil {
		t.Fatal(err)
	}

	mutations := []func(Manifest) (Manifest, error){
		func(m Manifest) (Manifest, error) {
			time.Sleep(50 * time.Millisecond)
			m.Mode = ModeAssistant
			return m, nil
		},
		func(m Manifest) (Manifest, error) {
			time.Sleep(50 * time.Millisecond)
			m.Runner = "codex"
			return m, nil
		},
	}
	var wg sync.WaitGroup
	for _, mutate := range mutations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := UpdateManifest(dir, mutate); err != nil {
				t.Errorf("UpdateManifest: %v", err)
			}
		}()
	}
	wg.Wait()

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeAssistant || got.Runner != "codex" {
		t.Fatalf("one update clobbered the other: mode = %q, runner = %q", got.Mode, got.Runner)
	}
}

func TestCycleStickerPersistsTheCompleteCycleAndClearsTheManifestKey(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "stickers"}); err != nil {
		t.Fatal(err)
	}
	want := []Sticker{StickerStar, StickerBookmark, StickerQuestion, StickerExclamation, ""}
	for _, expected := range want {
		got, err := CycleSticker(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != expected {
			t.Fatalf("CycleSticker() = %q, want %q", got, expected)
		}
		loaded, err := Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Sticker != expected {
			t.Fatalf("reloaded sticker = %q, want %q", loaded.Sticker, expected)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"sticker"`) {
		t.Fatalf("cleared manifest retained sticker: %s", b)
	}
}

func TestStickerTreatsAbsentAndUnknownValuesAsNone(t *testing.T) {
	for _, initial := range []Sticker{"", "retired"} {
		if got := initial.Effective(); got != "" {
			t.Fatalf("Sticker(%q).Effective() = %q, want none", initial, got)
		}
		if got := initial.Next(); got != StickerStar {
			t.Fatalf("Sticker(%q).Next() = %q, want %q", initial, got, StickerStar)
		}
	}

	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "legacy", Sticker: "retired"}); err != nil {
		t.Fatal(err)
	}
	got, err := CycleSticker(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != StickerStar {
		t.Fatalf("CycleSticker() from unknown = %q, want %q", got, StickerStar)
	}
}

func TestCycleStickerAndAnotherManifestMutationBothSurvive(t *testing.T) {
	dir := t.TempDir()
	if err := WriteManifest(dir, Manifest{SchemaVersion: manifestSchemaVersion, Slug: "concurrent"}); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := CycleSticker(dir); err != nil {
			t.Errorf("CycleSticker: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := UpdateManifest(dir, func(m Manifest) (Manifest, error) {
			time.Sleep(50 * time.Millisecond)
			m.Runner = "codex"
			return m, nil
		}); err != nil {
			t.Errorf("UpdateManifest: %v", err)
		}
	}()
	wg.Wait()

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sticker != StickerStar || got.Runner != "codex" {
		t.Fatalf("concurrent result = %+v", got)
	}
}
