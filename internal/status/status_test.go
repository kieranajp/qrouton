package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/gittest"
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
		Mode:                "ASSISTANT",
		Phase:               "scratch",
		Identity:            "lifesum-4f3a",
		Slug:                "lifesum-4f3a",
		Sessions:            []SessionRow{},
		Documents:           []Document{},
		RepositoryDocuments: []RepositoryDocuments{},
		Repos:               []RepoStat{},
		Agents:              AgentPanel{Agents: []AgentRecord{}},
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
	want := Fields{
		Sessions: []SessionRow{}, Documents: []Document{}, RepositoryDocuments: []RepositoryDocuments{}, Repos: []RepoStat{},
		Agents: AgentPanel{Agents: []AgentRecord{}},
	}
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

func TestReadPreservesKnownProviderAndLeavesLegacyProviderUnknown(t *testing.T) {
	root := t.TempDir()
	known := sessionDir(t, root, session.Manifest{Slug: "known", Runner: "codex"})
	legacy := sessionDir(t, root, session.Manifest{Slug: "legacy"})
	if got := Read(known).Agents.Provider; got != "codex" {
		t.Fatalf("known provider = %q, want codex", got)
	}
	if got := Read(legacy).Agents.Provider; got != "" {
		t.Fatalf("legacy provider = %q, want unknown", got)
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

func TestDocumentsAreClassifiedByArtifactTaxonomyThenFilenamePrefix(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "webhook"})
	shared := filepath.Join(dir, "thoughts", "shared")
	kinds := []struct{ path, want string }{
		{"readme.md", "NOTE"},
		{"research/questions-about-p1.md", "RESEARCH"},
		{"research/parity-checklist.md", "RESEARCH"},
		{"research/R7-editor.md", "RESEARCH"},
		{"plans/next-steps.md", "PLAN"},
		{"plans/p002-orchestrated.md", "PLAN"},
		{"specs/S006-gui-workbench.md", "SPEC"},
		{"plans/P006-design-system.md", "PLAN"},
		{"R8-retiring-the-tui.md", "RESEARCH"},
		{"explainers/onboarding.md", "EXPLAINER"},
		{"explainers/E1-billing-flow.md", "EXPLAINER"},
		{"E2-webhook-retries.md", "EXPLAINER"},
		{"escalation-notes.md", "NOTE"},
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
		if got := DocumentKind(doc.path); got != doc.want {
			t.Errorf("DocumentKind(%q) = %q, want %q", doc.path, got, doc.want)
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

func TestReadListsThoughtsFromEachRepositoryInManifestOrder(t *testing.T) {
	root := t.TempDir()
	m := session.Manifest{Slug: "webhook", Repos: []session.ManifestRepo{
		{Name: "api", Org: "acme", WorktreePath: "src/api"},
		{Name: "web", Org: "acme", Role: session.RepoRoleReference, WorktreePath: "src/web"},
		{Name: "empty", Org: "acme", WorktreePath: "src/empty"},
		{Name: "missing-path", Org: "acme"},
	}}
	dir := sessionDir(t, root, m)
	files := map[string]string{
		"src/api/thoughts/plans/P2-api.md":        "# API plan\n",
		"src/api/thoughts/research/R1-api.md":     "# API research\n",
		"src/api/thoughts/research/ignored.txt":   "not a document\n",
		"src/web/thoughts/specs/S3-navigation.md": "# Navigation\n",
		"thoughts/shared/plans/P0-session.md":     "# Session plan\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got := Read(dir).RepositoryDocuments
	if len(got) != 2 || got[0].Name != "api" || got[1].Name != "web" {
		t.Fatalf("repository documents = %#v, want non-empty repositories in manifest order", got)
	}
	want := []struct{ name, path, kind string }{
		{"plans/P2-api.md", filepath.Join("src", "api", "thoughts", "plans", "P2-api.md"), KindPlan},
		{"research/R1-api.md", filepath.Join("src", "api", "thoughts", "research", "R1-api.md"), KindResearch},
	}
	if len(got[0].Documents) != len(want) {
		t.Fatalf("api documents = %#v, want %#v", got[0].Documents, want)
	}
	for i, document := range got[0].Documents {
		if document.Name != want[i].name || document.Path != want[i].path || document.Kind != want[i].kind {
			t.Errorf("api document %d = %#v, want name %q, path %q, kind %q", i, document, want[i].name, want[i].path, want[i].kind)
		}
	}
	if documents := got[1].Documents; len(documents) != 1 || documents[0].Name != "specs/S3-navigation.md" || documents[0].Kind != KindSpec {
		t.Fatalf("web documents = %#v", documents)
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
	if got := byslug["extract-billing"].Repos; len(got) != 1 || got[0].Name != "lifesum/api" {
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
			{Name: "svc", Org: "lifesum", Role: session.RepoRoleEditing},
			{Name: "contracts", Org: "lifesum", Role: session.RepoRoleReference},
			{Name: "api", Org: "lifesum", Role: session.RepoRoleEditing},
		}})

	rows := Sessions(root)
	if len(rows) != 1 {
		t.Fatalf("listed %d sessions, want 1: %#v", len(rows), rows)
	}
	want := []SessionRepo{
		{Name: "lifesum/svc", Role: "editing"},
		{Name: "lifesum/api", Role: "editing"},
	}
	if got := rows[0].Repos; !reflect.DeepEqual(got, want) {
		t.Fatalf("row repositories = %#v, want %#v", got, want)
	}
}

func TestSessionRowsKeepLegacyEditingRolesAndOmitReferences(t *testing.T) {
	root := t.TempDir()
	sessionDir(t, root, session.Manifest{Slug: "webhook", Name: "Webhook retry",
		Repos: []session.ManifestRepo{
			{Name: "legacy", Org: "lifesum"},
			{Name: "docs", Org: "lifesum", Role: session.RepoRoleReference},
		}})
	sessionDir(t, root, session.Manifest{Slug: "reference-only", Name: "Reference only",
		Repos: []session.ManifestRepo{{Name: "contracts", Org: "lifesum", Role: session.RepoRoleReference}}})

	rows := map[string]SessionRow{}
	for _, row := range Sessions(root) {
		rows[row.Slug] = row
	}
	if got := rows["webhook"].Repos; !reflect.DeepEqual(got, []SessionRepo{{Name: "lifesum/legacy", Role: "editing"}}) {
		t.Fatalf("legacy row repositories = %#v", got)
	}
	if got := rows["reference-only"].Repos; got == nil || len(got) != 0 {
		t.Fatalf("reference-only row repositories = %#v, want an empty list", got)
	}
}

func TestAgentContractCarriesProviderAndNeverMarshalsANullRecordList(t *testing.T) {
	fields := Read(t.TempDir())
	fields.Agents.Provider = "claude"
	b, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	var keyed map[string]json.RawMessage
	if err := json.Unmarshal(b, &keyed); err != nil {
		t.Fatal(err)
	}
	var panel map[string]json.RawMessage
	if err := json.Unmarshal(keyed["agents"], &panel); err != nil {
		t.Fatal(err)
	}
	if got := string(panel["provider"]); got != `"claude"` {
		t.Fatalf("provider marshalled as %s", got)
	}
	if got := string(panel["agents"]); got != "[]" {
		t.Fatalf("agent records marshalled as %s, want []", got)
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

func TestReposMeasuresEditingRepositoriesAndSkipsReferences(t *testing.T) {
	root := t.TempDir()
	m := session.Manifest{Slug: "webhook", Name: "webhook", Mode: session.ModeRPI,
		Repos: []session.ManifestRepo{
			{Name: "svc", Org: "org", Role: session.RepoRoleEditing, DefaultBranch: "main",
				Branch: "fix/webhook", WorktreePath: "src/svc"},
			{Name: "quiet", Org: "org", Role: session.RepoRoleEditing, DefaultBranch: "main",
				Branch: "fix/webhook", WorktreePath: "src/quiet"},
			{Name: "docs", Org: "org", Role: session.RepoRoleReference, WorktreePath: "src/docs"},
		}}
	dir := sessionDir(t, root, m)

	// svc: one commit ahead of origin/main, adding one line.
	svc := gittest.Worktree(t, filepath.Join(dir, "src", "svc"))
	gittest.WriteFile(t, svc, "invoice.go", "package billing")
	gittest.Run(t, svc, "add", ".")
	gittest.Run(t, svc, "commit", "-m", "port")
	// quiet: one commit ahead, changing nothing.
	quiet := gittest.Worktree(t, filepath.Join(dir, "src", "quiet"))
	gittest.Run(t, quiet, "commit", "--allow-empty", "-m", "nothing")
	// docs: detached, as a reference checkout is.
	docs := gittest.Worktree(t, filepath.Join(dir, "src", "docs"))
	gittest.Run(t, docs, "checkout", "--detach")

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
		Repos: []session.ManifestRepo{{Name: "svc", Org: "org", Role: session.RepoRoleEditing,
			WorktreePath: "src/svc"}}})
	gittest.Worktree(t, filepath.Join(dir, "src", "svc"))

	stats := Repos(t.Context(), dir)
	if len(stats) != 1 || stats[0].Measured {
		t.Fatalf("stats = %#v, want an unmeasured repository", stats)
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
	for _, key := range []string{"sessions", "documents", "repositoryDocuments", "repos"} {
		if string(keyed[key]) == "null" {
			t.Errorf("%q marshalled as null", key)
		}
	}
	var agents map[string]json.RawMessage
	if err := json.Unmarshal(keyed["agents"], &agents); err != nil {
		t.Fatal(err)
	}
	if string(agents["agents"]) == "null" {
		t.Error("agent records marshalled as null")
	}

	// The failure paths are the ones that reach for a bare nil.
	if got := Repos(t.Context(), t.TempDir()); got == nil {
		t.Error("Repos returned nil for a directory with no manifest")
	}
}

func TestArtifactIDIsTheNumberedPrefixTheFilenameStates(t *testing.T) {
	for path, want := range map[string]string{
		"thoughts/shared/plans/P002-2026-08-29-pane-smoke-test.md": "P002",
		"p12_notes.md":     "P12",
		"R001.md":          "R001",
		"S3-shape.md":      "S3",
		"E9-explainer.md":  "E9",
		"plans/notes.md":   "",
		"P-unnumbered.md":  "",
		"P002nodelim.md":   "",
		"draft-P002-do.md": "",
	} {
		if got := ArtifactID(path); got != want {
			t.Errorf("ArtifactID(%q) = %q, want %q", path, got, want)
		}
	}
}

// Every producer starts from EmptyFields, so a slice added to Fields and left
// out of it reaches the page as JSON null.
func TestEmptyFieldsLeavesNoSliceNil(t *testing.T) {
	noNilSlice(t, reflect.ValueOf(EmptyFields()), "EmptyFields()")
	noNilSlice(t, reflect.ValueOf(Read(t.TempDir())), "Read()")
}

func noNilSlice(t *testing.T, v reflect.Value, path string) {
	t.Helper()
	switch v.Kind() {
	case reflect.Slice, reflect.Map:
		if v.IsNil() {
			t.Errorf("%s is nil, and marshals as null", path)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			noNilSlice(t, v.Field(i), path+"."+v.Type().Field(i).Name)
		}
	}
}

// A kind filed under a directory has to answer to its filename prefix and carry
// an ID as well, or a new one lands half-wired — and the prefix fails silently.
func TestEveryArtifactKindAnswersToItsDirectoryAndItsFilename(t *testing.T) {
	if len(kindByDir) != len(artifactKinds) || len(kindByLetter) != len(artifactKinds) {
		t.Fatalf("%d kinds share %d directories and %d letters", len(artifactKinds), len(kindByDir), len(kindByLetter))
	}
	for _, a := range artifactKinds {
		dir := "thoughts/shared/" + a.dir + "/notes.md"
		if got := DocumentKind(dir); got != a.kind {
			t.Errorf("DocumentKind(%q) = %q, want %q", dir, got, a.kind)
		}
		for _, name := range []string{a.letter + "007-shape.md", strings.ToUpper(a.letter) + "007_shape.md"} {
			if got := DocumentKind(name); got != a.kind {
				t.Errorf("DocumentKind(%q) = %q, want %q", name, got, a.kind)
			}
			if got, want := ArtifactID(name), strings.ToUpper(a.letter)+"007"; got != want {
				t.Errorf("ArtifactID(%q) = %q, want %q", name, got, want)
			}
		}
	}
	if got := DocumentKind("research/P002-shape.md"); got != KindResearch {
		t.Errorf("the filename prefix beat the directory: got %q", got)
	}
	if got := DocumentKind("thoughts/shared/escalation-notes.md"); got != KindNote {
		t.Errorf("DocumentKind of an unclassified document = %q, want %q", got, KindNote)
	}
}
