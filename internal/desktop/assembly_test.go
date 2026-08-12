package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
)

// Neither domain type is emittable as written: an error marshals to {} and an
// untagged field reaches the page under its Go name.
func TestTheRefreshAndProgressEventsRenderEveryFieldAndTheirErrors(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[refreshEvent](), reflect.TypeFor[progressEvent]()} {
		for i := range typ.NumField() {
			if typ.Field(i).Tag.Get("json") == "" {
				t.Errorf("%s.%s reaches the page under its Go name", typ.Name(), typ.Field(i).Name)
			}
		}
	}

	refresh, err := json.Marshal(newRefreshEvent(4, github.RefreshMsg{
		Owner: "acme", State: github.RefreshFailed, Err: errors.New("unavailable")}))
	if err != nil {
		t.Fatal(err)
	}
	var decodedRefresh map[string]any
	if err := json.Unmarshal(refresh, &decodedRefresh); err != nil {
		t.Fatal(err)
	}
	if decodedRefresh["error"] != "unavailable" || decodedRefresh["generation"] != 4.0 {
		t.Fatalf("refresh event = %s", refresh)
	}

	repo := github.Repo{Org: "acme", Name: "api"}
	progress, err := json.Marshal(newProgressEvent("cleanup", session.Progress{
		Step: session.ProgressMirror, Status: session.ProgressFailed, Repo: &repo,
		Role: session.RepoRoleEditing, Err: errors.New("clone failed")}))
	if err != nil {
		t.Fatal(err)
	}
	var decodedProgress map[string]any
	if err := json.Unmarshal(progress, &decodedProgress); err != nil {
		t.Fatal(err)
	}
	if decodedProgress["error"] != "clone failed" || decodedProgress["repo"] != "acme/api" ||
		decodedProgress["session"] != "cleanup" || decodedProgress["role"] != "editing" {
		t.Fatalf("progress event = %s", progress)
	}
}

func TestRunnersOffersOnlyTheAgentsWithAResolvedPath(t *testing.T) {
	a := &Assembly{runners: func() ([]assembly.Runner, error) {
		return []assembly.Runner{
			{ID: "claude", Label: "Claude Code", Installed: true},
			{ID: "codex", Label: "Codex CLI"},
		}, nil
	}}
	got, err := a.Runners()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "claude" {
		t.Fatalf("runners = %+v", got)
	}
	if _, err := (&Assembly{}).Runners(); !errors.Is(err, ErrNoAgentCommand) {
		t.Fatalf("a workbench with no runners answered %v", err)
	}
}

