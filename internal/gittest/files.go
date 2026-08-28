package gittest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WriteFile writes one newline-terminated file under dir, creating the
// directories it needs.
func WriteFile(t *testing.T, dir, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := mkdirAll(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	body := ""
	if len(lines) > 0 {
		body = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }
