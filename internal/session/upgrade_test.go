package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
)

// referenceSession is a session holding reference checkouts of the named
// repositories, with the origins they were mirrored from.
func referenceSession(t *testing.T, names ...string) (*config.Config, string, []string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	sels := make([]RepoSelection, 0, len(names))
	origins := make([]string, 0, len(names))
	for _, name := range names {
		origin, _ := makeOrigin(t, name)
		origins = append(origins, origin)
		sels = append(sels, RepoSelection{Role: RepoRoleReference,
			Repo: github.Repo{Name: name, Org: "org", SSHURL: origin, DefaultBranch: "main"}})
	}
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, strings.Join(names, "-"), "", "", "feat", ModeAssistant, "", sels, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, dir, origins
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return strings.TrimSpace(string(out))
}

func refs(names ...string) []RepoRef {
	out := make([]RepoRef, 0, len(names))
	for _, name := range names {
		out = append(out, RepoRef{Org: "org", Name: name})
	}
	return out
}

// The point of the whole change: a repository the session read is worked in, on
// the session branch, without a second clone.
func TestUpgradeReposMovesAReferenceCheckoutOntoTheSessionBranch(t *testing.T) {
	cfg, dir, origins := referenceSession(t, "docs")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	pinned := m.Repos[0].Revision

	// The default branch has moved on since the pin; the take-up follows its tip,
	// so the repo shares the base its siblings were cut from.
	os.WriteFile(filepath.Join(origins[0], "version"), []byte("two"), 0o644)
	run(t, origins[0], "add", ".")
	run(t, origins[0], "commit", "-m", "advance")
	tip := gitOutput(t, origins[0], "rev-parse", "HEAD")

	if err := UpgradeRepos(cfg, m, []RepoRef{{Org: "ORG", Name: "DOCS"}}, "feat/docs", nil); err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, wt, "branch", "--show-current"); got != "feat/docs" {
		t.Fatalf("checkout is on %q, not the session branch", got)
	}
	if got := gitOutput(t, wt, "rev-parse", "HEAD"); got != tip {
		t.Fatalf("checkout at %s, want the default branch tip %s (pinned was %s)", got, tip, pinned)
	}

	// UpgradeRepos writes nothing: the caller folds ApplyUpgrades into one write.
	stored, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Repos[0].Role != RepoRoleReference || stored.Repos[0].Revision != pinned {
		t.Fatalf("UpgradeRepos wrote the manifest: %+v", stored.Repos[0])
	}
	out, err := ApplyUpgrades(stored, refs("docs"), "feat/docs")
	if err != nil {
		t.Fatal(err)
	}
	if r := out.Repos[0]; r.Role != RepoRoleEditing || r.Branch != "feat/docs" || r.Revision != "" {
		t.Fatalf("rewritten manifest repo = %+v", r)
	}
}

