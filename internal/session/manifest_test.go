package session

import (
	"context"
	"os"
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
