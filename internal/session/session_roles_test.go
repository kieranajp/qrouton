package session

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func makeOrigin(t *testing.T, name string) (string, string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), name)
	run(t, "", "init", "-b", "main", origin)
	os.WriteFile(filepath.Join(origin, "version"), []byte("one"), 0o644)
	run(t, origin, "add", ".")
	run(t, origin, "commit", "-m", "initial")
	out, err := exec.Command("git", "-C", origin, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return origin, strings.TrimSpace(string(out))
}

func TestCreatePersistsSessionMode(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "svc")
	repos := []RepoSelection{{Repo: github.Repo{Name: "svc", Org: "org", SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleEditing}}
	dir, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Assistant sesh", Prefix: "feat", Mode: ModeAssistant, Repos: repos,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	b, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m.Mode != ModeAssistant || m.EffectiveMode() != ModeAssistant {
		t.Fatalf("assistant mode not persisted: %q", m.Mode)
	}
	if !strings.Contains(string(b), `"mode": "assistant"`) {
		t.Fatalf("manifest missing mode field:\n%s", b)
	}
	// An unset mode (legacy manifests) reads as the RPI default.
	if (Manifest{}).EffectiveMode() != ModeRPI {
		t.Fatal("empty mode should default to RPI")
	}
}

func TestCreateSessionWithEditingAndPinnedReference(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	editingOrigin, _ := makeOrigin(t, "editing")
	referenceOrigin, pinned := makeOrigin(t, "reference")
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, CreateRequest{Name: "Role test", Prefix: "feat", Mode: ModeRPI, Repos: []RepoSelection{
		{Repo: github.Repo{Name: "editing", Org: "org", SSHURL: editingOrigin, DefaultBranch: "main"}, Role: RepoRoleEditing},
		{Repo: github.Repo{Name: "reference", Org: "org", SSHURL: referenceOrigin, DefaultBranch: "main"}, Role: RepoRoleReference},
	}}, nil)
	if err != nil {
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
	if m.SchemaVersion != manifestSchemaVersion {
		t.Fatalf("schema version = %d", m.SchemaVersion)
	}
	if got := m.Repos[0]; got.Role != RepoRoleEditing || got.Branch != "feat/role-test" || got.Revision != "" {
		t.Fatalf("editing manifest repo = %+v", got)
	}
	if got := m.Repos[1]; got.Role != RepoRoleReference || got.Branch != "" || got.Revision != pinned {
		t.Fatalf("reference manifest repo = %+v", got)
	}
	if gitOK("-C", filepath.Join(dir, "src", "reference"), "symbolic-ref", "-q", "HEAD") {
		t.Fatal("reference checkout is not detached")
	}

	// Advance the remote, remove the worktree, and prove resume uses the recorded SHA.
	os.WriteFile(filepath.Join(referenceOrigin, "version"), []byte("two"), 0o644)
	run(t, referenceOrigin, "add", ".")
	run(t, referenceOrigin, "commit", "-m", "advance")
	os.RemoveAll(filepath.Join(dir, "src", "reference"))
	if err := EnsureWorktrees(cfg, m, nil); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("git", "-C", filepath.Join(dir, "src", "reference"), "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != pinned {
		t.Fatalf("resumed reference at %s, want pinned %s", got, pinned)
	}
}

// A repo with no explicit role resumes on its session branch: Create only ever
// writes "editing" or "reference", so an empty role means a hand-edited manifest
// and editing is the safe reading.
func TestManifestRepoWithoutRoleResumesOnItsBranch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	origin, _ := makeOrigin(t, "unroled")
	if err := ensureMirror(root, "org", "unroled", origin, nil); err != nil {
		t.Fatal(err)
	}
	m := Manifest{SchemaVersion: manifestSchemaVersion, Slug: "old", Repos: []ManifestRepo{{
		Name: "unroled", Org: "org", Branch: "feat/old", DefaultBranch: "main",
		WorktreePath: "src/unroled", SSHURL: origin,
	}}}
	if err := EnsureWorktrees(&config.Config{Root: root}, m, nil); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(root, "old", "src", "unroled")
	out, err := exec.Command("git", "-C", wt, "branch", "--show-current").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "feat/old" {
		t.Fatalf("checkout branch = %q", got)
	}
}

