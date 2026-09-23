package vault

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRefusesRootsThatCouldReachEachOther(t *testing.T) {
	base := t.TempDir()
	shared, private := filepath.Join(base, "shared"), filepath.Join(base, "private")
	for _, dir := range []string{shared, private} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(shared, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		profiles []Profile
		want     error
	}{
		{"separate roots", []Profile{{"shared", shared}, {"private", private}}, nil},
		{"a missing root", []Profile{{"shared", filepath.Join(base, "later")}}, nil},
		{"duplicate id", []Profile{{"shared", shared}, {"shared", private}}, ErrDuplicateProfile},
		{"duplicate id in another case", []Profile{{"shared", shared}, {"Shared", private}}, ErrDuplicateProfile},
		{"same root", []Profile{{"shared", shared}, {"private", shared + "/"}}, ErrOverlappingRoots},
		{"nested root", []Profile{{"shared", shared}, {"inner", filepath.Join(shared, "inner")}}, ErrOverlappingRoots},
		{"enclosing root", []Profile{{"shared", shared}, {"all", base}}, ErrOverlappingRoots},
		{"symlinked root", []Profile{{"shared", shared}, {"link", link}}, ErrOverlappingRoots},
		{"relative root", []Profile{{"shared", "shared"}}, ErrInvalidProfile},
		{"one root spelt in another case", []Profile{{"shared", shared}, {"upper", filepath.Join(base, "SHARED")}},
			caseInsensitive(base)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.profiles); !errors.Is(err, tc.want) {
				t.Fatalf("Validate = %v, want %v", err, tc.want)
			}
		})
	}
}

// caseInsensitive answers the error a second spelling of one root should get.
func caseInsensitive(base string) error {
	if _, err := os.Stat(filepath.Join(base, "SHARED")); err == nil {
		return ErrOverlappingRoots
	}
	return nil
}

func TestASetAnswersOnlyTheRootAPathSitsIn(t *testing.T) {
	base := t.TempDir()
	shared, private := filepath.Join(base, "shared"), filepath.Join(base, "private")
	writeFile(t, filepath.Join(shared, "s1", "roadmap.md"), "# Wombat roadmap\n\nShared wombat burrow survey.\n")
	writeFile(t, filepath.Join(private, "s2", "diary.md"), "# Wombat diary\n\nPrivate wombat burrow notes.\n")
	set := NewSet(&stubEmbedder{}, t.TempDir())
	if err := set.Configure([]Profile{{"shared", shared}, {"private", private}}); err != nil {
		t.Fatal(err)
	}

	service := set.ForPath(resolve(filepath.Join(shared, "s1")))
	if service == nil {
		t.Fatal("no service for a path in the shared root")
	}
	res, err := service.Search(context.Background(), Query{Text: "wombat burrow"})
	if err != nil || len(res.Hits) != 1 || res.Hits[0].Profile != "shared" {
		t.Fatalf("shared search = %+v, %v", res, err)
	}
	for _, ref := range []string{"private:s2/diary.md", "s2/diary", "shared:../private/s2/diary.md"} {
		if _, err := service.Read(ReadRequest{Ref: ref}); !errors.Is(err, ErrUnknownDocument) {
			t.Errorf("read %q from the shared root = %v", ref, err)
		}
	}
	if service := set.ForPath(base); service != nil {
		t.Fatalf("a path outside every root answered %s", service.profile.ID)
	}
}

func TestConfigureKeepsUnchangedIndexesAndDropsRemovedOnes(t *testing.T) {
	base := t.TempDir()
	profiles := []Profile{{"shared", filepath.Join(base, "shared")}, {"private", filepath.Join(base, "private")}}
	set := NewSet(&stubEmbedder{}, t.TempDir())
	if err := set.Configure(profiles); err != nil {
		t.Fatal(err)
	}
	kept := set.services["shared"]
	if err := set.Configure(profiles[:1]); err != nil {
		t.Fatal(err)
	}
	if set.services["shared"] != kept || set.services["private"] != nil {
		t.Fatalf("services = %v", set.services)
	}
	if err := set.Configure([]Profile{profiles[0], {"inner", filepath.Join(profiles[0].Root, "inner")}}); !errors.Is(err, ErrOverlappingRoots) {
		t.Fatalf("overlapping configure = %v", err)
	}
	if set.services["shared"] != kept {
		t.Fatal("a refused configure replaced the set")
	}
}
