package artefact

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/kieranajp/qrouton/internal/gittest"
	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/status"
)

var now = time.Date(2026, 10, 12, 9, 30, 0, 0, time.Local)

func fixedAuthor(t *testing.T, name string) {
	t.Helper()
	oldConfig, oldGit, oldOS := configAuthor, gitAuthor, osAuthor
	t.Cleanup(func() { configAuthor, gitAuthor, osAuthor = oldConfig, oldGit, oldOS })
	configAuthor = func() string { return name }
	gitAuthor = func() string { return "" }
	osAuthor = func() string { return "" }
}

func newSession(t *testing.T, repos ...session.ManifestRepo) string {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	err := session.WriteManifest(root, session.Manifest{SchemaVersion: 2, Name: "Team sync", Slug: "team-sync", Repos: repos})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func frontmatterOf(t *testing.T, root, rel string) (map[string]any, string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	head, body, ok := strings.Cut(strings.TrimPrefix(string(b), "---\n"), "\n---\n")
	if !ok {
		t.Fatalf("no frontmatter in %q", b)
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(head), &fm); err != nil {
		t.Fatal(err)
	}
	return fm, body
}

func TestCreateNamesAndStampsAResearchDocument(t *testing.T) {
	fixedAuthor(t, "Kieran")
	root := newSession(t)
	rel, err := Create(root, "research", "Team: sync, \"quoted\"", now)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("thoughts", "shared", "research", "R1-2026-10-12-team-sync-quoted.md")
	if rel != want {
		t.Fatalf("path = %q, want %q", rel, want)
	}
	fm, body := frontmatterOf(t, root, rel)
	for key, want := range map[string]any{
		"kind": "research", "title": "Team: sync, \"quoted\"", "author": "Kieran",
		"created": now.Format(time.RFC3339), "session": "team-sync",
	} {
		if fm[key] != want {
			t.Errorf("%s = %v, want %v", key, fm[key], want)
		}
	}
	if !strings.Contains(body, "# Team: sync, \"quoted\"") || strings.Contains(body, "<title>") {
		t.Fatalf("title not filled in body:\n%s", body)
	}
	b, _ := os.ReadFile(filepath.Join(root, rel))
	if title, ok := markdown.Title(string(b)); !ok || title != "Team: sync, \"quoted\"" {
		t.Fatalf("markdown title = %q, %v", title, ok)
	}
}

func TestCreateNumbersPastGapsAndIgnoresConflictCopies(t *testing.T) {
	fixedAuthor(t, "Kieran")
	root := newSession(t)
	dir := filepath.Join(sessionpaths.Thoughts(root), "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"P1-a.md", "P4-b.md", "P9-b.sync-conflict-20261012-120000-ABCDEFG.md", ".P12-hidden.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rel, err := Create(root, "plan", "Next", now)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(rel) != "P5-2026-10-12-next.md" {
		t.Fatalf("path = %q", rel)
	}
	if got := status.ArtifactID(rel); got != "P5" {
		t.Fatalf("id = %q", got)
	}
}

func TestCreateRetriesWhenTheNameIsTaken(t *testing.T) {
	fixedAuthor(t, "Kieran")
	root := newSession(t)
	dir := filepath.Join(sessionpaths.Thoughts(root), "specs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "S1-2026-10-12-same.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := nextNumber
	t.Cleanup(func() { nextNumber = old })
	nextNumber = func(string) int { return 1 }
	rel, err := Create(root, "spec", "Same", now)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(rel) != "S2-2026-10-12-same.md" {
		t.Fatalf("path = %q, want the retry to take S2", rel)
	}
}

func TestConcurrentCreatesWithDifferentTitlesGetDistinctNumbers(t *testing.T) {
	fixedAuthor(t, "Kieran")
	root := newSession(t)
	const n = 12
	paths := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rel, err := Create(root, "research", fmt.Sprintf("Topic %d", i), now)
			if err != nil {
				t.Error(err)
			}
			paths[i] = rel
		}()
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, rel := range paths {
		id := status.ArtifactID(rel)
		if id == "" || seen[id] {
			t.Fatalf("ids repeat or are missing: %v", paths)
		}
		seen[id] = true
	}
}

func TestALongTitleGivesACappedSlugCutAtAHyphen(t *testing.T) {
	fixedAuthor(t, "Kieran")
	root := newSession(t)
	rel, err := Create(root, "plan", strings.Repeat("retry budgets ", 25), now)
	if err != nil {
		t.Fatal(err)
	}
	slug := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(rel), "P1-2026-10-12-"), ".md")
	if len(slug) > slugLimit || strings.HasSuffix(slug, "-") || strings.HasPrefix(slug, "-") || !strings.HasSuffix(slug, "budgets") {
		t.Fatalf("slug = %q", slug)
	}
	if got := capSlug(strings.Repeat("a", 100)); len(got) != slugLimit {
		t.Fatalf("an unbroken slug was cut to %d", len(got))
	}
}

