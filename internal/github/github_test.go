package github

import (
	"context"
	"encoding/json"
	"github.com/kieranajp/qrouton/internal/config"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRefreshReposMergesPartialResultsAndRetainsFailedOwnerCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/users/good": `{"login":"good","type":"Organization"}`,
		"/orgs/good/repos?type=all&per_page=100&page=1": `[{"name":"fresh","pushed_at":"2026-02-01T00:00:00Z"}]`,
		// /users/bad intentionally returns 404.
	}, &paths)
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	cached := []Repo{
		{Org: "good", Name: "stale", PushedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Org: "bad", Name: "cached", PushedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	var complete RefreshMsg
	states := map[string][]RefreshState{}
	for msg := range RefreshRepos(context.Background(), client, "token", []string{"good", "bad"}, cached) {
		states[msg.Owner] = append(states[msg.Owner], msg.State)
		if msg.State == RefreshComplete {
			complete = msg
		}
	}
	if !reflect.DeepEqual(states["good"], []RefreshState{RefreshStarted, RefreshSucceeded}) {
		t.Fatalf("good states = %#v", states["good"])
	}
	if !reflect.DeepEqual(states["bad"], []RefreshState{RefreshStarted, RefreshFailed}) {
		t.Fatalf("bad states = %#v", states["bad"])
	}
	got := []string{complete.Repos[0].ID(), complete.Repos[1].ID()}
	if want := []string{"good/fresh", "bad/cached"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged repos = %#v, want %#v", got, want)
	}
}

func TestRefreshOwnerReposCanBeRetriedIndependently(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		status, body := http.StatusOK, `{"login":"owner","type":"Organization"}`
		if requests == 1 {
			status, body = http.StatusServiceUnavailable, `{}`
		} else if strings.Contains(r.URL.Path, "/repos") {
			body = `[{"name":"recovered"}]`
		}
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	if _, err := RefreshOwnerRepos(context.Background(), client, "token", "owner"); err == nil {
		t.Fatal("first refresh unexpectedly succeeded")
	}
	repos, err := RefreshOwnerRepos(context.Background(), client, "token", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].ID() != "owner/recovered" {
		t.Fatalf("retry repos = %#v", repos)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type requestPaths struct {
	mu    sync.Mutex
	paths []string
}

func (p *requestPaths) append(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = append(p.paths, path)
}

func (p *requestPaths) snapshot() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.paths...)
}

func githubTestClient(t *testing.T, responses map[string]string, paths *requestPaths) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		uri := r.URL.RequestURI()
		paths.append(uri)
		body, ok := responses[uri]
		status := http.StatusOK
		if !ok {
			status, body = http.StatusNotFound, `{"message":"Not Found"}`
		}
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
}

func TestSortReposByActivityNewestPushFirst(t *testing.T) {
	oldest := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newest := oldest.Add(24 * time.Hour)
	repos := []Repo{
		{Org: "b", Name: "old", PushedAt: oldest},
		{Org: "z", Name: "new", PushedAt: newest},
		{Org: "a", Name: "also-new", PushedAt: newest},
	}
	SortReposByActivity(repos)
	got := []string{repos[0].ID(), repos[1].ID(), repos[2].ID()}
	want := []string{"a/also-new", "z/new", "b/old"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestFetchOwnerReposUsesAuthenticatedEndpointForPersonalOwner(t *testing.T) {
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/users/kieranajp": `{"login":"kieranajp","type":"User"}`,
		"/user":            `{"login":"kieranajp"}`,
		"/user/repos?affiliation=owner&visibility=all&per_page=100&page=1": `[{"name":"private-repo","ssh_url":"git@example/repo","default_branch":"main"}]`,
	}, &paths)

	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })
	login := ""
	repos, err := fetchOwnerRepos(context.Background(), client, "token", "kieranajp", &login)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Org != "kieranajp" || repos[0].Name != "private-repo" {
		t.Fatalf("unexpected repos: %#v", repos)
	}
	wantPaths := []string{
		"/users/kieranajp",
		"/user",
		"/user/repos?affiliation=owner&visibility=all&per_page=100&page=1",
	}
	gotPaths := paths.snapshot()
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("requests = %#v, want %#v", gotPaths, wantPaths)
	}
}

