package session

// RepoStats against worktrees built the way a session builds them — a bare
// mirror under <root>/.mirrors and a worktree added off it — rather than a repo
// carrying a hand-made refs/remotes/origin/main. The base ref is the whole
// measurement, so it has to be the one a real clone produces.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
)

func TestRepoStatsMeasuresMirrorBackedWorktrees(t *testing.T) {
	requireGit(t)
	root := sessionsRoot(t)
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, "Webhook retry", "", "", "fix", ModeRPI, []RepoSelection{
		selection(t, "svc", RepoRoleActive),
		selection(t, "quiet", RepoRoleActive),
		selection(t, "level", RepoRoleActive),
		selection(t, "docs", RepoRoleReference),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// svc: two commits, one adding a two-line file and one dropping a line.
	svc := filepath.Join(dir, "src", "svc")
	writeLines(t, svc, "retry.go", "package svc", "func Retry() {}")
	run(t, svc, "add", ".")
	run(t, svc, "commit", "-m", "add retry")
	writeLines(t, svc, "README.md", "one", "three")
	run(t, svc, "commit", "-am", "drop a line")

	// quiet: a commit that changes nothing.
	run(t, filepath.Join(dir, "src", "quiet"), "commit", "--allow-empty", "-m", "nothing")

	// docs is a reference checkout, detached and pinned; scribbling on it must
	// not turn it into something with progress to report.
	docs := filepath.Join(dir, "src", "docs")
	if gitOK(dirFlag, docs, "symbolic-ref", quietFlag, headRef) {
		t.Fatal("reference checkout is not detached")
	}
	writeLines(t, docs, "notes.md", "scribble")
	run(t, docs, "add", ".")
	run(t, docs, "commit", "-m", "scribble")

	// level keeps the worktree exactly as it was added, at origin/main.

	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]RepoStat{
		"svc":   {Org: "org", Name: "svc", Role: RepoRoleActive, Commits: 2, Insertions: 2, Deletions: 1},
		"quiet": {Org: "org", Name: "quiet", Role: RepoRoleActive, Commits: 1},
		"level": {Org: "org", Name: "level", Role: RepoRoleActive},
		"docs":  {Org: "org", Name: "docs", Role: RepoRoleReference},
	}
	stats := RepoStats(root, m)
	if len(stats) != len(want) {
		t.Fatalf("measured %d repositories, want %d: %#v", len(stats), len(want), stats)
	}
	for _, got := range stats {
		if got != want[got.Name] {
			t.Errorf("%s = %#v, want %#v", got.Name, got, want[got.Name])
		}
	}
}

// A base branch that moved on since the session was cut is not the session's
// work: the diff is measured from the merge base, and a fetch on resume is what
// makes that distinction observable.
func TestRepoStatsIgnoresCommitsTheBaseBranchGainedAfterwards(t *testing.T) {
	requireGit(t)
	root := sessionsRoot(t)
	cfg := &config.Config{Root: root}
	origin := localOrigin(t, "svc")
	dir, err := Create(cfg, "Moved base", "", "", "fix", ModeRPI, []RepoSelection{
		{Repo: github.Repo{Name: "svc", Org: "org", SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleActive},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(dir, "src", "svc")
	writeLines(t, worktree, "retry.go", "package svc", "func Retry() {}")
	run(t, worktree, "add", ".")
	run(t, worktree, "commit", "-m", "add retry")

	upstream := strings.TrimPrefix(origin, "file://")
	writeLines(t, upstream, "README.md", "one", "two", "three", "four")
	run(t, upstream, "commit", "-am", "unrelated upstream work")

	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureWorktrees(cfg, m, nil); err != nil {
		t.Fatal(err)
	}
	// Without the resume fetch the base never moves and the rest proves nothing.
	if base, _ := resolveRevision(worktree, remoteRefPrefix+"main"); base != revisionOf(t, upstream, headRef) {
		t.Fatal("the fetch on resume left the base branch where it was")
	}
	want := RepoStat{Org: "org", Name: "svc", Role: RepoRoleActive, Commits: 1, Insertions: 2}
	if stats := RepoStats(root, m); len(stats) != 1 || stats[0] != want {
		t.Fatalf("stats = %#v, want %#v", stats, want)
	}
}

func revisionOf(t *testing.T, dir, ref string) string {
	t.Helper()
	sha, err := resolveRevision(dir, ref)
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(gitBin); err != nil {
		t.Skip("git is not installed: RepoStats has nothing to measure")
	}
}

func sessionsRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func selection(t *testing.T, name string, role RepoRole) RepoSelection {
	t.Helper()
	return RepoSelection{Repo: github.Repo{Name: name, Org: "org", SSHURL: localOrigin(t, name),
		DefaultBranch: "main"}, Role: role}
}

// localOrigin is a repository to clone a mirror from, addressed over file:// so
// the clone is a real transfer and needs no network.
func localOrigin(t *testing.T, name string) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), name)
	run(t, "", "init", "-b", "main", origin)
	writeLines(t, origin, "README.md", "one", "two", "three")
	run(t, origin, "add", ".")
	run(t, origin, "commit", "-m", "base")
	return "file://" + origin
}

func writeLines(t *testing.T, dir, name string, lines ...string) {
	t.Helper()
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