// The mirror is shared and expensive. A take-up fetches it and nothing more.
func TestUpgradeReposReusesTheExistingMirror(t *testing.T) {
	cfg, dir, _ := referenceSession(t, "shared")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	mirror := mirrorPath(cfg.Root, "org", "shared")
	before, err := os.Stat(mirror)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpgradeRepos(cfg, m, refs("shared"), "feat/shared", nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(mirror)
	if err != nil {
		t.Fatal("the mirror did not survive the take-up:", err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("the mirror was re-cloned rather than reused")
	}
}

// A reference checkout is read-only by role, not by permission, and anything git
// does not track has no copy anywhere else. A switch in place keeps all of it
// where replacing the worktree would not.
func TestUpgradeReposKeepsWhatGitDoesNotTrack(t *testing.T) {
	cfg, dir, _ := referenceSession(t, "built")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	kept := map[string]string{".env": "SECRET=1\n", "node_modules/dep": "vendored\n"}
	if err := os.Mkdir(filepath.Join(wt, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range kept {
		if err := os.WriteFile(filepath.Join(wt, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := UpgradeRepos(cfg, m, refs("built"), "feat/built", nil); err != nil {
		t.Fatal(err)
	}
	for name, want := range kept {
		body, err := os.ReadFile(filepath.Join(wt, name))
		if err != nil {
			t.Fatalf("%s did not survive the take-up: %v", name, err)
		}
		if string(body) != want {
			t.Fatalf("%s = %q, want %q", name, body, want)
		}
	}
	if got := gitOutput(t, wt, "branch", "--show-current"); got != "feat/built" {
		t.Fatalf("checkout is on %q", got)
	}
}

func TestUpgradeReposRefusesACheckoutThatHasMovedOffItsPin(t *testing.T) {
	cfg, dir, _ := referenceSession(t, "moved")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	os.WriteFile(filepath.Join(wt, "note"), []byte("mine"), 0o644)
	run(t, wt, "add", ".")
	run(t, wt, "commit", "-m", "local work")
	local := gitOutput(t, wt, "rev-parse", "HEAD")

	err = UpgradeRepos(cfg, m, refs("moved"), "feat/moved", nil)
	if !errors.Is(err, ErrReferenceMoved) {
		t.Fatalf("take-up of a moved checkout = %v", err)
	}
	if !strings.Contains(err.Error(), "org/moved") {
		t.Fatalf("refusal does not name the repository: %v", err)
	}
	if got := gitOutput(t, wt, "rev-parse", "HEAD"); got != local {
		t.Fatalf("the refused checkout moved to %s", got)
	}
}

// An editing checkout carries the session's own work, so the picker may not
// re-role it; a repository the session does not hold cannot be re-roled either.
func TestUpgradeReposRefusesAnythingButAReferenceRepoItHolds(t *testing.T) {
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	origin, _ := makeOrigin(t, "svc")
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, "Held", "", "", "feat", ModeRPI, "", []RepoSelection{
		{Repo: github.Repo{Name: "svc", Org: "org", SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleEditing},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpgradeRepos(cfg, m, refs("svc"), "feat/held", nil); !errors.Is(err, ErrNotReference) {
		t.Fatalf("take-up of an editing repo = %v", err)
	}
	if err := UpgradeRepos(cfg, m, refs("kraken"), "feat/held", nil); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("take-up of an unheld repo = %v", err)
	}
	if _, err := ApplyUpgrades(m, refs("kraken"), "feat/held"); !errors.Is(err, ErrNotHeld) {
		t.Fatalf("rewriting an unheld repo = %v", err)
	}
}

// One refusal must not leave half the batch switched and the manifest describing
// neither state, so every ref is checked before any checkout is touched.
func TestUpgradeReposTouchesNothingWhenOneOfTheBatchIsRefused(t *testing.T) {
	cfg, dir, _ := referenceSession(t, "alpha", "bravo")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	bravo := filepath.Join(dir, m.Repos[1].WorktreePath)
	run(t, bravo, "commit", "--allow-empty", "-m", "local work")

	err = UpgradeRepos(cfg, m, refs("alpha", "bravo"), "feat/both", nil)
	if !errors.Is(err, ErrReferenceMoved) {
		t.Fatalf("batch with one refusal = %v", err)
	}
	alpha := filepath.Join(dir, m.Repos[0].WorktreePath)
	if got := gitOutput(t, alpha, "branch", "--show-current"); got != "" {
		t.Fatalf("alpha was switched to %q before bravo was refused", got)
	}
}

// A take-up whose manifest write never landed leaves the checkout on the branch
// and the entry still saying reference — off its pin, which is what the refusal
// above tests for. Retrying has to finish it rather than blame the user.
func TestUpgradeReposCompletesATakeUpWhoseWriteNeverLanded(t *testing.T) {
	cfg, dir, origins := referenceSession(t, "again")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(origins[0], "version"), []byte("two"), 0o644)
	run(t, origins[0], "add", ".")
	run(t, origins[0], "commit", "-m", "advance")

	if err := UpgradeRepos(cfg, m, refs("again"), "feat/again", nil); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	if got := gitOutput(t, wt, "rev-parse", "HEAD"); got == m.Repos[0].Revision {
		t.Fatal("the checkout did not move off its pin, so the retry proves nothing")
	}

	if err := UpgradeRepos(cfg, m, refs("again"), "feat/again", nil); err != nil {
		t.Fatal("a take-up already on disk was not completable:", err)
	}
	if got := gitOutput(t, wt, "branch", "--show-current"); got != "feat/again" {
		t.Fatalf("checkout is on %q", got)
	}
}

// A pruned checkout has nothing to keep, so it is materialised on the branch.
func TestUpgradeReposMaterialisesAPrunedCheckoutStraightOntoTheBranch(t *testing.T) {
	cfg, dir, _ := referenceSession(t, "pruned")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	if err := os.RemoveAll(wt); err != nil {
		t.Fatal(err)
	}
	if err := UpgradeRepos(cfg, m, refs("pruned"), "feat/pruned", nil); err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, wt, "branch", "--show-current"); got != "feat/pruned" {
		t.Fatalf("re-materialised checkout is on %q", got)
	}
}

// A take-up reports the same two steps an addition does: the fetch is real, and so
// is the branch. Nothing draws them yet, and an overlay that grows a progress rail
// must not have to special-case one of the two ways a repo becomes editable.
func TestUpgradeReposReportsItsFetchAndItsWorktree(t *testing.T) {
	cfg, dir, _ := referenceSession(t, "watched")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var events []Progress
	if err := UpgradeRepos(cfg, m, refs("watched"), "feat/watched",
		func(event Progress) { events = append(events, event) }); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		step   ProgressStep
		status ProgressStatus
	}{
		{ProgressMirror, ProgressStarted}, {ProgressMirror, ProgressCompleted},
		{ProgressWorktree, ProgressStarted}, {ProgressWorktree, ProgressCompleted},
	}
	outcomes := make([]Progress, 0, len(want))
	for _, event := range events {
		if event.Status != ProgressAdvanced {
			outcomes = append(outcomes, event)
		}
	}
	if len(outcomes) != len(want) {
		t.Fatalf("got %d outcome events, want %d: %+v", len(outcomes), len(want), outcomes)
	}
	for i := range want {
		if outcomes[i].Step != want[i].step || outcomes[i].Status != want[i].status {
			t.Fatalf("event %d = %+v, want step=%s status=%s", i, outcomes[i], want[i].step, want[i].status)
		}
	}
	if outcomes[0].Repo == nil || outcomes[0].Repo.Name != "watched" || outcomes[0].Role != RepoRoleEditing {
		t.Fatalf("take-up progress lacks context: %+v", outcomes[0])
	}
}

// The safety this rests on: git will not move a checkout over work in it. What
// the user reads is a sentence, not the paragraph git names every file in.
func TestUpgradeReposRefusesToOverwriteUncommittedWork(t *testing.T) {
	cfg, dir, origins := referenceSession(t, "edited")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The same tracked file changed on both sides, so the switch cannot keep both.
	os.WriteFile(filepath.Join(origins[0], "version"), []byte("upstream"), 0o644)
	run(t, origins[0], "add", ".")
	run(t, origins[0], "commit", "-m", "advance")
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	if err := os.WriteFile(filepath.Join(wt, "version"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	err = UpgradeRepos(cfg, m, refs("edited"), "feat/edited", nil)
	if !errors.Is(err, ErrCheckoutHasWork) {
		t.Fatalf("take-up over conflicting work = %v", err)
	}
	if !strings.Contains(err.Error(), "org/edited") {
		t.Fatalf("refusal does not name the repository: %v", err)
	}
	if strings.Contains(err.Error(), gitBin+" [") {
		t.Fatalf("refusal is git's own paragraph: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(wt, "version"))
	if err != nil || string(body) != "mine" {
		t.Fatalf("the edit was overwritten: %q (%v)", body, err)
	}
	if got := gitOutput(t, wt, "branch", "--show-current"); got != "" {
		t.Fatalf("the refused checkout moved onto %q", got)
	}
}

// A change git can keep is kept: the switch carries it onto the session branch,
// where it can be committed, rather than refusing work nothing threatens.
func TestUpgradeReposCarriesWorkGitCanKeep(t *testing.T) {
	cfg, dir, _ := referenceSession(t, "carried")
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	if err := os.WriteFile(filepath.Join(wt, "version"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UpgradeRepos(cfg, m, refs("carried"), "feat/carried", nil); err != nil {
		t.Fatal(err)
	}
	if got := gitOutput(t, wt, "branch", "--show-current"); got != "feat/carried" {
		t.Fatalf("checkout is on %q", got)
	}
	body, err := os.ReadFile(filepath.Join(wt, "version"))
	if err != nil || string(body) != "mine" {
		t.Fatalf("the edit did not come along: %q (%v)", body, err)
	}
}
