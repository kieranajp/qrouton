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
	"github.com/kieranajp/qrouton/internal/gittest"
)

func TestRepoStatsMeasuresMirrorBackedWorktrees(t *testing.T) {
	requireGit(t)
	root := sessionsRoot(t)
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, CreateRequest{Name: "Webhook retry", Prefix: "fix", Mode: ModeRPI, Repos: []RepoSelection{
		selection(t, "svc", RepoRoleEditing),
		selection(t, "quiet", RepoRoleEditing),
		selection(t, "level", RepoRoleEditing),
		selection(t, "docs", RepoRoleReference),
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// svc: two commits, one adding a two-line file and one dropping a line.
	svc := filepath.Join(dir, "src", "svc")
	gittest.WriteFile(t, svc, "retry.go", "package svc", "func Retry() {}")
	gittest.Run(t, svc, "add", ".")
	gittest.Run(t, svc, "commit", "-m", "add retry")
	gittest.WriteFile(t, svc, "README.md", "one", "three")
	gittest.Run(t, svc, "commit", "-am", "drop a line")

	// quiet: a commit that changes nothing.
	gittest.Run(t, filepath.Join(dir, "src", "quiet"), "commit", "--allow-empty", "-m", "nothing")

	// docs is a reference checkout, detached and pinned; scribbling on it must
	// not turn it into something with progress to report.
	docs := filepath.Join(dir, "src", "docs")
	if gitOK(dirFlag, docs, "symbolic-ref", quietFlag, headRef) {
		t.Fatal("reference checkout is not detached")
	}
	gittest.WriteFile(t, docs, "notes.md", "scribble")
	gittest.Run(t, docs, "add", ".")
	gittest.Run(t, docs, "commit", "-m", "scribble")

	// level keeps the worktree exactly as it was added, at origin/main.

	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]RepoStat{
		"svc":   {Org: "org", Name: "svc", Role: RepoRoleEditing, Commits: 2, Insertions: 2, Deletions: 1, Measured: true},
		"quiet": {Org: "org", Name: "quiet", Role: RepoRoleEditing, Commits: 1, Measured: true},
		"level": {Org: "org", Name: "level", Role: RepoRoleEditing, Measured: true},
		"docs":  {Org: "org", Name: "docs", Role: RepoRoleReference},
	}
	stats := RepoStats(t.Context(), root, m)
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
// work: the diff is measured from the merge base.
func TestRepoStatsIgnoresCommitsTheBaseBranchGainedAfterwards(t *testing.T) {
	requireGit(t)
	root := sessionsRoot(t)
	cfg := &config.Config{Root: root}
	origin := localOrigin(t, "svc")
	dir, err := Create(cfg, CreateRequest{Name: "Moved base", Prefix: "fix", Mode: ModeRPI, Repos: []RepoSelection{
		{Repo: github.Repo{Name: "svc", Org: "org", SSHURL: origin, DefaultBranch: "main"}, Role: RepoRoleEditing},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(dir, "src", "svc")
	gittest.WriteFile(t, worktree, "retry.go", "package svc", "func Retry() {}")
	gittest.Run(t, worktree, "add", ".")
	gittest.Run(t, worktree, "commit", "-m", "add retry")

	upstream := strings.TrimPrefix(origin, "file://")
	gittest.WriteFile(t, upstream, "README.md", "one", "two", "three", "four")
	gittest.Run(t, upstream, "commit", "-am", "unrelated upstream work")

	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureMirror(root, "org", "svc", origin, nil); err != nil {
		t.Fatal(err)
	}
	// Without the fetch the base never moves and the rest proves nothing.
	if base, _ := resolveRevision(worktree, remoteRefPrefix+"main"); base != revisionOf(t, upstream, headRef) {
		t.Fatal("the fetch left the base branch where it was")
	}
	want := RepoStat{Org: "org", Name: "svc", Role: RepoRoleEditing, Commits: 1, Insertions: 2, Measured: true}
	if stats := RepoStats(t.Context(), root, m); len(stats) != 1 || stats[0] != want {
		t.Fatalf("stats = %#v, want %#v", stats, want)
	}
}

// An older manifest's blank default branch must never reach git as "origin/".
func TestRepoStatsLeavesAnEmptyDefaultBranchUnmeasured(t *testing.T) {
	requireGit(t)
	root := sessionsRoot(t)
	cfg := &config.Config{Root: root}
	dir, err := Create(cfg, CreateRequest{Name: "No base branch", Prefix: "fix", Mode: ModeRPI, Repos: []RepoSelection{
		selection(t, "svc", RepoRoleEditing),
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Repos[0].DefaultBranch = ""

	stats := RepoStats(t.Context(), root, m)
	if len(stats) != 1 || stats[0].Measured {
		t.Fatalf("stats = %#v, want an unmeasured repository", stats)
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

// localOrigin is a repository to clone a mirror from, addressed over file:// so
// the clone is a real transfer and needs no network. Its three lines are what
// the diff counts in these tests measure against.
func localOrigin(t *testing.T, name string) string {
	t.Helper()
	return gittest.Origin(t, name,
		gittest.WithFile("README.md", "one", "two", "three"),
		gittest.WithMessage("base"),
		gittest.AsFileURL())
}

func selection(t *testing.T, name string, role RepoRole) RepoSelection {
	t.Helper()
	return RepoSelection{Repo: github.Repo{Name: name, Org: "org", SSHURL: localOrigin(t, name),
		DefaultBranch: "main"}, Role: role}
}