func TestCreateWritesTheSpecsOwnKeysAfterTheStamp(t *testing.T) {
	fixedAuthor(t, "Kieran")
	root := newSession(t)
	rel, err := Create(root, "spec", "Shape", now)
	if err != nil {
		t.Fatal(err)
	}
	fm, body := frontmatterOf(t, root, rel)
	if v, ok := fm["research"]; !ok || v != nil {
		t.Fatalf("research = %v, present %v", v, ok)
	}
	if !strings.Contains(body, "## Open questions\n\nNone.") {
		t.Fatalf("body:\n%s", body)
	}
}

func TestCreateRejectsAnUnknownKindNamingTheValidOnes(t *testing.T) {
	root := newSession(t)
	_, err := Create(root, "notes", "x", now)
	if !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("err = %v", err)
	}
	for _, kind := range Kinds {
		if !strings.Contains(err.Error(), kind) {
			t.Errorf("error %q does not name %q", err, kind)
		}
	}
	if _, statErr := os.Stat(sessionpaths.Thoughts(root)); statErr == nil {
		t.Fatal("a rejected kind left thoughts behind")
	}
}

func TestCreateRecordsRevisionsForReferenceAndEditingRepos(t *testing.T) {
	fixedAuthor(t, "Kieran")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	root := t.TempDir()
	worktree := gittest.Worktree(t, filepath.Join(root, "src", "app"))
	head := gittest.Head(t, worktree)
	pinned := strings.Repeat("a", 40)
	err := session.WriteManifest(root, session.Manifest{SchemaVersion: 2, Slug: "s", Repos: []session.ManifestRepo{
		{Org: "o", Name: "app", Role: session.RepoRoleEditing, WorktreePath: "src/app"},
		{Org: "o", Name: "lib", Role: session.RepoRoleReference, Revision: pinned, WorktreePath: "src/lib"},
		{Org: "o", Name: "gone", Role: session.RepoRoleEditing, WorktreePath: "src/gone"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	rel, err := Create(root, "research", "Revs", now)
	if err != nil {
		t.Fatal(err)
	}
	fm, _ := frontmatterOf(t, root, rel)
	revisions, _ := fm["revisions"].(map[string]any)
	if len(revisions) != 2 || revisions["o/app"] != head || revisions["o/lib"] != pinned {
		t.Fatalf("revisions = %v, want app at %s and lib at %s only", revisions, head, pinned)
	}
}

func TestAuthorFallsBackFromConfigToGitToTheLoginName(t *testing.T) {
	oldConfig, oldGit, oldOS := configAuthor, gitAuthor, osAuthor
	t.Cleanup(func() { configAuthor, gitAuthor, osAuthor = oldConfig, oldGit, oldOS })
	for _, tc := range []struct{ config, git, os, want string }{
		{"Cfg", "Git", "login", "Cfg"},
		{"", "Git\n", "login", "Git"},
		{"  ", "", "login", "login"},
		{"", "", "", ""},
	} {
		configAuthor = func() string { return tc.config }
		gitAuthor = func() string { return tc.git }
		osAuthor = func() string { return tc.os }
		if got := author(); got != tc.want {
			t.Errorf("author(%+v) = %q, want %q", tc, got, tc.want)
		}
	}
}

func TestCreateRefusesAKindFolderThatLinksOutOfTheThoughts(t *testing.T) {
	fixedAuthor(t, "Kieran")
	root := newSession(t)
	outside := t.TempDir()
	shared := sessionpaths.Thoughts(root)
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(shared, "research")); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(root, "research", "Escape", now); !errors.Is(err, ErrOutsideThoughts) {
		t.Fatalf("err = %v", err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("a file was written outside the thoughts: %v", entries)
	}
}
