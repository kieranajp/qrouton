package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
)

// boardAnswer is what the fake GitHub says: rows and errors per repository
// name, or one error for the whole request. spent rides beside the rows.
type boardAnswer struct {
	prs   map[string][]github.PullRequest
	errs  map[string]error
	err   error
	spent error
}

type fakeBoardGitHub struct {
	mu          sync.Mutex
	token       string
	answer      boardAnswer
	credentials int
	calls       [][]github.BranchTarget
}

func (g *fakeBoardGitHub) say(a boardAnswer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.answer = a
}

func (g *fakeBoardGitHub) asked() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.calls)
}

func (g *fakeBoardGitHub) credential(context.Context) (github.Credential, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.credentials++
	if g.token == "" {
		return github.CredentialSignedOut, ""
	}
	return github.CredentialCLI, g.token
}

func (g *fakeBoardGitHub) pulls(_ context.Context, _ string, targets []github.BranchTarget) (string, []github.BranchResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, targets)
	if g.answer.err != nil {
		return "", nil, g.answer.err
	}
	results := make([]github.BranchResult, len(targets))
	for i, target := range targets {
		if err := g.answer.errs[target.Name]; err != nil {
			results[i].Err = err
			continue
		}
		results[i].PullRequests = append([]github.PullRequest{}, g.answer.prs[target.Name]...)
	}
	return "kieran", results, g.answer.spent
}

type testBoard struct {
	*Dashboard
	gh    *fakeBoardGitHub
	clock *chromeClock
	root  string
	emits atomic.Int32
	last  chan DashboardStatus
}

// newTestBoard is a dashboard over a sessions root of its own, a fake GitHub,
// a git measurement that answers nothing, a fixed clock and a temporary cache.
func newTestBoard(t *testing.T) *testBoard {
	t.Helper()
	b := &testBoard{
		gh:    &fakeBoardGitHub{token: "t"},
		clock: &chromeClock{at: time.Date(2026, 11, 20, 8, 41, 0, 0, time.UTC)},
		root:  t.TempDir(),
		last:  make(chan DashboardStatus, 1),
	}
	b.Dashboard = newDashboard(b.root, func() bool { return false }, func(event string, payload any) {
		if event != dashboardEvent {
			return
		}
		b.emits.Add(1)
		select {
		case <-b.last:
		default:
		}
		b.last <- payload.(DashboardStatus)
	}, dashboardAPI{
		credential: b.gh.credential,
		pulls:      b.gh.pulls,
		measure:    func(context.Context, string) []status.RepoStat { return []status.RepoStat{} },
	}, b.clock.now, filepath.Join(t.TempDir(), "cache", "pulls.json"))
	b.every, b.shownEvery = time.Hour, time.Hour
	t.Cleanup(b.stop)
	return b
}

// session writes a session holding repos and answers its root.
func (b *testBoard) session(t *testing.T, slug string, repos ...session.ManifestRepo) string {
	t.Helper()
	dir := sessionDir(t, b.root, slug)
	if err := session.WriteManifest(dir, session.Manifest{Slug: slug, Repos: repos}); err != nil {
		t.Fatal(err)
	}
	return dir
}

// waitFor answers the first emitted status that satisfies want.
func (b *testBoard) waitFor(t *testing.T, what string, want func(DashboardStatus) bool) DashboardStatus {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case got := <-b.last:
			if want(got) {
				return got
			}
		case <-deadline:
			t.Fatalf("never emitted %s", what)
		}
	}
}

