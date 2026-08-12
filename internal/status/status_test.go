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
	got := Read(dir)
	want := Fields{
		Mode:      "ASSISTANT",
		Phase:     "scratch",
		Identity:  "lifesum-4f3a",
		Slug:      "lifesum-4f3a",
		Sessions:  []SessionRow{},
		Documents: []Document{},
		Repos:     []RepoStat{},
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
	got := Read(dir)
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
	if got := Read(dir); got.Phase != "Plan" {
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
	if got := Read(dir); got.Phase != "Implement" {
		t.Fatalf("phase did not advance to Implement: %q", got.Phase)
	}
}

// The landing path, before onboarding has chosen a session: the window still has
// to be told something, and no slice in it may be nil.
func TestReadWithoutManifest(t *testing.T) {
	want := Fields{Sessions: []SessionRow{}, Documents: []Document{}, Repos: []RepoStat{}}
	if got := Read(t.TempDir()); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing manifest = %#v, want %#v", got, want)
	}
}

// The slug is how the page addresses a session: its rail row, its window
// surfaces and its documents are all keyed on it.
func TestFieldsCarryTheSessionsSlug(t *testing.T) {
	dir := sessionDir(t, t.TempDir(), session.Manifest{Slug: "extract-billing", Name: "Extract billing"})
	if got := Read(dir).Slug; got != "extract-billing" {
		t.Fatalf("slug = %q, want the manifest's", got)
	}
	if got := Read(t.TempDir()).Slug; got != "" {
		t.Fatalf("a directory with no manifest answered with slug %q", got)
	}
}

// The page reads both off the chrome payload: without them it can name no
// session and attach to no conversation.
func TestTheSlugAndTerminalReachTheWire(t *testing.T) {
	b, err := json.Marshal(Fields{Slug: "extract-billing", Terminal: "term-1"})
	if err != nil {
		t.Fatal(err)
	}
	var keyed map[string]any
	if err := json.Unmarshal(b, &keyed); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"slug": "extract-billing", "terminal": "term-1"} {
		if keyed[key] != want {
			t.Fatalf("%q marshalled as %v, want %q", key, keyed[key], want)
		}
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

	fields := Read(dir)
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

// The rail is drawn before any session is on screen, so this answers from the
// root alone and leaves the workbench's own knowledge to the caller that has it.
func TestSessionsListsEverySessionUnderTheRoot(t *testing.T) {
	root := t.TempDir()
	sessionDir(t, root, session.Manifest{Slug: "flaky-suite", Name: "Flaky suite", Mode: session.ModeAssistant})
	sessionDir(t, root, session.Manifest{Slug: "extract-billing", Name: "Extract billing", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{{Name: "api", Org: "lifesum"}}})

	rows := Sessions(root)
	if len(rows) != 2 {
		t.Fatalf("listed %d sessions, want 2: %#v", len(rows), rows)
	}
	byslug := map[string]SessionRow{}
	for _, row := range rows {
		byslug[row.Slug] = row
		if row.Terminal != "" || row.Activity != "" {
			t.Fatalf("row %#v claims a conversation; a file scan cannot know that", row)
		}
	}
	if got := byslug["extract-billing"]; got.Name != "Extract billing" || got.Initials != "eb" || got.Mode != "RPI" {
		t.Fatalf("billing row = %#v", got)
	}
	if got := byslug["extract-billing"].Repos; len(got) != 1 || got[0].Name != "api" {
		t.Fatalf("billing row repositories = %#v, want the one its manifest names", got)
	}
	if got := byslug["flaky-suite"]; got.Name != "Flaky suite" || got.Initials != "fs" || got.Mode != "ASSISTANT" {
		t.Fatalf("flaky row = %#v", got)
	}
}

// The rail has room for three repository names and counts the rest, so manifest
// order is a ranking: reorder it and truncation keeps an arbitrary three.
func TestSessionRowsCarryRepositoriesInManifestOrder(t *testing.T) {
	root := t.TempDir()
	sessionDir(t, root, session.Manifest{Slug: "webhook", Name: "Webhook retry",
		Repos: []session.ManifestRepo{
			{Name: "svc", Org: "lifesum", Role: session.RepoRoleActive},
			{Name: "contracts", Org: "lifesum", Role: session.RepoRoleReference},
			{Name: "api", Org: "lifesum", Role: session.RepoRoleActive},
		}})

	rows := Sessions(root)
	if len(rows) != 1 {
		t.Fatalf("listed %d sessions, want 1: %#v", len(rows), rows)
	}
	want := []SessionRepo{
		{Name: "svc", Role: "editing"},
		{Name: "contracts", Role: "reference"},
		{Name: "api", Role: "editing"},
	}
	if got := rows[0].Repos; !reflect.DeepEqual(got, want) {
		t.Fatalf("row repositories = %#v, want %#v", got, want)
	}
}

// A nil slice marshals as null, and the page's defaults only fill keys the
// payload omits, so null reaches a .length and unwinds the render.
func TestSessionRowsWithoutRepositoriesCarryAnEmptyList(t *testing.T) {
	root := t.TempDir()
	sessionDir(t, root, session.Manifest{Slug: "bare", Name: "Bare"})

	rows := Sessions(root)
	if len(rows) != 1 {
		t.Fatalf("listed %d sessions, want 1: %#v", len(rows), rows)
	}
	if rows[0].Repos == nil {
		t.Fatal("a session with no repositories carries a nil repository list")
	}
	b, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(b, &keyed); err != nil {
		t.Fatal(err)
	}
	if got := string(keyed["repos"]); got != "[]" {
		t.Fatalf("repos marshalled as %s, want []", got)
	}
}

func TestSessionRowsCarryWhenTheyWereLastShown(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "extract-billing", Name: "Extract billing"})
	sessionDir(t, root, session.Manifest{Slug: "flaky-suite", Name: "Flaky suite"})
	at := time.Now().Add(-time.Hour).UTC()
	if err := session.MarkOpened(dir, at); err != nil {
		t.Fatal(err)
	}

	for _, row := range Sessions(root) {
		switch row.Slug {
		case "extract-billing":
			if !row.Opened.Equal(at) {
				t.Fatalf("stamped row = %#v, want Opened %v", row, at)
			}
		case "flaky-suite":
			if !row.Opened.IsZero() {
				t.Fatalf("row %#v carries a stamp for a session never shown", row)
			}
		}
	}
}

