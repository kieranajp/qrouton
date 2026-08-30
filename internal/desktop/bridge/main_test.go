package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A page built from a stale module calls a name the workbench no longer binds,
// or defaults a field that has since been added, and neither is visible until a
// window opens on it.
func TestTheCommittedModuleIsWhatTheGoSourceProduces(t *testing.T) {
	want, err := render(desktopPackage, statusPackage, moduleFile)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(generatedPage)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s no longer matches the workbench it is generated from; run make front", generatedPage)
	}
}

// A field whose type the generator has no default for is left out of the page
// silently, so it stops the build instead.
func TestAFieldWithNoDefaultStopsTheGenerator(t *testing.T) {
	dir := t.TempDir()
	source := "package status\n\ntype Fields struct {\n\tAt map[string]int `json:\"at\"`\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "status.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := render(desktopPackage, dir, moduleFile)
	if err == nil {
		t.Fatal("a field the page cannot be given a default for generated anyway")
	}
	if !strings.Contains(err.Error(), "at") {
		t.Fatalf("the failure does not name the field: %v", err)
	}
}

func TestBoundNamesAreSpelledTheWayTheModuleImportsThem(t *testing.T) {
	for _, spelling := range []struct{ go_, js string }{
		{"Term", "TERM"},
		{"FirstRun", "FIRST_RUN"},
		{"OpenDocument", "OPEN_DOCUMENT"},
		{"ptyDataEvent", "PTY_DATA_EVENT"},
	} {
		if got := screaming(spelling.go_); got != spelling.js {
			t.Errorf("screaming(%q) = %q, want %q", spelling.go_, got, spelling.js)
		}
	}
}