func TestFetchOwnerReposUsesOrganizationEndpoint(t *testing.T) {
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/users/lifesum": `{"login":"lifesum","type":"Organization"}`,
		"/orgs/lifesum/repos?type=all&per_page=100&page=1": `[]`,
	}, &paths)

	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })
	login := ""
	if _, err := fetchOwnerRepos(context.Background(), client, "token", "lifesum", &login); err != nil {
		t.Fatal(err)
	}
	want := []string{"/users/lifesum", "/orgs/lifesum/repos?type=all&per_page=100&page=1"}
	gotPaths := paths.snapshot()
	if !reflect.DeepEqual(gotPaths, want) {
		t.Fatalf("requests = %#v, want %#v", gotPaths, want)
	}
}

func TestRepoIDIncludesOrganization(t *testing.T) {
	if got := (Repo{Org: "acme", Name: "api"}).ID(); got != "acme/api" {
		t.Fatalf("Repo.ID() = %q", got)
	}
}

func TestAuthenticatedLoginNamesTheAccountTheTokenBelongsTo(t *testing.T) {
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/user": `{"login":"kieranajp"}`,
	}, &paths)
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	login, err := AuthenticatedLogin(context.Background(), client, "token")
	if err != nil {
		t.Fatal(err)
	}
	if login != "kieranajp" {
		t.Fatalf("login = %q, want %q", login, "kieranajp")
	}
}

func TestAuthenticatedLoginReportsAFailedLookup(t *testing.T) {
	var paths requestPaths
	client := githubTestClient(t, map[string]string{}, &paths)
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	if _, err := AuthenticatedLogin(context.Background(), client, "token"); err == nil {
		t.Fatal("AuthenticatedLogin invented a login from a failed lookup")
	}
}

// Owner discovery resolves a personal owner through the same lookup, so the
// shared implementation must still answer it.
func TestRefreshOwnerReposStillResolvesTheAuthenticatedUsersOwnRepos(t *testing.T) {
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/users/kieranajp": `{"login":"kieranajp","type":"User"}`,
		"/user":            `{"login":"kieranajp"}`,
		"/user/repos?affiliation=owner&visibility=all&per_page=100&page=1": `[{"name":"qrouton"}]`,
	}, &paths)
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	repos, err := RefreshOwnerRepos(context.Background(), client, "token", "kieranajp")
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Name != "qrouton" {
		t.Fatalf("repos = %#v, want the authenticated user's own", repos)
	}
}

func TestRefreshReposDropsOwnersNoLongerConfigured(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/users/kept": `{"login":"kept","type":"Organization"}`,
		"/orgs/kept/repos?type=all&per_page=100&page=1": `[{"name":"fresh","pushed_at":"2026-02-01T00:00:00Z"}]`,
	}, &paths)
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	cached := []Repo{{Org: "gone", Name: "orphan", PushedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
	var complete RefreshMsg
	for msg := range RefreshRepos(context.Background(), client, "token", []string{"kept"}, cached) {
		if msg.State == RefreshComplete {
			complete = msg
		}
	}
	if got := repoIDs(complete.Repos); !reflect.DeepEqual(got, []string{"kept/fresh"}) {
		t.Fatalf("repos = %#v, want the configured owner's alone", got)
	}
	if got, _, ok := CachedRepos([]string{"kept"}); !ok || !reflect.DeepEqual(repoIDs(got), []string{"kept/fresh"}) {
		t.Fatalf("cache = %#v (found %v), want the configured owner's alone", repoIDs(got), ok)
	}
}

func repoIDs(repos []Repo) []string {
	out := make([]string, 0, len(repos))
	for _, repo := range repos {
		out = append(out, repo.ID())
	}
	return out
}
func seedRepoCache(t *testing.T, orgs []string, owners map[string]OwnerRepos) {
	t.Helper()
	b, err := json.Marshal(repoCache{SchemaVersion: cacheSchemaVersion, Orgs: orgs, Owners: owners})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(config.CachePath()), cacheDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.CachePath(), b, cacheFileMode); err != nil {
		t.Fatal(err)
	}
}