func writeManifestFile(t *testing.T, role string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "session")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"schemaVersion":2,"name":"Old","slug":"old","repos":[` +
		`{"name":"svc","org":"org","role":` + role + `,"worktreePath":"src/svc"}]}`
	if err := os.WriteFile(sessionpaths.Manifest(dir), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// "active" is the role literal qrouton wrote before the rename. It is not
// understood, and a session carrying it must say so rather than be read as
// editing by everything that only tests for reference.
func TestManifestWithARetiredRoleFailsToLoad(t *testing.T) {
	dir := writeManifestFile(t, `"active"`)
	_, err := Load(dir)
	if err == nil {
		t.Fatal("a manifest carrying the retired role loaded")
	}
	if !errors.Is(err, ErrInvalidRole) || !strings.Contains(err.Error(), "active") {
		t.Fatalf("error does not name the role: %v", err)
	}
	sessions, err := Scan(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("a session that cannot be loaded was listed as resumable: %+v", sessions)
	}
}

func TestManifestRoundTripsEditingAndDefaultsAnEmptyRole(t *testing.T) {
	m, err := Load(writeManifestFile(t, `"editing"`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Repos[0].Role != RepoRoleEditing {
		t.Fatalf("role = %q", m.Repos[0].Role)
	}
	m, err = Load(writeManifestFile(t, `""`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Repos[0].Role.Effective(); got != RepoRoleEditing {
		t.Fatalf("empty role reads as %q", got)
	}
}

// Create always records a clone URL. A manifest without one cannot be resumed,
// and guessing github.com for it would mirror the wrong repository in silence.
func TestEnsureWorktreesRejectsManifestWithoutCloneURL(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	m := Manifest{SchemaVersion: manifestSchemaVersion, Slug: "old", Repos: []ManifestRepo{{
		Name: "urlless", Org: "org", Branch: "feat/old", DefaultBranch: "main", WorktreePath: "src/urlless",
	}}}
	err := EnsureWorktrees(&config.Config{Root: root}, m, nil)
	if err == nil {
		t.Fatal("manifest without a clone URL was resumed")
	}
	if !strings.Contains(err.Error(), "urlless") {
		t.Fatalf("error does not name the repository: %v", err)
	}
}

func TestCreateFailureCleansNewSessionDirectoryAndAllowsRetry(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "retry")
	repo := github.Repo{Name: "retry", Org: "org", SSHURL: origin, DefaultBranch: "main"}
	bad := []RepoSelection{{Repo: repo, Role: RepoRole("invalid")}}
	if _, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Retry me", Prefix: "feat", Mode: ModeRPI, Repos: bad,
	}, nil); err == nil {
		t.Fatal("invalid role unexpectedly succeeded")
	}
	dir := filepath.Join(root, "retry-me")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("failed session directory was not cleaned up: %v", err)
	}
	if _, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Retry me", Prefix: "feat", Mode: ModeRPI,
		Repos: []RepoSelection{{Repo: repo, Role: RepoRoleEditing}},
	}, nil); err != nil {
		t.Fatal("retry failed:", err)
	}
}

func TestCreateReclaimsAbandonedAssemblyDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "reclaim")

	// Simulate a run killed mid-assembly: marker written, manifest never reached.
	dir := filepath.Join(root, "reclaim-me")
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, assemblingMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	repo := github.Repo{Name: "reclaim", Org: "org", SSHURL: origin, DefaultBranch: "main"}
	created, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Reclaim me", Prefix: "feat", Mode: ModeRPI,
		Repos: []RepoSelection{{Repo: repo, Role: RepoRoleEditing}},
	}, nil)
	if err != nil {
		t.Fatal("abandoned directory blocked its session name:", err)
	}
	if _, err := os.Stat(filepath.Join(created, manifestName)); err != nil {
		t.Fatal("reclaimed session has no manifest:", err)
	}
	if _, err := os.Stat(filepath.Join(created, assemblingMarker)); !os.IsNotExist(err) {
		t.Fatal("assembly marker survived a completed session")
	}
}

func TestCreateRefusesToReclaimUnmarkedDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "keep")

	// A user directory that merely shares the slug must never be deleted.
	dir := filepath.Join(root, "keep-me")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	repo := github.Repo{Name: "keep", Org: "org", SSHURL: origin, DefaultBranch: "main"}
	if _, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Keep me", Prefix: "feat", Mode: ModeRPI,
		Repos: []RepoSelection{{Repo: repo, Role: RepoRoleEditing}},
	}, nil); err == nil {
		t.Fatal("unmarked directory was silently taken over")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatal("user file lost:", err)
	}
}

func TestEnsureWorktreesReclonesMissingMirrorFromRecordedURL(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "custom")
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, CreateRequest{
		Name: "Custom origin", Prefix: "feat", Mode: ModeRPI,
		Repos: []RepoSelection{{Repo: github.Repo{Name: "custom", Org: "org", SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleEditing}},
	}, nil)
	if err != nil {
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
	if m.Repos[0].SSHURL != origin {
		t.Fatalf("manifest sshUrl = %q, want %q", m.Repos[0].SSHURL, origin)
	}

	// Lose both the mirror and the worktree; resume must re-clone from the
	// recorded URL, not a reconstructed github.com address.
	if err := os.RemoveAll(mirrorPath(root, "org", "custom")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "src", "custom")); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWorktrees(cfg, m, nil); err != nil {
		t.Fatal("resume with recorded URL failed:", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "custom", "version")); err != nil {
		t.Fatal("worktree not re-materialised:", err)
	}
}

// Assembling into a session that already exists is what escalation does: the
// picker composes repositories into the loaded manifest and folds them into one
// atomic write alongside the mode and the escalation stanza.
func TestComposeReposAssemblesIntoZeroRepoSession(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, CreateRequest{Name: "Scratch pad", Mode: ModeAssistant}, nil)
	if err != nil {
		t.Fatal(err)
	}
	editingOrigin, _ := makeOrigin(t, "editing")
	referenceOrigin, pinned := makeOrigin(t, "reference")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ComposeRepos(cfg, m, []RepoSelection{
		{Repo: github.Repo{Name: "editing", Org: "org", SSHURL: editingOrigin, DefaultBranch: "main"}, Role: RepoRoleEditing},
		{Repo: github.Repo{Name: "reference", Org: "org", SSHURL: referenceOrigin, DefaultBranch: "main"}, Role: RepoRoleReference},
	}, "fix/webhook-retry", nil)
	if err != nil {
		t.Fatal(err)
	}
	out.Mode = ModeRPI
	if err := WriteManifest(dir, out); err != nil {
		t.Fatal(err)
	}
	if !gitOK("-C", mirrorPath(root, "org", "editing"), "show-ref", "--verify", "--quiet", "refs/heads/fix/webhook-retry") {
		t.Fatal("session branch missing from the mirror after assembly")
	}
	if gitOK("-C", filepath.Join(dir, "src", "reference"), "symbolic-ref", "-q", "HEAD") {
		t.Fatal("reference checkout is not detached")
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 2 {
		t.Fatalf("manifest repos after assembly = %+v", got.Repos)
	}
	if r := got.Repos[0]; r.Role != RepoRoleEditing || r.Branch != "fix/webhook-retry" {
		t.Fatalf("editing manifest repo = %+v", r)
	}
	if r := got.Repos[1]; r.Role != RepoRoleReference || r.Revision != pinned {
		t.Fatalf("reference manifest repo = %+v", r)
	}
}

func TestSetModePersistsAcrossReread(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Modal", Mode: ModeAssistant,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetMode(dir, ModeRPI); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeRPI {
		t.Fatalf("mode after SetMode = %q, want rpi", got.Mode)
	}
}

// Escalating out of assistant mode owes the next runner a fresh conversation,
// and that debt is recorded on disk so a restart cannot lose it. Every other
// mode write must leave no marker, or a later launch would discard a
// conversation it was meant to keep.
func TestWriteManifestMarksOnlyTheEscalationHandoff(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Modal", Mode: ModeAssistant,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	marker := sessionpaths.HandoffPending(dir)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("a freshly created session already owes a handoff")
	}

	for _, tc := range []struct {
		name string
		to   SessionMode
		want bool
	}{
		{"assistant to rpi is a handoff", ModeRPI, true},
		{"rpi to rpi only adds repositories", ModeRPI, false},
		{"rpi to assistant is de-escalation", ModeAssistant, false},
		{"assistant to assistant is a no-op", ModeAssistant, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			os.Remove(marker)
			if err := SetMode(dir, tc.to); err != nil {
				t.Fatal(err)
			}
			_, err := os.Stat(marker)
			if got := err == nil; got != tc.want {
				t.Fatalf("marker present = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSessionProgressReportsAssemblyOperations(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "progress")
	var events []Progress
	_, err := Create(&config.Config{Root: root}, CreateRequest{
		Name: "Progress", Prefix: "feat", Mode: ModeRPI,
		Repos: []RepoSelection{{Repo: github.Repo{Name: "progress", Org: "org", SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleEditing}},
	}, func(event Progress) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		step   ProgressStep
		status ProgressStatus
	}{
		{ProgressMirror, ProgressStarted}, {ProgressMirror, ProgressCompleted},
		{ProgressWorktree, ProgressStarted}, {ProgressWorktree, ProgressCompleted},
		{ProgressScaffold, ProgressStarted}, {ProgressScaffold, ProgressCompleted},
		{ProgressManifest, ProgressStarted}, {ProgressManifest, ProgressCompleted},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i := range want {
		if events[i].Step != want[i].step || events[i].Status != want[i].status {
			t.Fatalf("event %d = %+v, want step=%s status=%s", i, events[i], want[i].step, want[i].status)
		}
	}
	if events[0].Repo == nil || events[0].Repo.Name != "progress" || events[0].Role != RepoRoleEditing {
		t.Fatalf("repo progress lacks context: %+v", events[0])
	}
}

// The guard lives in ComposeRepos rather than in the picker, so every route into
// it is covered — the collision handling below it counts names, and cannot tell a
// repository from itself.
func TestComposeReposSkipsRepositoriesAlreadyInTheSession(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: root}
	origin, _ := makeOrigin(t, "repo123")
	repo := github.Repo{Name: "repo123", Org: "kieranajp", SSHURL: origin, DefaultBranch: "main"}
	dir, err := Create(cfg, CreateRequest{
		Name: "repo123", Prefix: "feat", Mode: ModeAssistant,
		Repos: []RepoSelection{{Repo: repo, Role: RepoRoleEditing}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Owner and name identify a repository, whatever case they arrive in.
	shouty := github.Repo{Name: "REPO123", Org: "KieranAJP", SSHURL: origin, DefaultBranch: "main"}
	out, err := ComposeRepos(cfg, m, []RepoSelection{{Repo: shouty, Role: RepoRoleEditing}}, "fix/other", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Repos) != 1 {
		t.Fatalf("repository re-materialised: %+v", out.Repos)
	}
	if r := out.Repos[0]; r.Branch != "feat/repo123" {
		t.Fatalf("existing branch changed to %q", r.Branch)
	}

	// A different owner sharing the name is still a different repository, and
	// still gets an org-qualified path.
	otherOrigin, _ := makeOrigin(t, "other-repo123")
	other := github.Repo{Name: "repo123", Org: "lifesum", SSHURL: otherOrigin, DefaultBranch: "main"}
	out, err = ComposeRepos(cfg, out, []RepoSelection{{Repo: other, Role: RepoRoleEditing}}, "fix/other", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Repos) != 2 {
		t.Fatalf("a distinct owner's repo was skipped: %+v", out.Repos)
	}
	if got := out.Repos[1].WorktreePath; got != filepath.Join("src", "lifesum-repo123") {
		t.Fatalf("second owner's worktree = %q", got)
	}
}
