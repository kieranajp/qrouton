package assembly

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

func makeTestOrigin(t *testing.T, name string) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), name)
	for _, args := range [][]string{
		{"init", "-b", "main", origin},
		{"-C", origin, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "initial"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return origin
}

func testRepo(t *testing.T, name string) github.Repo {
	t.Helper()
	return github.Repo{Name: name, Org: "org", SSHURL: makeTestOrigin(t, name), DefaultBranch: "main"}
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
	dir, err := session.Create(cfg, "scratch", "", "", "", session.ModeAssistant, "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return Assembler{Cfg: cfg}, dir
}

func TestConfirmWritesReposModeAndStanzaTogether(t *testing.T) {
	a, dir := scratch(t)
	draft := Draft{Name: "Webhook retry backoff", Prefix: "fix", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, true, nil); err != nil {
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
	if got.Escalation == nil || got.Escalation.Status != session.EscalationConfirmed || got.Escalation.At.IsZero() {
		t.Fatalf("confirmed stanza = %+v", got.Escalation)
	}
}

// The workbench's button adds repositories to the session as it stands. Moving
// it to RPI would also hand the next launch a fresh conversation, so an
// assistant session would lose its chat to a button that promised repositories.
func TestAddingReposLeavesTheModeAndConversationAlone(t *testing.T) {
	a, dir := scratch(t)
	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, false, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.EffectiveMode() != session.ModeAssistant {
		t.Fatalf("mode = %q, want assistant", got.Mode)
	}
	if got.Escalation != nil {
		t.Fatalf("adding repositories recorded an escalation: %+v", got.Escalation)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos = %+v", got.Repos)
	}
	if _, err := os.Stat(sessionpaths.HandoffPending(dir)); !os.IsNotExist(err) {
		t.Fatal("adding repositories owed the next launch a fresh conversation")
	}
}

// The final write must merge into what is on disk after a long clone, rather
// than restoring the mode and window record from before it began.
func TestAddingReposPreservesManifestChangesMadeDuringAssembly(t *testing.T) {
	a, dir := scratch(t)
	windows := []session.WindowRecord{{Kind: "terminal", Label: "tests", Cwd: "src/svc"}}
	wrote := false
	progress := func(p session.Progress) {
		if wrote || p.Step != session.ProgressWorktree || p.Status != session.ProgressCompleted {
			return
		}
		wrote = true
		if err := session.SetMode(dir, session.ModeRPI); err != nil {
			t.Error(err)
		}
		if err := session.SetWindows(dir, windows); err != nil {
			t.Error(err)
		}
	}
	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, false, progress); err != nil {
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
	if len(got.Windows) != 1 || got.Windows[0].Label != "tests" {
		t.Fatalf("windows written during assembly were lost: %+v", got.Windows)
	}
	if len(got.Repos) != 1 || got.Repos[0].Name != "svc" {
		t.Fatalf("assembled repository was lost: %+v", got.Repos)
	}
}

// The picker can sit open for the escalate tool's full ~30 minutes while the
// workbench keeps rewriting Windows underneath it. Confirm used to write back
// the manifest snapshot the picker loaded on open, discarding any Windows
// written since.
func TestConfirmPreservesWindowsWrittenAfterPickerOpened(t *testing.T) {
	a, dir := scratch(t)
	windows := []session.WindowRecord{{Kind: "terminal", Label: "repo", Cwd: "src/repo"}}
	if err := session.SetWindows(dir, windows); err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Webhook retry backoff", Prefix: "fix", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, true, nil); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 1 || got.Windows[0].Label != "repo" {
		t.Fatalf("confirm discarded Windows written after the picker opened: %+v", got.Windows)
	}
}

// A second trip through the picker used to derive a branch from the form's
// prefix, which defaults to feat — so a fix/… session's fourth repository
// landed on a branch of its own.
func TestAddedReposJoinTheSessionBranch(t *testing.T) {
	cfg := &config.Config{Orgs: []string{"org"}, Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, "Webhook retry", "", "", "fix", session.ModeRPI, "",
		editing(testRepo(t, "svc")), nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Webhook retry", Prefix: "feat", Repos: editing(testRepo(t, "contracts"))}
	if err := a.Confirm(dir, draft, false, nil); err != nil {
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
// checkout the work is in. Selecting the repo you are working on is the obvious
// move in the picker, and it used to produce a second clone of it on a second
// branch, leaving the original's commits behind on the old one.
func TestEscalationLeavesAnAlreadyPresentRepoAlone(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	repo := github.Repo{Name: "repo123", Org: "kieranajp", SSHURL: makeTestOrigin(t, "repo123"), DefaultBranch: "main"}
	dir, err := session.Create(cfg, "repo123", "", "", "feat", session.ModeAssistant, "", editing(repo), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Uncommitted work in the original checkout.
	stub := filepath.Join(dir, "src", "repo123", "stub.go")
	if err := os.WriteFile(stub, []byte("package stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	draft := Draft{Name: "Webhook retry backoff", Prefix: "fix", Repos: editing(repo)}
	if err := a.Confirm(dir, draft, true, nil); err != nil {
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

// Escalation signals the supervisor so the assistant is replaced by a fresh
// orchestrator; adding repositories leaves the running conversation alone.
func TestOnlyAnEscalationSignalsTheSupervisor(t *testing.T) {
	a, dir := scratch(t)
	var signalled []string
	a.Signal = func(root string) { signalled = append(signalled, root) }

	draft := Draft{Name: "scratch", Prefix: "feat", Repos: editing(testRepo(t, "svc"))}
	if err := a.Confirm(dir, draft, false, nil); err != nil {
		t.Fatal(err)
	}
	if len(signalled) != 0 {
		t.Fatalf("adding repositories signalled the supervisor: %v", signalled)
	}
	if err := a.Confirm(dir, draft, true, nil); err != nil {
		t.Fatal(err)
	}
	if len(signalled) != 1 || signalled[0] != dir {
		t.Fatalf("escalation signalled %v", signalled)
	}
}

func TestCancelWritesTheCancelledStanzaOnlyOnAnEscalation(t *testing.T) {
	dir := t.TempDir()
	if err := session.WriteManifest(dir, session.Manifest{Slug: "scratch", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	if err := Cancel(dir, false); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation != nil {
		t.Fatalf("a plain add-repos cancel wrote a stanza: %+v", got.Escalation)
	}

	if err := Cancel(dir, true); err != nil {
		t.Fatal(err)
	}
	got, err = session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation == nil || got.Escalation.Status != session.EscalationCancelled || got.Escalation.At.IsZero() {
		t.Fatalf("cancelled stanza = %+v", got.Escalation)
	}
	if got.EffectiveMode() != session.ModeAssistant || len(got.Repos) != 0 {
		t.Fatalf("cancel touched the session beyond the stanza: %+v", got)
	}
}

// Same staleness risk as confirm: cancel must not overwrite Windows the
// workbench wrote while the picker was up with the picker's own load-time copy.
func TestCancelPreservesWindowsWrittenAfterPickerOpened(t *testing.T) {
	dir := t.TempDir()
	if err := session.WriteManifest(dir, session.Manifest{Slug: "scratch", Mode: session.ModeAssistant}); err != nil {
		t.Fatal(err)
	}
	windows := []session.WindowRecord{{Kind: "terminal", Label: "repo", Cwd: "src/repo"}}
	if err := session.SetWindows(dir, windows); err != nil {
		t.Fatal(err)
	}
	if err := Cancel(dir, true); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Windows) != 1 || got.Windows[0].Label != "repo" {
		t.Fatalf("cancel discarded Windows written after the picker opened: %+v", got.Windows)
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
	dir, err := session.Create(cfg, "Read only", "", "", "feat", session.ModeAssistant, "",
		referenced(testRepo(t, "docs")), nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Read only", Prefix: "chore",
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, false, nil); err != nil {
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
	dir, err := session.Create(cfg, "Both", "", "", "feat", session.ModeAssistant, "",
		referenced(testRepo(t, "docs")), nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Both", Prefix: "fix", Repos: editing(testRepo(t, "svc")),
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, false, nil); err != nil {
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
	dir, err := session.Create(cfg, "Refused", "", "", "feat", session.ModeAssistant, "",
		referenced(testRepo(t, "docs")), nil)
	if err != nil {
		t.Fatal(err)
	}
	commit(t, filepath.Join(dir, "src", "docs"), "local work")

	draft := Draft{Name: "Refused", Prefix: "feat", Repos: editing(testRepo(t, "svc")),
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, true, nil); err == nil {
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
	if got.Escalation != nil {
		t.Fatalf("a refused take-up recorded an escalation: %+v", got.Escalation)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "svc")); !os.IsNotExist(err) {
		t.Fatalf("the addition was cloned before the take-up was refused (%v)", err)
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "src")); err != nil || len(entries) != 1 {
		t.Fatalf("src/ holds %d checkouts, want 1 (err %v)", len(entries), err)
	}

	// Nothing is orphaned, so the same confirm succeeds once the refusal is gone.
	draft.Upgrades = nil
	if err := a.Confirm(dir, draft, true, nil); err != nil {
		t.Fatal("retrying after a refused take-up failed:", err)
	}
}

// Two repositories read, both taken up, one branch.
func TestConfirmUpgradesAWholeBatchOntoOneBranch(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := Assembler{Cfg: cfg}
	dir, err := session.Create(cfg, "Batch", "", "", "feat", session.ModeAssistant, "",
		referenced(testRepo(t, "docs"), testRepo(t, "specs")), nil)
	if err != nil {
		t.Fatal(err)
	}
	draft := Draft{Name: "Batch", Prefix: "chore",
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}, {Org: "org", Name: "specs"}}}
	if err := a.Confirm(dir, draft, false, nil); err != nil {
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
	dir, err := session.Create(cfg, "Half", "", "", "feat", session.ModeAssistant, "",
		referenced(testRepo(t, "docs")), nil)
	if err != nil {
		t.Fatal(err)
	}
	unclonable := github.Repo{Name: "ghost", Org: "org", SSHURL: filepath.Join(t.TempDir(), "nowhere"),
		DefaultBranch: "main"}
	draft := Draft{Name: "Half", Prefix: "feat", Repos: editing(unclonable),
		Upgrades: []session.RepoRef{{Org: "org", Name: "docs"}}}
	if err := a.Confirm(dir, draft, true, nil); err == nil {
		t.Fatal("a session was confirmed with a repository that cannot be cloned")
	}

	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r := got.Repos[0]; r.Role != session.RepoRoleEditing || r.Branch != "feat/half" || r.Revision != "" {
		t.Fatalf("the manifest describes a checkout that is not on disk: %+v", r)
	}
	if got.Escalation != nil {
		t.Fatalf("a failed addition recorded an escalation: %+v", got.Escalation)
	}
	branch, err := exec.Command("git", "-C", filepath.Join(dir, "src", "docs"), "branch", "--show-current").Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(branch)) != "feat/half" {
		t.Fatalf("the checkout is on %q", strings.TrimSpace(string(branch)))
	}
}
