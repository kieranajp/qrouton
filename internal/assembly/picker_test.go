package assembly

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/gittest"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func testRepo(t *testing.T, name string) github.Repo {
	t.Helper()
	return github.Repo{Name: name, Org: "org", SSHURL: gittest.Origin(t, name), DefaultBranch: "main"}
}

func editing(repos ...github.Repo) []session.RepoSelection {
	sels := make([]session.RepoSelection, 0, len(repos))
	for _, r := range repos {
		sels = append(sels, session.RepoSelection{Repo: r, Role: session.RepoRoleEditing})
	}
	return sels
}

// scratch is a live assistant session with no repositories, which is what the
// escalate tool and the add-repos button both open the picker over.
func scratch(t *testing.T) (Assembler, string) {
	t.Helper()
	cfg := &config.Config{Orgs: []string{"org"}, Root: t.TempDir()}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "scratch", Mode: session.ModeAssistant,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return Assembler{Cfg: cfg}, dir
}

func TestConfirmWritesReposModeAndStanzaTogether(t *testing.T) {
	a, dir := scratch(t)
	draft := Draft{Name: "Webhook retry backoff", Prefix: "fix", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, Answer{Escalating: true, Awaited: true}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != session.ModeRPI || got.Name != "Webhook retry backoff" {
		t.Fatalf("escalated manifest = mode %q name %q", got.Mode, got.Name)
	}
	if len(got.Repos) != 1 || got.Repos[0].Branch != "fix/webhook-retry-backoff" {
		t.Fatalf("escalated repos = %+v", got.Repos)
	}
	if got.Picker == nil || got.Picker.Status != session.PickerConfirmed || got.Picker.At.IsZero() {
		t.Fatalf("confirmed stanza = %+v", got.Picker)
	}
}

// The workbench's button adds repositories to the session as it stands. Moving
// it to RPI would also hand the next launch a fresh conversation, so an
// assistant session would lose its chat to a button that promised repositories.
func TestAddingReposLeavesTheModeAndConversationAlone(t *testing.T) {
	a, dir := scratch(t)
	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, Answer{}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.EffectiveMode() != session.ModeAssistant {
		t.Fatalf("mode = %q, want assistant", got.Mode)
	}
	if got.Picker != nil {
		t.Fatalf("adding repositories recorded an escalation: %+v", got.Picker)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos = %+v", got.Repos)
	}
	if _, err := os.Stat(sessionpaths.HandoffPending(dir)); !os.IsNotExist(err) {
		t.Fatal("adding repositories owed the next launch a fresh conversation")
	}
}

// The final write must merge into what is on disk after a long clone, rather
// than restoring the manifest from before it began.
func TestAddingReposPreservesManifestChangesMadeDuringAssembly(t *testing.T) {
	a, dir := scratch(t)
	wrote := false
	progress := func(p session.Progress) {
		if wrote || p.Step != session.ProgressWorktree || p.Status != session.ProgressCompleted {
			return
		}
		wrote = true
		if err := session.SetMode(dir, session.ModeRPI); err != nil {
			t.Error(err)
		}
		// Runner is a field Confirm never sets, so it witnesses the merge
		// independently of the mode.
		if err := setRunner(dir, "codex"); err != nil {
			t.Error(err)
		}
	}
	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, Answer{}, progress); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("test did not update the manifest during assembly")
	}
	if got.EffectiveMode() != session.ModeRPI {
		t.Fatalf("mode = %q, want the mode written during assembly", got.Mode)
	}
	if got.Runner != "codex" {
		t.Fatalf("runner written during assembly was lost: %q", got.Runner)
	}
	if len(got.Repos) != 1 || got.Repos[0].Name != "svc" {
		t.Fatalf("assembled repository was lost: %+v", got.Repos)
	}
}

// The picker can sit open for the escalate tool's full ~30 minutes while
// something else rewrites the manifest underneath it, so Confirm merges into the
// manifest as it stands rather than writing back the snapshot it opened on.
func TestConfirmPreservesManifestChangesMadeAfterPickerOpened(t *testing.T) {
	a, dir := scratch(t)
	if err := setRunner(dir, "codex"); err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Webhook retry backoff", Prefix: "fix", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, Answer{Escalating: true, Awaited: true}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Runner != "codex" {
		t.Fatalf("confirm discarded the runner written after the picker opened: %q", got.Runner)
	}
}

// setRunner writes a field Confirm never touches, so a test can prove the final
// write merged rather than restored.
func setRunner(dir, runner string) error {
	m, err := session.Load(dir)
	if err != nil {
		return err
	}
	m.Runner = runner
	return session.WriteManifest(dir, m)
}

