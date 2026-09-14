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

func TestImageReferencesPreservesOrderAliasesAndDuplicates(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"z last.PNG", "é first.webp"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("image"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	abs := filepath.Join(root, "é first.webp")
	refs, err := ImageReferences(root, []string{"z last.PNG", abs, "z last.PNG"})
	if err != nil {
		t.Fatal(err)
	}
	want := []workbench.ImageRef{{Source: "z last.PNG"}, {Source: "é first.webp"}, {Source: "z last.PNG"}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs = %#v, want %#v", refs, want)
	}
}

func TestImageReferencesAcceptsRasterFormatsThoughtsAndLargeFiles(t *testing.T) {
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
		body := []byte("image")
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
	refs, err := ImageReferences(root, inputs)
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

func TestImageReferencesRejectsInvalidEntriesWithoutExpanding(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.png")
	_ = os.WriteFile(outside, []byte("image"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "vector.svg"), []byte("svg"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "clip.mp4"), []byte("video"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "note.txt"), []byte("text"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "actual.png"), []byte("image"), 0o644)
	_ = os.Mkdir(filepath.Join(root, "folder.png"), 0o755)
	_ = os.Symlink(outside, filepath.Join(root, "escape.png"))
	for name, paths := range map[string][]string{
		"empty list": {}, "empty member": {"actual.png", ""}, "missing": {"missing.png"},
		"directory": {"folder.png"}, "outside": {outside}, "external symlink": {"escape.png"}, "svg": {"vector.svg"},
		"video": {"clip.mp4"}, "unsupported": {"note.txt"}, "literal glob": {"*.png"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ImageReferences(root, paths); err == nil {
				t.Fatal("accepted invalid image input")
			}
		})
	}
	if _, err := ImageReferences(root, nil); !errors.Is(err, ErrImagePathsRequired) {
		t.Fatalf("empty = %v", err)
	}
}

func TestImageReferencesRejectsUnreadableRegularFile(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "locked.png")
	if err := os.WriteFile(name, []byte("image"), 0o000); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(name)
	if err == nil {
		file.Close()
		t.Skip("current user can read mode 000 files")
	}
	if _, err := ImageReferences(root, []string{"locked.png"}); err == nil {
		t.Fatal("accepted unreadable file")
	}
}

func TestRasterMediaTypes(t *testing.T) {
	for name, want := range map[string]string{"x.PNG": "image/png", "x.jpg": "image/jpeg", "x.JPEG": "image/jpeg", "x.gif": "image/gif", "x.webp": "image/webp", "x.avif": "image/avif"} {
		if got, ok := workbench.RasterMediaType(name); !ok || got != want {
			t.Errorf("%s = %q, %v", name, got, ok)
		}
	}
	if _, ok := workbench.RasterMediaType("x.svg"); ok {
		t.Fatal("accepted SVG")
	}
}
