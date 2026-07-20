package github

import (
	"context"
	"io"
	"net/http"
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

func TestFetchRepoResolvesSingleRepository(t *testing.T) {
	var paths requestPaths
	client := githubTestClient(t, map[string]string{
		"/repos/KieranAJP/qrouton": `{"name":"qrouton","ssh_url":"git@github.com:kieranajp/qrouton.git","default_branch":"main","pushed_at":"2026-03-02T00:00:00Z","owner":{"login":"kieranajp"}}`,
	}, &paths)
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	repo, err := FetchRepo(context.Background(), client, "token", "KieranAJP", "qrouton")
	if err != nil {
		t.Fatal(err)
	}
	// The canonical owner login from the payload wins over the typed casing.
	if repo.Org != "kieranajp" || repo.Name != "qrouton" || repo.DefaultBranch != "main" {
		t.Fatalf("repo = %+v", repo)
	}
	if repo.SSHURL != "git@github.com:kieranajp/qrouton.git" {
		t.Fatalf("ssh url = %q", repo.SSHURL)
	}
}

func TestFetchRepoReportsMissingRepository(t *testing.T) {
	var paths requestPaths
	client := githubTestClient(t, map[string]string{}, &paths) // everything 404s
	oldBase := githubAPIBase
	githubAPIBase = "https://api.test"
	t.Cleanup(func() { githubAPIBase = oldBase })

	if _, err := FetchRepo(context.Background(), client, "token", "who", "what"); err == nil {
		t.Fatal("expected error for a repository that does not resolve")
	}
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
	repos, err := fetchOwnerRepos(client, "token", "kieranajp", &login)
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
	if _, err := fetchOwnerRepos(client, "token", "lifesum", &login); err != nil {
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