// A repository added on a second trip through the picker joins the session's own
// branch, not one derived from the form's prefix — which defaults to feat
// whatever prefix the session was cut with.
func TestAddedReposJoinTheSessionBranch(t *testing.T) {
	cfg := &config.Config{Orgs: []string{"org"}, Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "Webhook retry", Prefix: "fix", Mode: session.ModeRPI, Repos: editing(testRepo(t, "svc")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Webhook retry", Prefix: "feat", Repos: editing(testRepo(t, "contracts"))}
	if err := a.Confirm(dir, draft, Answer{}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 2 {
		t.Fatalf("repos after adding one = %+v", got.Repos)
	}
	for _, r := range got.Repos {
		if r.Branch != "fix/webhook-retry" {
			t.Fatalf("%s went on %q, want the session's own branch", r.Name, r.Branch)
		}
	}
}

// Escalating a session that has already been worked in must not disturb the
// checkout the work is in. Selecting the repo you are working in is the obvious
// move in the picker, so a repository the session already holds is left where it
// stands rather than cloned again onto a second branch.
func TestEscalationLeavesAnAlreadyPresentRepoAlone(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	repo := github.Repo{Name: "repo123", Org: "kieranajp", SSHURL: gittest.Origin(t, "repo123"), DefaultBranch: "main"}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "repo123", Prefix: "feat", Mode: session.ModeAssistant, Repos: editing(repo),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Uncommitted work in the original checkout.
	stub := filepath.Join(dir, "src", "repo123", "stub.go")
	if err := os.WriteFile(stub, []byte("package stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	draft := Draft{Name: "Webhook retry backoff", Prefix: "fix", Repos: editing(repo)}
	if err := a.Confirm(dir, draft, Answer{Escalating: true, Awaited: true}, nil); err != nil {
		t.Fatal(err)
	}

	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repository duplicated on escalation: %+v", got.Repos)
	}
	if r := got.Repos[0]; r.Branch != "feat/repo123" || r.WorktreePath != "src/repo123" {
		t.Fatalf("existing checkout was moved: branch=%q worktree=%q", r.Branch, r.WorktreePath)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "src")); err != nil || len(entries) != 1 {
		t.Fatalf("src/ holds %d checkouts, want 1 (err %v)", len(entries), err)
	}
	if _, err := os.Stat(stub); err != nil {
		t.Fatal("uncommitted work lost from the original checkout:", err)
	}
	if got.EffectiveMode() != session.ModeRPI {
		t.Fatalf("mode = %q, want rpi", got.Mode)
	}
}

func TestRepositoryChangesSignalTheSupervisorWithAQueuedNotice(t *testing.T) {
	a, dir := scratch(t)
	var signalled []string
	a.Signal = func(root string) { signalled = append(signalled, root) }

	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, Answer{}, nil); err != nil {
		t.Fatal(err)
	}
	if len(signalled) != 1 || signalled[0] != dir {
		t.Fatalf("adding repositories signalled %v", signalled)
	}
	notice, err := os.ReadFile(sessionpaths.AgentNotice(dir))
	if err != nil {
		t.Fatal(err)
	}
	want := "qrouton: session repositories changed — added org/svc for editing at src/svc. " +
		"Re-read qrouton.json before continuing with the updated workspace."
	if string(notice) != want {
		t.Fatalf("queued notice = %q, want %q", notice, want)
	}
	if err := os.Remove(sessionpaths.AgentNotice(dir)); err != nil {
		t.Fatal(err)
	}
	if err := a.Confirm(dir, draft, Answer{Escalating: true, Awaited: true}, nil); err != nil {
		t.Fatal(err)
	}
	if len(signalled) != 2 || signalled[1] != dir {
		t.Fatalf("escalation signalled %v", signalled)
	}
}

func TestRepositoryNoticeNamesReferenceAdditionsAndPromotions(t *testing.T) {
	before := session.Manifest{Repos: []session.ManifestRepo{
		{Org: "org", Name: "docs", Role: session.RepoRoleReference, WorktreePath: "src/docs"},
		{Org: "org", Name: "svc", Role: session.RepoRoleEditing, WorktreePath: "src/svc"},
	}}
	after := session.Manifest{Repos: []session.ManifestRepo{
		{Org: "org", Name: "docs", Role: session.RepoRoleEditing, WorktreePath: "src/docs"},
		{Org: "org", Name: "svc", Role: session.RepoRoleEditing, WorktreePath: "src/svc"},
		{Org: "org", Name: "contracts", Role: session.RepoRoleReference, WorktreePath: "src/contracts"},
	}}
	want := "qrouton: session repositories changed — promoted org/docs to editing at src/docs; " +
		"added org/contracts as a read-only reference at src/contracts. " +
		"Re-read qrouton.json before continuing with the updated workspace."
	if got := repositoryNotice(before, after); got != want {
		t.Fatalf("notice = %q, want %q", got, want)
	}
	if got := repositoryNotice(after, after); got != "" {
		t.Fatalf("unchanged repositories produced notice %q", got)
	}
}

// A repository request is awaited without escalating: the tool blocked on it
// reads its answer from the stanza, and the session stays in the mode it was in.
// It is also the one confirm that must not signal — the supervisor would
// relaunch the runner, killing the tool call waiting for this answer.
func TestConfirmingAnAwaitedPickerRecordsItWithoutMovingTheMode(t *testing.T) {
	a, dir := scratch(t)
	signalled := 0
	a.Signal = func(string) { signalled++ }
	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, Answer{Awaited: true}, nil); err != nil {
		t.Fatal(err)
	}
	if signalled != 0 {
		t.Fatalf("an awaited confirm signalled the supervisor %d times", signalled)
	}
	if _, err := os.Stat(sessionpaths.AgentNotice(dir)); err == nil {
		t.Fatal("an awaited confirm queued a notice; its caller already has the set")
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.EffectiveMode() != session.ModeAssistant {
		t.Fatalf("mode = %q, want assistant", got.Mode)
	}
	if got.Picker == nil || got.Picker.Status != session.PickerConfirmed || got.Picker.At.IsZero() {
		t.Fatalf("confirmed stanza = %+v", got.Picker)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos after an awaited confirm = %+v", got.Repos)
	}
}

// The add-repos button has nobody polling for an answer, so it leaves no stanza
// for the next request's poll to read as its own.
func TestConfirmingAPickerNobodyAwaitsWritesNoStanza(t *testing.T) {
	a, dir := scratch(t)
	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, Answer{}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Picker != nil {
		t.Fatalf("an unawaited confirm wrote %+v", got.Picker)
	}
}

func TestCancelWritesTheCancelledStanzaOnlyWhenSomethingAwaitsIt(t *testing.T) {
	dir := t.TempDir()
	if err := session.WriteManifest(dir, session.Manifest{Slug: "scratch", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	if err := Cancel(dir, Answer{}); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Picker != nil {
		t.Fatalf("a plain add-repos cancel wrote a stanza: %+v", got.Picker)
	}

	if err := Cancel(dir, Answer{Awaited: true}); err != nil {
		t.Fatal(err)
	}
	got, err = session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Picker == nil || got.Picker.Status != session.PickerCancelled || got.Picker.At.IsZero() {
		t.Fatalf("cancelled stanza = %+v", got.Picker)
	}
	if got.EffectiveMode() != session.ModeAssistant || len(got.Repos) != 0 {
		t.Fatalf("cancel touched the session beyond the stanza: %+v", got)
	}
}

// Same staleness risk as confirm: cancel must not overwrite what was written
// while the picker was up with the picker's own load-time copy.
func TestCancelPreservesManifestChangesMadeAfterPickerOpened(t *testing.T) {
	dir := t.TempDir()
	if err := session.WriteManifest(dir, session.Manifest{Slug: "scratch", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	if err := setRunner(dir, "codex"); err != nil {
		t.Fatal(err)
	}
	if err := Cancel(dir, Answer{Escalating: true, Awaited: true}); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Runner != "codex" {
		t.Fatalf("cancel discarded the runner written after the picker opened: %q", got.Runner)
	}
}

func commit(t *testing.T, dir, message string) {
	t.Helper()
	args := []string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false",
		"commit", "--allow-empty", "-m", message}
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func referenced(repos ...github.Repo) []session.RepoSelection {
	sels := make([]session.RepoSelection, 0, len(repos))
	for _, r := range repos {
		sels = append(sels, session.RepoSelection{Repo: r, Role: session.RepoRoleReference})
	}
	return sels
}

// Taking up a repository the session reads is how a session assembled for
// reading alone starts being worked in: the branch is derived here, because a
// reference-only session has none of its own yet.
func TestConfirmUpgradesAHeldReferenceRepoOntoTheSessionBranch(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "Read only", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: referenced(testRepo(t, "docs")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Read only", Prefix: "chore",
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, Answer{}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos after an upgrade = %+v", got.Repos)
	}
	if r := got.Repos[0]; r.Role != session.RepoRoleEditing || r.Branch != "chore/read-only" || r.Revision != "" {
		t.Fatalf("upgraded repo = %+v", r)
	}
	if got.Branch() != "chore/read-only" {
		t.Fatalf("session branch after an upgrade = %q", got.Branch())
	}
}

// An upgrade and an addition land on the same branch, in the same write.
func TestConfirmUpgradesAndAddsInOneWrite(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "Both", Prefix: "feat", Mode: session.ModeAssistant, Repos: referenced(testRepo(t, "docs")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Both", Prefix: "fix", Repos: editing(testRepo(t, "svc")),
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, Answer{}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 2 {
		t.Fatalf("repos = %+v", got.Repos)
	}
	for _, r := range got.Repos {
		if r.Role != session.RepoRoleEditing || r.Branch != "fix/both" {
			t.Fatalf("%s = role %q branch %q", r.Name, r.Role, r.Branch)
		}
	}
}

// A refused take-up leaves the file describing what is on disk, and — because it
// is attempted before anything is cloned — leaves the session holding no checkout
// the manifest never learned about.
func TestConfirmClonesNothingWhenAnUpgradeIsRefused(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "Refused", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: referenced(testRepo(t, "docs")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, filepath.Join(dir, "src", "docs"), "local work")

	draft := Draft{Name: "Refused", Prefix: "feat", Repos: editing(testRepo(t, "svc")),
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, Answer{Escalating: true, Awaited: true}, nil); err == nil {
		t.Fatal("a take-up of a checkout carrying commits was confirmed")
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos after a refusal = %+v", got.Repos)
	}
	if r := got.Repos[0]; r.Role != session.RepoRoleReference || r.Branch != "" || r.Revision == "" {
		t.Fatalf("a refused take-up rewrote the entry: %+v", r)
	}
	if got.Picker != nil {
		t.Fatalf("a refused take-up recorded an escalation: %+v", got.Picker)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "svc")); !os.IsNotExist(err) {
		t.Fatalf("the addition was cloned before the take-up was refused (%v)", err)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "src")); err != nil || len(entries) != 1 {
		t.Fatalf("src/ holds %d checkouts, want 1 (err %v)", len(entries), err)
	}

	// Nothing is orphaned, so the same confirm succeeds once the refusal is gone.
	draft.Upgrades = nil
	if err := a.Confirm(dir, draft, Answer{Escalating: true, Awaited: true}, nil); err != nil {
		t.Fatal("retrying after a refused take-up failed:", err)
	}
}

// Two repositories read, both taken up, one branch.
func TestConfirmUpgradesAWholeBatchOntoOneBranch(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "Batch", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: referenced(testRepo(t, "docs"), testRepo(t, "specs")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Batch", Prefix: "chore",
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}, {Org: "org", Name: "specs"}}}
	if err := a.Confirm(dir, draft, Answer{}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 2 {
		t.Fatalf("repos after the batch = %+v", got.Repos)
	}
	for _, r := range got.Repos {
		if r.Role != session.RepoRoleEditing || r.Branch != "chore/batch" || r.Revision != "" {
			t.Fatalf("%s = %+v", r.Name, r)
		}
	}
}

// The take-up is recorded before the additions are cloned, so a clone that fails
// cannot leave the file calling a checkout pinned that is on the session branch.
func TestAFailedAdditionLeavesTheTakeUpRecorded(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "Half", Prefix: "feat", Mode: session.ModeAssistant, Repos: referenced(testRepo(t, "docs")),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	unclonable := github.Repo{Name: "ghost", Org: "org", SSHURL: filepath.Join(t.TempDir(), "nowhere"),
		DefaultBranch: "main"}
	draft := Draft{Name: "Half", Prefix: "feat", Repos: editing(unclonable),
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, Answer{Escalating: true, Awaited: true}, nil); err == nil {
		t.Fatal("a session was confirmed with a repository that cannot be cloned")
	}

	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r := got.Repos[0]; r.Role != session.RepoRoleEditing || r.Branch != "feat/half" || r.Revision != "" {
		t.Fatalf("the manifest describes a checkout that is not on disk: %+v", r)
	}
	if got.Picker != nil {
		t.Fatalf("a failed addition recorded an escalation: %+v", got.Picker)
	}
	branch, err := exec.Command("git", "-C", filepath.Join(dir, "src", "docs"), "branch", "--show-current").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(branch)) != "feat/half" {
		t.Fatalf("the checkout is on %q", strings.TrimSpace(string(branch)))
	}
}
