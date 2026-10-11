package launch

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/workbench"
)

func TestAssetReferencesPreservesOrderAliasesAndDuplicates(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"z last.PNG", "é first.webp"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("asset"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	abs := filepath.Join(root, "é first.webp")
	refs, err := AssetReferences(root, []string{"z last.PNG", abs, "z last.PNG"})
	if err != nil {
		t.Fatal(err)
	}
	want := []workbench.AssetRef{{Source: "z last.PNG"}, {Source: "é first.webp"}, {Source: "z last.PNG"}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs = %#v, want %#v", refs, want)
	}
}

func TestAssetReferencesAcceptsRasterFormatsThoughtsAndLargeFiles(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "session")
	home := filepath.Join(parent, "artifacts")
	if err := os.MkdirAll(filepath.Join(home, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(home, filepath.Join(root, "thoughts")); err != nil {
		t.Fatal(err)
	}
	names := []string{"a.png", "b.JPG", "c.JpEg", "d.gif", "e.webp", "large.AVIF"}
	for _, name := range names {
		body := []byte("asset")
		if strings.HasPrefix(name, "large") {
			body = make([]byte, workbench.DocumentLimit+1)
		}
		if err := os.WriteFile(filepath.Join(home, "assets", name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := make([]string, len(names))
	for i, name := range names {
		inputs[i] = filepath.Join("thoughts", "assets", name)
	}
	inputs[0] = filepath.Join(home, "assets", names[0])
	refs, err := AssetReferences(root, inputs)
	if err != nil || len(refs) != len(names) {
		t.Fatalf("refs = %#v, %v", refs, err)
	}
	for i, ref := range refs {
		want := filepath.ToSlash(filepath.Join("thoughts", "assets", names[i]))
		if ref.Source != want {
			t.Errorf("ref %d = %q, want %q", i, ref.Source, want)
		}
	}
}

func TestAssetReferencesRejectsInvalidEntriesWithoutExpanding(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.png")
	_ = os.WriteFile(outside, []byte("asset"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "vector.svg"), []byte("svg"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "note.txt"), []byte("text"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "actual.png"), []byte("asset"), 0o644)
	_ = os.Mkdir(filepath.Join(root, "folder.png"), 0o755)
	_ = os.Symlink(outside, filepath.Join(root, "escape.png"))
	for name, paths := range map[string][]string{
		"empty list": {}, "empty member": {"actual.png", ""}, "missing": {"missing.png"},
		"directory": {"folder.png"}, "outside": {outside}, "external symlink": {"escape.png"}, "svg": {"vector.svg"},
		"unsupported": {"note.txt"}, "literal glob": {"*.png"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := AssetReferences(root, paths); err == nil {
				t.Fatal("accepted invalid asset input")
			}
		})
	}
	if _, err := AssetReferences(root, nil); !errors.Is(err, ErrAssetPathsRequired) {
		t.Fatalf("empty = %v", err)
	}
}

func TestAssetReferencesRejectsUnreadableRegularFile(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "locked.png")
	if err := os.WriteFile(name, []byte("asset"), 0o000); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(name)
	if err == nil {
		file.Close()
		t.Skip("current user can read mode 000 files")
	}
	if _, err := AssetReferences(root, []string{"locked.png"}); err == nil {
		t.Fatal("accepted unreadable file")
	}
}

func TestAssetReferencesAdmitsMixedMediaInOrder(t *testing.T) {
	root := t.TempDir()
	names := []string{"a.png", "b.mp4", "c.MOV", "d.webm", "a.png"}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte("asset"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	refs, err := AssetReferences(root, names)
	if err != nil || len(refs) != len(names) {
		t.Fatalf("refs = %+v, %v", refs, err)
	}
	for i, ref := range refs {
		if ref.Source != names[i] {
			t.Errorf("refs[%d] = %q, want %q", i, ref.Source, names[i])
		}
	}
}

func TestAssetReferencesJudgesTheSymlinkTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("notes.txt", filepath.Join(root, "clip.mp4")); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := AssetReferences(root, []string{"clip.mp4"}); !errors.Is(err, ErrUnsupportedAsset) {
		t.Fatalf("err = %v", err)
	}
}
