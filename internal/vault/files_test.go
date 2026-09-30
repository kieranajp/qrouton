package vault

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestWalkSkipsCopiesSyncToolsMakeBesideARealFile(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"note.md",
		"note.sync-conflict-20260923-101010-ABC1234.md",
		"~syncthing~note.md.tmp.md",
		"note (conflicted copy 2026-09-23).md",
		"note (Conflict 2026-09-23).md",
		".stversions/note.md",
	} {
		writeFile(t, filepath.Join(root, name), "# Note\n")
	}
	stamps, err := walk(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for name := range stamps {
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"note.md"}) {
		t.Fatalf("walk indexed %v", names)
	}
}