// An expired token fails every owner, and the list the picker draws is then
// exactly the one it already had. Re-stamping it as fetched now would tell the
// user a week-old list is current.
func TestRefreshReposLeavesEveryTimestampAloneWhenEveryOwnerFails(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var paths requestPaths
	client := githubTestClient(t, nil, &paths) // every owner 404s
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	orgs := []string{"acme", "globex"}
	fetched := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	seedRepoCache(t, orgs, map[string]OwnerRepos{
		"acme":   {FetchedAt: fetched, Repos: []Repo{{Org: "acme", Name: "api"}}},
		"globex": {FetchedAt: fetched, Repos: []Repo{{Org: "globex", Name: "web"}}},
	})
	cached, _, _ := CachedRepos(orgs)
	for range RefreshRepos(context.Background(), client, "token", orgs, cached) {
	}

	owners, ok := CachedOwnerRepos(orgs)
	if !ok {
		t.Fatal("a wholly failed refresh discarded the cache")
	}
	for _, owner := range orgs {
		if got := owners[owner].FetchedAt; !got.Equal(fetched) {
			t.Fatalf("%s fetched at %s, want the cache left at %s", owner, got, fetched)
		}
	}
	if _, at, _ := CachedRepos(orgs); !at.Equal(fetched) {
		t.Fatalf("aggregate fetched at %s, want %s", at, fetched)
	}
}

func TestRefreshReposAdvancesOnlyTheOwnersThatFetched(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/users/good": `{"login":"good","type":"Organization"}`,
		"/orgs/good/repos?type=all&per_page=100&page=1": `[{"name":"fresh","pushed_at":"2026-02-01T00:00:00Z"}]`,
		// /users/bad intentionally returns 404.
	}, &paths)
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	orgs := []string{"good", "bad"}
	fetched := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	seedRepoCache(t, orgs, map[string]OwnerRepos{
		"good": {FetchedAt: fetched, Repos: []Repo{{Org: "good", Name: "stale"}}},
		"bad":  {FetchedAt: fetched, Repos: []Repo{{Org: "bad", Name: "cached"}}},
	})
	cached, _, _ := CachedRepos(orgs)
	started := time.Now()
	for range RefreshRepos(context.Background(), client, "token", orgs, cached) {
	}

	owners, ok := CachedOwnerRepos(orgs)
	if !ok {
		t.Fatal("a partly failed refresh discarded the cache")
	}
	if owners["good"].FetchedAt.Before(started) {
		t.Fatalf("good fetched at %s, want a successful refetch stamped after %s", owners["good"].FetchedAt, started)
	}
	if got := repoIDs(owners["good"].Repos); !reflect.DeepEqual(got, []string{"good/fresh"}) {
		t.Fatalf("good repos = %#v", got)
	}
	if got := owners["bad"].FetchedAt; !got.Equal(fetched) {
		t.Fatalf("bad fetched at %s, want the cache left at %s", got, fetched)
	}
	if got := repoIDs(owners["bad"].Repos); !reflect.DeepEqual(got, []string{"bad/cached"}) {
		t.Fatalf("bad repos = %#v", got)
	}
	if _, at, _ := CachedRepos(orgs); !at.Equal(fetched) {
		t.Fatalf("aggregate fetched at %s, want the oldest owner's %s", at, fetched)
	}
}

func TestCachedReposReadsACachePredatingPerOwnerTimestampsAsAbsent(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	orgs := []string{"acme"}
	if err := os.MkdirAll(filepath.Dir(config.CachePath()), cacheDirMode); err != nil {
		t.Fatal(err)
	}
	flat := `{"schemaVersion":2,"fetchedAt":"2026-01-02T03:04:05Z","orgs":["acme"],"repos":[{"name":"api","org":"acme"}]}`
	if err := os.WriteFile(config.CachePath(), []byte(flat), cacheFileMode); err != nil {
		t.Fatal(err)
	}
	repos, at, ok := CachedRepos(orgs)
	if ok || repos != nil || !at.IsZero() {
		t.Fatalf("CachedRepos = %#v, %s, %v; want a flat cache to read as absent", repos, at, ok)
	}
}

func TestWriteRepoCacheStampsEveryListedOwner(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	orgs := []string{"acme", "globex"}
	started := time.Now()
	if err := WriteRepoCache(orgs, []Repo{{Org: "acme", Name: "api"}}); err != nil {
		t.Fatal(err)
	}
	owners, ok := CachedOwnerRepos(orgs)
	if !ok {
		t.Fatal("WriteRepoCache wrote nothing readable")
	}
	for _, owner := range orgs {
		if owners[owner].FetchedAt.Before(started) {
			t.Fatalf("%s fetched at %s, want a stamp after %s", owner, owners[owner].FetchedAt, started)
		}
	}
	if got := repoIDs(owners["acme"].Repos); !reflect.DeepEqual(got, []string{"acme/api"}) {
		t.Fatalf("acme repos = %#v", got)
	}
	if len(owners["globex"].Repos) != 0 {
		t.Fatalf("globex repos = %#v, want an owner with nothing to be empty", owners["globex"].Repos)
	}
}
