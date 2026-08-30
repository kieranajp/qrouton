package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
)

var entropySuffix = regexp.MustCompile(`^[0-9a-f]{4}$`)

func TestSessionSlugAddsSuppliedEntropyWithoutChangingTheDisplayName(t *testing.T) {
	if got := SessionSlug("API Cleanup!", "4F3A"); got != "api-cleanup-4f3a" {
		t.Fatalf("session slug = %q", got)
	}
	if got := SessionSlug("API Cleanup!", ""); got != "api-cleanup" {
		t.Fatalf("legacy session slug = %q", got)
	}
}

// The suffix is entropy, not a constant: a handful of them must not all agree.
func TestNewEntropyIsShortAndVaries(t *testing.T) {
	seen := map[string]bool{}
	for range 5 {
		got := NewEntropy()
		if !entropySuffix.MatchString(got) {
			t.Fatalf("entropy = %q", got)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Fatalf("session slugs carry no entropy: %v", seen)
	}
}

func TestCreateZeroRepoScratchSession(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "lifesum-4f3a", Mode: ModeAssistant,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	sessions, err := Scan(root)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("scan = %v, %v", sessions, err)
	}
	m := sessions[0]
	if m.Slug != "lifesum-4f3a" || m.Mode != ModeAssistant || len(m.Repos) != 0 {
		t.Fatalf("scratch manifest = %+v", m)
	}

	// thoughts is a relative symlink into the root's thoughts home, and the
	// scaffold resolves through it.
	target, err := os.Readlink(filepath.Join(dir, "thoughts"))
	if err != nil {
		t.Fatal("thoughts is not a symlink:", err)
	}
	if want := filepath.Join("..", "thoughts", "lifesum-4f3a"); target != want {
		t.Fatalf("thoughts -> %q, want %q", target, want)
	}
	for _, d := range scaffoldDirs {
		fi, err := os.Stat(filepath.Join(dir, "thoughts", "shared", d))
		if err != nil || !fi.IsDir() {
			t.Fatalf("scaffold %s not reachable through the link: %v", d, err)
		}
	}
}

func TestDeleteKeepsDocumentsWrittenThroughTheThoughtsLink(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, CreateRequest{Name: "scratch-1a2b", Mode: ModeAssistant}, nil)
	if err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "thoughts", "shared", "research", "R1-findings.md")
	if err := os.WriteFile(doc, []byte("findings"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	// Status still infers phase through the symlink.
	if got := Status(root, m); !got.Research {
		t.Fatalf("status through symlink = %#v", got)
	}

	if err := Delete(root, m); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("session still exists after delete: %v", err)
	}
	survivor := filepath.Join(root, "thoughts", "scratch-1a2b", "shared", "research", "R1-findings.md")
	if _, err := os.Stat(survivor); err != nil {
		t.Fatal("documents died with the session:", err)
	}
}
