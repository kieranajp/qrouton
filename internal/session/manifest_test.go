package session

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
)

// WriteManifest stages through a fixed-name temp file until the concurrency
// fix, so two processes racing os.WriteFile onto the same path can rename a
// short write over the manifest. This proves the fix: concurrent writers must
// never let Load observe a torn file, and none may leave a temp file behind.
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