func (b *testBoard) cached(t *testing.T) pullsCache {
	t.Helper()
	raw, err := os.ReadFile(b.cachePath)
	if err != nil {
		t.Fatal(err)
	}
	var c pullsCache
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func editing(name, branch string) session.ManifestRepo {
	return session.ManifestRepo{Org: "acme", Name: name, Role: session.RepoRoleEditing, Branch: branch, WorktreePath: "src/" + name}
}

func pr(n int, title string) github.PullRequest {
	return github.PullRequest{Number: n, Title: title, URL: "https://github.com/acme/x/pull/" + title, State: github.PullStateOpen}
}

func titles(list status.PullRequestList) []string {
	out := []string{}
	for _, item := range list.Items {
		out = append(out, item.Repo+"#"+item.Title)
	}
	return out
}

func TestDashboardKeepsTheLastRowsOnAnAliasError(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"), editing("api", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}, "api": {pr(3, "c")}}})
	b.refresh(t.Context())

	b.gh.say(boardAnswer{
		prs:  map[string][]github.PullRequest{"api": {pr(4, "d")}},
		errs: map[string]error{"web": fmt.Errorf("github: %w: Something went wrong", github.ErrQuery)},
	})
	b.refresh(t.Context())

	if rows := titles(b.rows(root)); !reflect.DeepEqual(rows, []string{"acme/web#a", "acme/api#d"}) {
		t.Fatalf("rows after web's error = %v", rows)
	}
	if kept := b.cached(t).Sessions["pulls"]["acme/web"]["feat/x"]; len(kept) != 1 || kept[0].Title != "a" {
		t.Fatalf("pulls.json dropped web's rows: %+v", b.cached(t).Sessions)
	}
	if s := b.Snapshot(); s.Failed || s.SignedOut {
		t.Fatalf("one alias failing marked the refresh %+v", s)
	}
}

func TestDashboardDropsOnlyTheRepositoryGitHubCannotFind(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"), editing("api", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}, "api": {pr(3, "c")}}})
	b.refresh(t.Context())

	b.gh.say(boardAnswer{
		prs:  map[string][]github.PullRequest{"api": {pr(3, "c")}},
		errs: map[string]error{"web": fmt.Errorf("github: %w: Could not resolve", github.ErrNotFound)},
	})
	b.refresh(t.Context())

	if rows := b.rows(root); !reflect.DeepEqual(titles(rows), []string{"acme/api#c"}) || rows.SignedOut {
		t.Fatalf("rows after web went missing = %v, signed out %v", titles(rows), rows.SignedOut)
	}
}

// Two sessions on one branch share one alias, so its failure is one line too.
func TestDashboardLogsARepeatedAliasFailureOnce(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	b := newTestBoard(t)
	b.session(t, "one", editing("web", "feat/x"))
	b.session(t, "two", editing("web", "feat/x"))
	b.gh.say(boardAnswer{errs: map[string]error{"web": fmt.Errorf("github: %w: 502 Bad Gateway", github.ErrQuery)}})

	for range 3 {
		b.refresh(t.Context())
	}

	if got := strings.Count(logged.String(), "502 Bad Gateway"); got != 1 {
		t.Fatalf("logged the failure %d times:\n%s", got, logged.String())
	}
}

func TestDashboardAsksOnceForABranchTwoSessionsShare(t *testing.T) {
	b := newTestBoard(t)
	one := b.session(t, "one", editing("web", "feat/x"))
	two := b.session(t, "two", editing("web", "feat/x"), editing("api", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})

	b.refresh(t.Context())

	if calls := b.gh.calls; len(calls) != 1 || len(calls[0]) != 2 {
		t.Fatalf("asked for %+v", calls)
	}
	first, second := titles(b.rows(one)), titles(b.rows(two))
	if !reflect.DeepEqual(first, []string{"acme/web#a"}) || !reflect.DeepEqual(second, first) {
		t.Fatalf("shared branch rows = %v and %v", first, second)
	}
}

func TestDashboardFailsWholeOnAnErrorWithNoAliasPath(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})
	b.refresh(t.Context())
	written, err := os.ReadFile(b.cachePath)
	if err != nil {
		t.Fatal(err)
	}
	checked := b.Snapshot().CheckedAt

	b.clock.advance(time.Minute)
	b.gh.say(boardAnswer{err: fmt.Errorf("github: listing pull requests: %w: Parse error", github.ErrQuery)})
	b.refresh(t.Context())

	s := b.Snapshot()
	if !s.Failed || s.Refreshing || !s.CheckedAt.Equal(checked) {
		t.Fatalf("status after a whole failure = %+v", s)
	}
	if rows := titles(b.rows(root)); !reflect.DeepEqual(rows, []string{"acme/web#a"}) {
		t.Fatalf("rows after a whole failure = %v", rows)
	}
	if now, _ := os.ReadFile(b.cachePath); !bytes.Equal(now, written) {
		t.Fatalf("a whole failure rewrote pulls.json:\n%s", now)
	}

	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})
	b.refresh(t.Context())
	if s := b.Snapshot(); s.Failed || !s.CheckedAt.Equal(b.clock.now()) {
		t.Fatalf("status after recovering = %+v", s)
	}
}

