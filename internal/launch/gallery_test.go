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
	for path, content := range map[string]string{
		outside:                           "asset",
		filepath.Join(root, "vector.svg"): "svg",
		filepath.Join(root, "note.txt"):   "text",
		filepath.Join(root, "actual.png"): "asset",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "folder.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.png")); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		paths []string
		want  error
	}{
		"empty list":       {[]string{}, ErrAssetPathsRequired},
		"nil list":         {nil, ErrAssetPathsRequired},
		"empty member":     {[]string{"actual.png", ""}, ErrAssetPathRequired},
		"missing":          {[]string{"missing.png"}, ErrOutsideSessionMissing},
		"literal glob":     {[]string{"*.png"}, ErrOutsideSessionMissing},
		"directory":        {[]string{"folder.png"}, ErrNotRegularFile},
		"outside":          {[]string{outside}, ErrOutsideSession},
		"external symlink": {[]string{"escape.png"}, ErrOutsideSession},
		"svg":              {[]string{"vector.svg"}, ErrUnsupportedAsset},
		"unsupported":      {[]string{"note.txt"}, ErrUnsupportedAsset},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := AssetReferences(root, tc.paths); !errors.Is(err, tc.want) {
				t.Fatalf("AssetReferences(%q) = %v, want %v", tc.paths, err, tc.want)
			}
		})
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
	names := []string{"a.png", "b.mp4", "c.MOV", "d.m4v", "a.png"}
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

func TestAssetReferencesRefusesWebM(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "clip.webm"), []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := AssetReferences(root, []string{"clip.webm"}); !errors.Is(err, ErrUnsupportedAsset) {
		t.Fatalf("webm = %v, want %v", err, ErrUnsupportedAsset)
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
