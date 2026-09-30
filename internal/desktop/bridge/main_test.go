package main

import (
	"bytes"
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

// A set whose constants were renamed or retyped would otherwise vanish from the
// page, and every comparison against it would quietly stop matching.
func TestAValueSetThatMatchesNothingStopsTheGenerator(t *testing.T) {
	dir := t.TempDir()
	source := "package session\n\ntype RepoRole string\n\nconst Editing RepoRole = \"editing\"\n"
	if err := os.WriteFile(filepath.Join(dir, "manifest.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var js bytes.Buffer
	sets := []valueSet{{name: "REPO_ROLES", typedef: "RepoRole", dir: dir, goType: "RepoRole", prefix: "RepoRole"}}
	if err := writeValues(&js, sets, nil); err == nil || !strings.Contains(err.Error(), "REPO_ROLES") {
		t.Fatalf("a set with no constants generated anyway: %v", err)
	}
	if err := writeValues(&js, nil, []valueName{{dir, "agentRootID"}}); err == nil {
		t.Fatal("a missing single value generated anyway")
	}
}

func TestAValueSetKeepsSourceOrderAndItsOwnType(t *testing.T) {
	dir := t.TempDir()
	source := "package session\n\ntype Sticker string\ntype Other string\n\nconst (\n" +
		"\tStickerStar Sticker = \"star\"\n\tStickerOther Other = \"other\"\n\tStickerBookmark Sticker = \"bookmark\"\n)\n"
	if err := os.WriteFile(filepath.Join(dir, "manifest.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var js bytes.Buffer
	sets := []valueSet{{name: "STICKERS", typedef: "Sticker", dir: dir, goType: "Sticker", prefix: "Sticker"}}
	if err := writeValues(&js, sets, nil); err != nil {
		t.Fatal(err)
	}
	want := "/** @typedef {\"star\"|\"bookmark\"} Sticker */\nexport const STICKERS = Object.freeze({\n" +
		"  STAR: \"star\",\n  BOOKMARK: \"bookmark\",\n});\n\n"
	if js.String() != want {
		t.Fatalf("generated\n%s\nwant\n%s", js.String(), want)
	}
}
