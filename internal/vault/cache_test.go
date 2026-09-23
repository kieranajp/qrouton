package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCacheMissesOnAnythingButAnExactHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.gob")
	header := cacheHeader{Model: OllamaModel, Digest: "d", Preprocessing: preprocessing}
	vectors := map[string][][]float32{"h": {{1, 2}}}
	b, err := encodeCache(header, vectors)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCache(path, b); err != nil {
		t.Fatal(err)
	}
	if got := loadCache(path, header); len(got["h"]) != 1 {
		t.Fatalf("round trip = %v", got)
	}
	for name, other := range map[string]cacheHeader{
		"digest":        {Model: OllamaModel, Digest: "e", Preprocessing: preprocessing},
		"preprocessing": {Model: OllamaModel, Digest: "d", Preprocessing: preprocessing + 1},
	} {
		if got := loadCache(path, other); len(got) != 0 {
			t.Errorf("%s change reused %v", name, got)
		}
	}
	mixed, _ := encodeCache(header, map[string][][]float32{"a": {{1, 2}}, "b": {{1, 2, 3}}})
	_ = os.WriteFile(path, mixed, 0o644)
	if got := loadCache(path, header); len(got) != 0 {
		t.Errorf("mixed dimensions reused %v", got)
	}
	_ = os.WriteFile(path, b[:len(b)/2], 0o644)
	if got := loadCache(path, header); len(got) != 0 {
		t.Errorf("truncated file reused %v", got)
	}
}
