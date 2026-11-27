package pagetest

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the page JSON goldens")

// Golden compares got with the frozen file, or rewrites the file under -update.
func Golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		var parsed any
		if err := json.Unmarshal(got, &parsed); err != nil {
			t.Fatalf("got is not JSON: %v\n%s", err, got)
		}
		got, _ = json.MarshalIndent(parsed, "", "  ")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run the test with -update to write it", err)
	}
	Same(t, got, want)
}
