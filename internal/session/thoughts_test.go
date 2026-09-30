package session

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// routedConfig maps org acme to a shared root beside the sessions root.
func routedConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	base := t.TempDir()
	root, shared := filepath.Join(base, "sessions"), filepath.Join(base, "shared")
	for _, dir := range []string{root, shared} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &config.Config{Root: root, Thoughts: config.Thoughts{Roots: []config.ThoughtsRoot{
		{ID: "work", Path: shared, Orgs: []string{"acme"}},
	}}}, shared
}

func orgRepo(t *testing.T, org, name string) RepoSelection {
	t.Helper()
	origin, _ := makeOrigin(t, name)
	return RepoSelection{Repo: github.Repo{Name: name, Org: org, SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleEditing}
}

func loadManifest(t *testing.T, dir string) Manifest {
	t.Helper()
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCreateRoutesASessionWhoseReposAllMapToOneRoot(t *testing.T) {
	cfg, shared := routedConfig(t)
	dir, err := Create(cfg, CreateRequest{Name: "Routed", Slug: "routed", Prefix: "feat",
		Repos: []RepoSelection{orgRepo(t, "acme", "api"), orgRepo(t, "ACME", "web")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	link, err := os.Readlink(filepath.Join(dir, sessionpaths.ThoughtsDirName))
	if err != nil || link != filepath.Join(shared, "routed") {
		t.Fatalf("thoughts -> %q, %v", link, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "thoughts", "shared", "research")); err != nil {
		t.Fatal("scaffold not reachable through the link:", err)
	}
	if m := loadManifest(t, dir); m.ThoughtsRoot != "work" {
		t.Fatalf("thoughtsRoot = %q", m.ThoughtsRoot)
	}
}

func TestCreateKeepsMixedAndUnmappedSessionsInTheDefaultRoot(t *testing.T) {
	for name, repos := range map[string]func(t *testing.T) []RepoSelection{
		"mixed": func(t *testing.T) []RepoSelection {
			return []RepoSelection{orgRepo(t, "acme", "api"), orgRepo(t, "jdoe", "dots")}
		},
		"unmapped":  func(t *testing.T) []RepoSelection { return []RepoSelection{orgRepo(t, "jdoe", "dots")} },
		"repo-free": func(*testing.T) []RepoSelection { return nil },
	} {
		t.Run(name, func(t *testing.T) {
			cfg, shared := routedConfig(t)
			dir, err := Create(cfg, CreateRequest{Name: name, Prefix: "feat", Repos: repos(t)}, nil)
			if err != nil {
				t.Fatal(err)
			}
			slug := filepath.Base(dir)
			if link, _ := os.Readlink(filepath.Join(dir, "thoughts")); link != filepath.Join("..", "thoughts", slug) {
				t.Fatalf("thoughts -> %q", link)
			}
			if m := loadManifest(t, dir); m.ThoughtsRoot != config.DefaultThoughtsID {
				t.Fatalf("thoughtsRoot = %q", m.ThoughtsRoot)
			}
			if entries, _ := os.ReadDir(shared); len(entries) != 0 {
				t.Fatalf("shared root gained %v", entries)
			}
		})
	}
}

func TestCreateFallsBackToTheDefaultRootWhenTheMappedOneIsMissing(t *testing.T) {
	cfg, shared := routedConfig(t)
	if err := os.Remove(shared); err != nil {
		t.Fatal(err)
	}
	dir, err := Create(cfg, CreateRequest{Name: "Fallback", Slug: "fallback", Prefix: "feat",
		Repos: []RepoSelection{orgRepo(t, "acme", "api")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if link, _ := os.Readlink(filepath.Join(dir, "thoughts")); link != filepath.Join("..", "thoughts", "fallback") {
		t.Fatalf("thoughts -> %q", link)
	}
	if m := loadManifest(t, dir); m.ThoughtsRoot != config.DefaultThoughtsID {
		t.Fatalf("thoughtsRoot = %q", m.ThoughtsRoot)
	}
	notice, err := os.ReadFile(sessionpaths.AgentNotice(dir))
	if err != nil || !strings.Contains(string(notice), `"work"`) || !strings.Contains(string(notice), shared) {
		t.Fatalf("notice = %q, %v", notice, err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatal("creation made the missing root")
	}
}

func TestCreateRerollsASlugTheMappedRootAlreadyHolds(t *testing.T) {
	suffixed := regexp.MustCompile(`^fix-login-[0-9a-f]{4}$`)
	for _, tc := range []struct{ slug, entropy string }{
		{"fix-login-ab12", "ab12"},
		{"fix-login", ""},
	} {
		cfg, shared := routedConfig(t)
		if err := os.Mkdir(filepath.Join(shared, tc.slug), 0o755); err != nil {
			t.Fatal(err)
		}
		dir, err := Create(cfg, CreateRequest{Name: "Fix login", Slug: tc.slug, Entropy: tc.entropy, Prefix: "fix",
			Repos: []RepoSelection{orgRepo(t, "acme", "api")}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		slug := filepath.Base(dir)
		if slug == tc.slug || !suffixed.MatchString(slug) {
			t.Fatalf("slug %q from %q", slug, tc.slug)
		}
		m := loadManifest(t, dir)
		if m.Slug != slug || m.Branch() != "fix/"+slug {
			t.Fatalf("manifest slug %q branch %q, want %q", m.Slug, m.Branch(), slug)
		}
		if link, _ := os.Readlink(filepath.Join(dir, "thoughts")); link != filepath.Join(shared, slug) {
			t.Fatalf("thoughts -> %q", link)
		}
	}
}