func TestDashboardWithoutACredentialOrWithARejectedTokenIsSignedOutAndKeepsRows(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})
	b.refresh(t.Context())

	b.gh.token = ""
	b.refresh(t.Context())
	if rows := b.rows(root); !rows.SignedOut || len(rows.Items) != 1 || b.gh.asked() != 1 {
		t.Fatalf("no credential: rows = %+v, asked %d times", rows, b.gh.asked())
	}

	b.gh.token = "t"
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})
	b.refresh(t.Context())
	if b.rows(root).SignedOut {
		t.Fatal("a good token left the dashboard signed out")
	}
	b.gh.say(boardAnswer{err: fmt.Errorf("github: %w: 401 Unauthorized", github.ErrTokenRejected)})
	b.refresh(t.Context())
	if rows := b.rows(root); !rows.SignedOut || len(rows.Items) != 1 || !b.Snapshot().SignedOut {
		t.Fatalf("rejected token: rows = %+v", rows)
	}
}

func TestASessionWithNoBranchedRepoIsNeverSignedOut(t *testing.T) {
	b := newTestBoard(t)
	branched := b.session(t, "branched", editing("web", "feat/x"))
	reference := b.session(t, "reference", session.ManifestRepo{Org: "acme", Name: "docs", Role: session.RepoRoleReference, WorktreePath: "src/docs"})
	unbranched := b.session(t, "unbranched", editing("api", ""))
	b.gh.token = ""
	b.refresh(t.Context())

	if !b.rows(branched).SignedOut {
		t.Fatal("a branched session did not show the sign-in note")
	}
	if b.rows(reference).SignedOut || b.rows(unbranched).SignedOut {
		t.Fatal("a session with nothing to fetch showed the sign-in note")
	}
}

func TestARateLimitedRefreshKeepsRowsAndWaitsForTheReset(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})
	b.refresh(t.Context())

	until := b.clock.now().Add(10 * time.Minute)
	b.gh.say(boardAnswer{err: fmt.Errorf("github: listing pull requests: %w", &github.RateLimitError{Until: until})})
	b.refresh(t.Context())
	if s := b.Snapshot(); !s.PausedUntil.Equal(until) || s.Failed || s.Refreshing {
		t.Fatalf("status when rate limited = %+v", s)
	}
	if rows := titles(b.rows(root)); !reflect.DeepEqual(rows, []string{"acme/web#a"}) {
		t.Fatalf("rows when rate limited = %v", rows)
	}

	b.clock.advance(9 * time.Minute)
	b.refresh(t.Context())
	if asked := b.gh.asked(); asked != 2 {
		t.Fatalf("asked GitHub %d times before the reset", asked)
	}

	b.clock.advance(time.Minute)
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})
	b.refresh(t.Context())
	if asked, s := b.gh.asked(), b.Snapshot(); asked != 3 || !s.PausedUntil.IsZero() {
		t.Fatalf("at the reset: asked %d times, status %+v", asked, s)
	}
}

// A 200 that spends the last point still answered, so dropping its rows loses a paid-for refresh.
func TestARefreshThatSpendsTheLastPointKeepsItsRowsAndPausesUntilTheReset(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"))
	until := b.clock.now().Add(10 * time.Minute)
	b.gh.say(boardAnswer{
		prs:   map[string][]github.PullRequest{"web": {pr(1, "a")}},
		spent: fmt.Errorf("github: listing pull requests: %w", &github.RateLimitError{Until: until}),
	})
	b.refresh(t.Context())

	if rows := titles(b.rows(root)); !reflect.DeepEqual(rows, []string{"acme/web#a"}) {
		t.Fatalf("rows from the last point = %v", rows)
	}
	if kept := b.cached(t).Sessions["pulls"]["acme/web"]["feat/x"]; len(kept) != 1 || kept[0].Title != "a" {
		t.Fatalf("pulls.json = %+v", b.cached(t).Sessions)
	}
	if s := b.Snapshot(); !s.PausedUntil.Equal(until) || s.Failed || !s.CheckedAt.Equal(b.clock.now()) {
		t.Fatalf("status after spending the last point = %+v", s)
	}

	b.clock.advance(time.Minute)
	b.refresh(t.Context())
	if asked := b.gh.asked(); asked != 1 {
		t.Fatalf("asked GitHub %d times before the reset", asked)
	}
}

func TestDashboardWithNoTargetsNeitherRequestsNorWrites(t *testing.T) {
	b := newTestBoard(t)
	b.session(t, "reference", session.ManifestRepo{Org: "acme", Name: "docs", Role: session.RepoRoleReference, Branch: "feat/x"})
	b.session(t, "unbranched", editing("web", ""))
	b.session(t, "empty")

	b.refresh(t.Context())
	b.refresh(t.Context())

	if b.gh.credentials != 0 || b.gh.asked() != 0 {
		t.Fatalf("looked up a credential %d times and asked GitHub %d times", b.gh.credentials, b.gh.asked())
	}
	if _, err := os.Stat(b.cachePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("pulls.json was written: %v", err)
	}
	if got := b.emits.Load(); got != 1 {
		t.Fatalf("two refreshes that changed nothing emitted %d times", got)
	}
}