// A repository the page names that the live list no longer holds is dropped, so
// a draft whose only editing repo has gone is refused rather than assembled.
func TestADraftDropsRepositoriesTheListNoLongerHolds(t *testing.T) {
	repos := &Repositories{cfg: &config.Config{}, errs: map[string]error{},
		repos: []github.Repo{{Org: "acme", Name: "api", DefaultBranch: "main"}}}
	a := &Assembly{cfg: &config.Config{}, repos: repos}

	in := draftInput{Name: "Cleanup", Prefix: "feat", Repos: []repoPick{
		{ID: "acme/api", Role: "editing"},
		{ID: "acme/gone", Role: "editing"},
	}}
	if got := a.draft(in); len(got.Repos) != 1 || got.Repos[0].Repo.Name != "api" {
		t.Fatalf("draft repos = %+v", got.Repos)
	}
	if problems := a.Check(in); len(problems) != 0 {
		t.Fatalf("a draft keeping one editing repo was refused: %+v", problems)
	}

	in.Repos = []repoPick{{ID: "acme/gone", Role: "editing"}}
	problems := a.Check(in)
	if len(problems) != 1 || problems[0].Field != assembly.FieldRepos {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestPreviewAndPrefixesReachThePageThroughTheBinding(t *testing.T) {
	a := &Assembly{cfg: &config.Config{}, repos: &Repositories{cfg: &config.Config{}, errs: map[string]error{}}}
	if got := a.Preview(draftInput{Name: "API Cleanup!", Prefix: "fix"}); got != "fix/api-cleanup" {
		t.Fatalf("preview = %q", got)
	}
	if got := a.Prefixes(); len(got) == 0 || got[0] != "feat" {
		t.Fatalf("prefixes = %v", got)
	}
}

func testOrigin(t *testing.T, name string) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), name)
	for _, args := range [][]string{
		{"init", "-b", "main", origin},
		{"-C", origin, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false",
			"commit", "--allow-empty", "-m", "initial"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return origin
}

// Create is the whole of assembly from the overlay: the manifest, the worktrees,
// and a session that boots itself. A webview has no PTY to hand over, so the
// adoption is in process and the supervisor lands on a socket of its own.
func TestCreateAssemblesTheSessionAndBootsItOnItsOwnSocket(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	cfg := &config.Config{Root: root, Orgs: []string{"acme"}}
	repo := github.Repo{Org: "acme", Name: "api", SSHURL: testOrigin(t, "api"), DefaultBranch: "main"}
	repos := &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}

	var steps []progressEvent
	a := newAssembly(cfg, repos, reg, func(event string, payload any) {
		if event == assemblyProgressEvent {
			steps = append(steps, payload.(progressEvent))
		}
	}, nil, nil)

	in := draftInput{Name: "API Cleanup", Description: "tidy", Prefix: "fix", Mode: "rpi",
		Runner: "codex", Repos: []repoPick{{ID: "acme/api", Role: "editing"}}}
	if err := a.Create(in); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, "api-cleanup")
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "API Cleanup" || m.Description != "tidy" || m.EffectiveMode() != session.ModeRPI {
		t.Fatalf("manifest = %+v", m)
	}
	if len(m.Repos) != 1 || m.Repos[0].Role != session.RepoRoleEditing || m.Repos[0].Branch != "fix/api-cleanup" {
		t.Fatalf("manifest repos = %+v", m.Repos)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "api", ".git")); err != nil {
		t.Fatal("the worktree was not checked out:", err)
	}
	if steps == nil {
		t.Fatal("assembly reported no progress")
	}
	for _, step := range steps {
		if step.Session != "api-cleanup" {
			t.Fatalf("progress event names %q, not the session being assembled", step.Session)
		}
	}

	// Adopted with boot: the session came up here rather than being handed a PTY.
	if reg.current().root() != dir {
		t.Fatalf("the created session is not on screen: %q", reg.current().root())
	}
	if boot.sockets[dir] == "" {
		t.Fatal("the adopted session did not boot its own supervisor")
	}
	if boot.resumes[dir] {
		t.Fatal("a session assembled from nothing was told to resume a conversation")
	}
	if boot.runners[dir] != "codex" {
		t.Fatalf("the session booted the %q agent, not the one the overlay chose", boot.runners[dir])
	}
}

func TestCreateRefusesADraftItsOwnRulesReject(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := newAssembly(cfg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil, nil, nil, nil)
	err := a.Create(draftInput{Name: "Cleanup", Prefix: "feat"})
	if err == nil || !strings.Contains(err.Error(), "editing") {
		t.Fatalf("Create with no editing repo answered %v", err)
	}
}

// The rail freezes its order at the first poll and prepends anything it has not
// seen, so a session created mid-lifetime takes the front and the first shortcut.
func TestASessionCreatedMidLifetimeTakesTheFrontOfTheRail(t *testing.T) {
	reg := newSessions()
	first := reg.railOrder([]status.SessionRow{{Slug: "older"}})
	if len(first) != 1 || first[0].Slug != "older" {
		t.Fatalf("rail = %+v", first)
	}

	after := reg.railOrder([]status.SessionRow{{Slug: "fresh"}, {Slug: "older"}})
	if len(after) != 2 || after[0].Slug != "fresh" || after[1].Slug != "older" {
		t.Fatalf("a session created mid-lifetime landed at %+v", after)
	}
}

// A control whose effect expires after the first boot is worse than no control,
// so the session records the agent it was assembled with and every later boot
// reads it back.
func TestABootAfterTheFirstStartsTheAgentTheSessionRecords(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	cfg := &config.Config{Root: root, Orgs: []string{"acme"}}
	repo := github.Repo{Org: "acme", Name: "api", SSHURL: testOrigin(t, "api"), DefaultBranch: "main"}
	repos := &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}
	a := newAssembly(cfg, repos, reg, func(event string, payload any) {}, nil, nil)

	in := draftInput{Name: "API Cleanup", Prefix: "fix", Mode: "rpi", Runner: "codex",
		Repos: []repoPick{{ID: "acme/api", Role: "editing"}}}
	if err := a.Create(in); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "api-cleanup")

	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Runner != "codex" {
		t.Fatalf("manifest records runner %q, want the one the overlay chose", m.Runner)
	}

	// The rail booting it later names no runner, so the manifest's is what reaches
	// the agent command.
	reg.retire(reg.bySlug("api-cleanup"))
	if err := reg.Show("api-cleanup"); err != nil {
		t.Fatal(err)
	}
	if got := boot.runners[dir]; got != "codex" {
		t.Fatalf("the rail booted the %q agent, not the one the session records", got)
	}
}