// sessionDoc writes one document under a session's thoughts tree at an explicit
// mtime; filesystem timestamps are too coarse to order writes by wall clock.
func sessionDoc(t *testing.T, dir, name string, at time.Time) {
	t.Helper()
	path := filepath.Join(dir, "thoughts", "shared", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestUnseenCountsTheDocumentsWrittenSinceTheStamp(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "webhook"})
	stamp := time.Now().Add(-time.Hour)
	if err := session.MarkOpened(dir, stamp); err != nil {
		t.Fatal(err)
	}
	sessionDoc(t, dir, "research/R1-findings.md", stamp.Add(-time.Minute))
	sessionDoc(t, dir, "plans/P1-rollout.md", stamp.Add(time.Minute))
	sessionDoc(t, dir, "specs/S1-shape.md", stamp.Add(2*time.Minute))
	sessionDoc(t, dir, "scratch.txt", stamp.Add(3*time.Minute))

	if got := Unseen(root)["webhook"]; got != 2 {
		t.Fatalf("unseen = %d, want the 2 documents newer than the stamp", got)
	}
}

func TestUnseenIsZeroWithNothingNewToReport(t *testing.T) {
	root := t.TempDir()
	stamp := time.Now().Add(-time.Hour)
	empty := sessionDir(t, root, session.Manifest{Slug: "empty"})
	stale := sessionDir(t, root, session.Manifest{Slug: "stale"})
	for _, dir := range []string{empty, stale} {
		if err := session.MarkOpened(dir, stamp); err != nil {
			t.Fatal(err)
		}
	}
	sessionDoc(t, stale, "research/R1-findings.md", stamp.Add(-time.Minute))

	unseen := Unseen(root)
	for _, slug := range []string{"empty", "stale"} {
		got, measured := unseen[slug]
		if !measured || got != 0 {
			t.Fatalf("%q reports %d unseen documents, measured %v; want a measured zero", slug, got, measured)
		}
	}
}

// A session this workbench has never shown has nothing to measure against, so
// every document it ever wrote would count.
func TestUnseenSkipsASessionWithNoStamp(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "never-shown"})
	sessionDoc(t, dir, "research/R1-findings.md", time.Now())
	sessionDoc(t, dir, "plans/P1-rollout.md", time.Now())

	got, measured := Unseen(root)["never-shown"]
	if measured || got != 0 {
		t.Fatalf("a session never shown reports %d unseen documents, measured %v", got, measured)
	}
}

// Opening the app comes back to the session you were last in, and the rail is
// ordered the same way.
func TestSessionsPutTheMostRecentlyShownFirst(t *testing.T) {
	root := t.TempDir()
	shown := []struct {
		slug string
		ago  time.Duration
	}{
		{slug: "older", ago: 2 * time.Hour},
		{slug: "newer", ago: time.Minute},
	}
	for _, s := range shown {
		dir := sessionDir(t, root, session.Manifest{Slug: s.slug, Name: s.slug})
		if err := session.MarkOpened(dir, time.Now().Add(-s.ago)); err != nil {
			t.Fatal(err)
		}
	}
	// Never shown, and named so that name order alone would put them first.
	sessionDir(t, root, session.Manifest{Slug: "alpha", Name: "alpha"})
	sessionDir(t, root, session.Manifest{Slug: "beta", Name: "beta"})

	var got []string
	for _, row := range Sessions(root) {
		got = append(got, row.Slug)
	}
	want := []string{"newer", "older", "alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rail order = %v, want %v", got, want)
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

	fields := Read(dir)
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