func TestDashboardLoadsTheCacheBeforeTheFirstRefresh(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"))
	checked := time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)
	row := status.PullRequest{Repo: "acme/web", Number: 1, Title: "yesterday", State: status.PullRequestStateOpen}
	stale := status.PullRequest{Repo: "acme/web", Number: 2, Title: "old branch"}
	if err := writePullsCache(b.cachePath, pullsCache{
		SchemaVersion: pullsCacheSchemaVersion, Viewer: "kieran", CheckedAt: checked,
		Sessions: map[string]map[string]map[string][]status.PullRequest{
			"pulls": {"acme/web": {"feat/x": {row}, "feat/old": {stale}}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	d := newDashboard(b.root, func() bool { return false }, nil, dashboardAPI{}, b.clock.now, b.cachePath)

	if rows := d.rows(root); !reflect.DeepEqual(rows.Items, []status.PullRequest{row}) {
		t.Fatalf("rows before any refresh = %+v", rows)
	}
	s := d.Snapshot()
	if s.Viewer != "kieran" || !s.CheckedAt.Equal(checked) || !reflect.DeepEqual(s.Sessions["pulls"].PullRequests, []status.PullRequest{row}) {
		t.Fatalf("snapshot before any refresh = %+v", s)
	}
}

func TestDashboardWritesOnlyTheCurrentBranchesOfSessionsOnDisk(t *testing.T) {
	b := newTestBoard(t)
	one := b.session(t, "one", editing("web", "feat/x"))
	two := b.session(t, "two", editing("api", "feat/y"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}, "api": {pr(2, "b")}}})
	b.refresh(t.Context())

	if err := session.WriteManifest(one, session.Manifest{Slug: "one", Repos: []session.ManifestRepo{editing("web", "feat/z")}}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(two); err != nil {
		t.Fatal(err)
	}
	b.refresh(t.Context())

	got := b.cached(t)
	want := map[string]map[string]map[string][]status.PullRequest{"one": {"acme/web": {"feat/z": pullRequestRows("acme/web", []github.PullRequest{pr(1, "a")})}}}
	if got.SchemaVersion != pullsCacheSchemaVersion || got.Viewer != "kieran" || !reflect.DeepEqual(got.Sessions, want) {
		t.Fatalf("pulls.json = %+v", got)
	}
}

func TestDashboardRowsFollowManifestRepoOrder(t *testing.T) {
	b := newTestBoard(t)
	root := b.session(t, "pulls", editing("web", "feat/x"), editing("api", "feat/x"), editing("cli", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}, "api": {pr(2, "b")}, "cli": {pr(3, "c")}}})
	b.refresh(t.Context())

	for range 20 {
		if rows := titles(b.rows(root)); !reflect.DeepEqual(rows, []string{"acme/web#a", "acme/api#b", "acme/cli#c"}) {
			t.Fatalf("rows = %v", rows)
		}
	}
	if err := session.WriteManifest(root, session.Manifest{Slug: "pulls", Repos: []session.ManifestRepo{
		editing("cli", "feat/x"), editing("web", "feat/x"), editing("api", "feat/x"),
	}}); err != nil {
		t.Fatal(err)
	}
	if rows := titles(b.rows(root)); !reflect.DeepEqual(rows, []string{"acme/cli#c", "acme/web#a", "acme/api#b"}) {
		t.Fatalf("rows after reordering the manifest = %v", rows)
	}
}

// signal is a fake call that reports each root or target list it is asked for.
type signal chan struct{}

func (s signal) expect(t *testing.T, what string) {
	t.Helper()
	select {
	case <-s:
	case <-time.After(5 * time.Second):
		t.Fatalf("no %s", what)
	}
}

