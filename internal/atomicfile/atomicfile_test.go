package atomicfile

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// A reader polling a file several processes replace must always see one whole
// document: never an empty file, never half of one, and never a staging file
// left behind.
func TestConcurrentReplacesAreNeverObservedTorn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.json")
	bodies := make([][]byte, 24)
	whole := make(map[byte]int, len(bodies))
	for i := range bodies {
		bodies[i] = bytes.Repeat([]byte{byte('a' + i)}, (i+1)*997)
		whole[byte('a'+i)] = len(bodies[i])
	}
	if err := Replace(path, bodies[0], 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var torn []byte
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for ctx.Err() == nil {
			b, err := os.ReadFile(path)
			if err != nil {
				torn = []byte(err.Error())
				return
			}
			if len(b) == 0 || whole[b[0]] != len(b) || bytes.Count(b, b[:1]) != len(b) {
				torn = b
				return
			}
		}
	}()

	var writers sync.WaitGroup
	for _, b := range bodies {
		writers.Add(1)
		go func(b []byte) {
			defer writers.Done()
			if err := Replace(path, b, 0o644); err != nil {
				t.Errorf("Replace: %v", err)
			}
		}(b)
	}
	writers.Wait()
	cancel()
	readers.Wait()

	if torn != nil {
		t.Fatalf("a concurrent reader saw a partial file: %q", torn)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatalf("directory holds %v, want only %s", entries, filepath.Base(path))
	}
}

func TestReplaceKeepsTheRequestedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc")
	if err := Replace(path, []byte("body"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}
