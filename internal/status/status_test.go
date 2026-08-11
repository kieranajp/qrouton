package status

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
)

// sessionDir writes a session directory with the given manifest under a fake
// qrouton root and returns it — the chrome only ever reads.
func sessionDir(t *testing.T, root string, m session.Manifest) string {
	t.Helper()
	dir := filepath.Join(root, m.Slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qrouton.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReadScratchAssistant(t *testing.T) {
	dir := sessionDir(t, t.TempDir(), session.Manifest{Slug: "lifesum-4f3a", Mode: session.ModeAssistant})
	got, ok := Read(dir)
	if !ok {
		t.Fatal("Read reported no manifest")
	}
	want := Fields{
		Mode:     "ASSISTANT",
		Phase:    "scratch",
		Identity: "lifesum-4f3a",
		Sessions: []SessionRow{
			{Name: "lifesum-4f3a", Slug: "lifesum-4f3a", Initials: "l4", Mode: "ASSISTANT", Live: true},
		},
		Documents: []Document{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %#v, want %#v", got, want)
	}
}

func TestReadEscalatedShowsPhaseNameAndBranch(t *testing.T) {
	root := t.TempDir()
	m := session.Manifest{Slug: "webhook", Name: "webhook retry backoff", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{
			{Name: "consumer", Org: "org", Role: session.RepoRoleReference, Revision: "41c3", WorktreePath: "src/consumer"},
			{Name: "svc", Org: "org", Branch: "fix/webhook-retry-backoff", WorktreePath: "src/svc"},
		}}
	dir := sessionDir(t, root, m)

	// No documents yet: freshly escalated means Research.
	got, ok := Read(dir)
	if !ok {
		t.Fatal("Read reported no manifest")
	}
	if got.Mode != "RPI" || got.Phase != "Research" {
		t.Fatalf("fields = %#v", got)
	}
	if got.Identity != "webhook retry backoff" || got.Branch != "fix/webhook-retry-backoff" {
		t.Fatalf("identity = %q, branch = %q", got.Identity, got.Branch)
	}

	// A research document moves the session into planning.
	research := filepath.Join(dir, "thoughts", "shared", "research")
	if err := os.MkdirAll(research, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(research, "r1-findings.md"), []byte("# findings"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Read(dir); got.Phase != "Plan" {
		t.Fatalf("phase did not advance to Plan: %q", got.Phase)
	}

	// A plan document moves it into implementation.
	plans := filepath.Join(dir, "thoughts", "shared", "plans")
	if err := os.MkdirAll(plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plans, "p1.md"), []byte("- [ ] step"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := Read(dir); got.Phase != "Implement" {
		t.Fatalf("phase did not advance to Implement: %q", got.Phase)
	}
}

func TestReadWithoutManifest(t *testing.T) {
	got, ok := Read(t.TempDir())
	if ok || !reflect.DeepEqual(got, Fields{}) {
		t.Fatalf("missing manifest = %#v, %v", got, ok)
	}
}

func TestDocumentsAreClassifiedByFilenamePrefix(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "webhook"})
	shared := filepath.Join(dir, "thoughts", "shared")
	kinds := []struct{ path, want string }{
		{"readme.md", "NOTE"},
		{"research/questions-about-p1.md", "NOTE"},
		{"research/parity-checklist.md", "NOTE"},
		{"research/R7-editor.md", "RESEARCH"},
		{"plans/p002-orchestrated.md", "PLAN"},
		{"specs/S006-gui-workbench.md", "SPEC"},
		{"plans/P006-design-system.md", "PLAN"},
	}
	base := time.Now().Add(-time.Hour)
	for i, doc := range kinds {
		full := filepath.Join(shared, doc.path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		mtime := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(full, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		if got := kind(filepath.Base(doc.path)); got != doc.want {
			t.Errorf("kind(%q) = %q, want %q", doc.path, got, doc.want)
		}
	}

	fields, _ := Read(dir)
	if len(fields.Documents) != len(kinds) {
		t.Fatalf("found %d documents, want %d: %#v", len(fields.Documents), len(kinds), fields.Documents)
	}
	// Newest first: the header announces exactly one.
	for i, got := range fields.Documents {
		doc := kinds[len(kinds)-1-i].path
		if got.Name != filepath.Base(doc) {
			t.Fatalf("document %d = %q, want %q (not newest-first): %#v", i, got.Name, doc, fields.Documents)
		}
		// The path is resolved against the session root, so an absolute one gets
		// past the escape guard.
		if want := filepath.Join("thoughts", "shared", doc); got.Path != want {
			t.Fatalf("document %d is at %q, want %q", i, got.Path, want)
		}
	}
}

func TestSessionsListsEverySessionAndMarksTheLiveOne(t *testing.T) {
	root := t.TempDir()
	sessionDir(t, root, session.Manifest{Slug: "flaky-suite", Name: "Flaky suite", Mode: session.ModeAssistant})
	dir := sessionDir(t, root, session.Manifest{Slug: "extract-billing", Name: "Extract billing", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{{Name: "api", Org: "lifesum"}}})

	fields, _ := Read(dir)
	if len(fields.Sessions) != 2 {
		t.Fatalf("listed %d sessions, want 2", len(fields.Sessions))
	}
	live := fields.Sessions[0]
	if !live.Live || live.Slug != "extract-billing" {
		t.Fatalf("live row = %#v", live)
	}
	if live.Initials != "eb" || live.Mode != "RPI" || live.Repos != 1 {
		t.Fatalf("live row = %#v", live)
	}
	if fields.Sessions[1].Live || fields.Sessions[1].Initials != "fs" {
		t.Fatalf("second row = %#v", fields.Sessions[1])
	}
}

func TestInitialsSurviveShortAndPunctuatedNames(t *testing.T) {
	for name, want := range map[string]string{
		"Extract billing service": "eb",
		"webhook":                 "we",
		"a":                       "a",
		"":                        "",
		"lifesum-4f3a":            "l4",
	} {
		if got := initials(name); got != want {
			t.Errorf("initials(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestReposMeasuresActiveRepositoriesAndSkipsReferences(t *testing.T) {
	root := t.TempDir()
	m := session.Manifest{Slug: "webhook", Name: "webhook", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{
			{Name: "svc", Org: "org", Role: session.RepoRoleActive, DefaultBranch: "main",
				Branch: "fix/webhook", WorktreePath: "src/svc"},
			{Name: "quiet", Org: "org", Role: session.RepoRoleActive, DefaultBranch: "main",
				Branch: "fix/webhook", WorktreePath: "src/quiet"},
			{Name: "docs", Org: "org", Role: session.RepoRoleReference, WorktreePath: "src/docs"},
		}}
	dir := sessionDir(t, root, m)

	// svc: one commit ahead of origin/main, adding one line.
	svc := fixtureRepo(t, filepath.Join(dir, "src", "svc"))
	writeFile(t, svc, "invoice.go", "package billing\n")
	git(t, svc, "add", ".")
	git(t, svc, "commit", "-m", "port")
	// quiet: one commit ahead, changing nothing.
	quiet := fixtureRepo(t, filepath.Join(dir, "src", "quiet"))
	git(t, quiet, "commit", "--allow-empty", "-m", "nothing")
	// docs: detached, as a reference checkout is.
	docs := fixtureRepo(t, filepath.Join(dir, "src", "docs"))
	git(t, docs, "checkout", "--detach")

	stats := Repos(t.Context(), dir)
	if len(stats) != 3 {
		t.Fatalf("measured %d repositories, want 3", len(stats))
	}
	if stats[0] != (RepoStat{Name: "org/svc", Role: "editing", Commits: 1, Insertions: 1, Measured: true}) {
		t.Errorf("svc = %#v", stats[0])
	}
	if stats[1] != (RepoStat{Name: "org/quiet", Role: "editing", Commits: 1, Measured: true}) {
		t.Errorf("quiet = %#v", stats[1])
	}
	if stats[2] != (RepoStat{Name: "org/docs", Role: "reference"}) {
		t.Errorf("docs = %#v", stats[2])
	}
}

// An older manifest's blank default branch must never reach git as "origin/".
func TestReposReportsNothingWithoutABaseBranch(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "webhook", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{{Name: "svc", Org: "org", Role: session.RepoRoleActive,
			WorktreePath: "src/svc"}}})
	fixtureRepo(t, filepath.Join(dir, "src", "svc"))

	stats := Repos(t.Context(), dir)
	if len(stats) != 1 || stats[0].Measured {
		t.Fatalf("stats = %#v, want an unmeasured repository", stats)
	}
}

// fixtureRepo has a base commit reachable as origin/main, as a worktree does.
func fixtureRepo(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, path, "init", "-b", "main")
	git(t, path, "config", "user.email", "test@example.com")
	git(t, path, "config", "user.name", "test")
	writeFile(t, path, "README.md", "base\n")
	git(t, path, "add", ".")
	git(t, path, "commit", "-m", "base")
	git(t, path, "update-ref", "refs/remotes/origin/main", "HEAD")
	return path
}

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// A nil slice marshals as null, which the page's defaults do not defend against
// — it reaches a .length, throws, and unwinds the render before the fields
// registered after it ever run.
func TestEmptySlicesMarshalAsArraysNotNull(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "bare", Mode: session.ModeAssistant})

	fields, ok := Read(dir)
	if !ok {
		t.Fatal("Read reported no manifest")
	}
	fields.Repos = Repos(t.Context(), dir)

	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(b, &keyed); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"sessions", "documents", "repos"} {
		if string(keyed[key]) == "null" {
			t.Errorf("%q marshalled as null", key)
		}
	}

	// The failure paths are the ones that reach for a bare nil.
	if got := Repos(t.Context(), t.TempDir()); got == nil {
		t.Error("Repos returned nil for a directory with no manifest")
	}
}