func (s signal) expectNone(t *testing.T, what string) {
	t.Helper()
	select {
	case <-s:
		t.Fatalf("an unprompted %s", what)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDashboardMeasuresGitOnlyOnFullRefreshes(t *testing.T) {
	b := newTestBoard(t)
	b.session(t, "pulls", editing("web", "feat/x"))
	asked, measured := make(signal, 8), make(signal, 8)
	b.api.pulls = func(ctx context.Context, token string, targets []github.BranchTarget) (string, []github.BranchResult, error) {
		defer func() { asked <- struct{}{} }()
		return b.gh.pulls(ctx, token, targets)
	}
	b.api.measure = func(context.Context, string) []status.RepoStat {
		measured <- struct{}{}
		return []status.RepoStat{}
	}

	b.start(t.Context())
	asked.expect(t, "startup refresh")
	measured.expect(t, "startup measurement")

	b.Request()
	asked.expect(t, "requested refresh")
	measured.expectNone(t, "measurement on a request")

	b.Refresh()
	asked.expect(t, "Refresh")
	measured.expect(t, "measurement on Refresh")
}

func TestASlowMeasurementDoesNotHoldBackPullRequestRows(t *testing.T) {
	b := newTestBoard(t)
	b.session(t, "pulls", editing("web", "feat/x"))
	b.gh.say(boardAnswer{prs: map[string][]github.PullRequest{"web": {pr(1, "a")}}})
	release := make(chan struct{})
	var measuring atomic.Bool
	stat := status.RepoStat{Name: "acme/web", Role: "editing", Commits: 5}
	b.api.measure = func(ctx context.Context, _ string) []status.RepoStat {
		measuring.Store(true)
		select {
		case <-release:
		case <-ctx.Done():
		}
		measuring.Store(false)
		return []status.RepoStat{stat}
	}

	b.start(t.Context())
	b.waitFor(t, "rows while git is still measuring", func(s DashboardStatus) bool {
		return !s.Refreshing && len(s.Sessions["pulls"].PullRequests) == 1 && measuring.Load()
	})
	close(release)
	b.waitFor(t, "the measurement", func(s DashboardStatus) bool {
		return reflect.DeepEqual(s.Sessions["pulls"].Repos, []status.RepoStat{stat})
	})
}

// The joins rule: requests during a refresh run once after it, never once each.
func TestARequestDuringARefreshRunsExactlyOneMoreAfterIt(t *testing.T) {
	b := newTestBoard(t)
	b.session(t, "pulls", editing("web", "feat/x"))
	asked, release := make(signal, 8), make(chan struct{})
	b.api.pulls = func(ctx context.Context, token string, targets []github.BranchTarget) (string, []github.BranchResult, error) {
		asked <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return b.gh.pulls(ctx, token, targets)
	}

	b.start(t.Context())
	asked.expect(t, "startup refresh")
	for range 3 {
		b.Request()
	}
	asked.expectNone(t, "refresh beside the one running")
	release <- struct{}{}
	asked.expect(t, "refresh after the one the requests arrived during")
	release <- struct{}{}
	asked.expectNone(t, "third refresh")
}

func TestARefreshDuringAMeasurementMeasuresOnceMoreAfterIt(t *testing.T) {
	b := newTestBoard(t)
	b.session(t, "pulls", editing("web", "feat/x"))
	asked, started, release := make(signal, 8), make(signal, 8), make(chan struct{}, 8)
	b.api.pulls = func(ctx context.Context, token string, targets []github.BranchTarget) (string, []github.BranchResult, error) {
		defer func() { asked <- struct{}{} }()
		return b.gh.pulls(ctx, token, targets)
	}
	b.api.measure = func(ctx context.Context, _ string) []status.RepoStat {
		started <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
		return []status.RepoStat{}
	}

	b.start(t.Context())
	asked.expect(t, "startup refresh")
	started.expect(t, "startup measurement")
	b.Refresh()
	asked.expect(t, "Refresh")
	started.expectNone(t, "measurement beside the one running")
	release <- struct{}{}
	started.expect(t, "measurement after the one Refresh arrived during")
	release <- struct{}{}
	started.expectNone(t, "third measurement")
}

// Catches stop returning while a refresh can still write pulls.json, which after a
// test's env restore is the user's own.
func TestDashboardStopWaitsForTheRefreshInFlight(t *testing.T) {
	b := newTestBoard(t)
	b.session(t, "pulls", editing("web", "feat/x"))
	started := make(signal, 1)
	var finished atomic.Bool
	b.api.pulls = func(ctx context.Context, token string, targets []github.BranchTarget) (string, []github.BranchResult, error) {
		started <- struct{}{}
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		finished.Store(true)
		return b.gh.pulls(ctx, token, targets)
	}

	b.start(context.Background())
	started.expect(t, "refresh")
	b.stop()

	if !finished.Load() {
		t.Fatal("the refresh was still running after stop")
	}
	if _, err := os.Stat(b.cachePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a cancelled refresh wrote pulls.json: %v", err)
	}
}
