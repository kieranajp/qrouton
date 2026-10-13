package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func pullsServer(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	oldBase := githubAPIBase
	githubAPIBase = server.URL
	t.Cleanup(func() { githubAPIBase = oldBase })
	return server.Client()
}

func TestListPullRequestsFiltersByHeadBranchInEveryState(t *testing.T) {
	var query, path, auth string
	client := pullsServer(t, func(w http.ResponseWriter, r *http.Request) {
		path, query, auth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		w.Header().Set("ETag", `W/"one"`)
		_, _ = w.Write([]byte(`[{"number":7,"title":"Add pulls","html_url":"https://github.com/acme/web/pull/7"}]`))
	})

	prs, etag, notModified, err := ListPullRequests(context.Background(), client, "token", "acme", "web", "feat/pulls", "")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/repos/acme/web/pulls" {
		t.Fatalf("path = %q", path)
	}
	if want := "head=acme:feat%2Fpulls&state=all&per_page=100"; query != want {
		t.Fatalf("query = %q, want %q", query, want)
	}
	if auth != "Bearer token" {
		t.Fatalf("authorization = %q", auth)
	}
	want := []PullRequest{{Number: 7, Title: "Add pulls", URL: "https://github.com/acme/web/pull/7"}}
	if !reflect.DeepEqual(prs, want) || etag != `W/"one"` || notModified {
		t.Fatalf("got %#v %q %v", prs, etag, notModified)
	}
}

func TestListPullRequestsSendsTheETagAndKeepsItOnNotModified(t *testing.T) {
	var sent string
	client := pullsServer(t, func(w http.ResponseWriter, r *http.Request) {
		sent = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	})

	prs, etag, notModified, err := ListPullRequests(context.Background(), client, "token", "acme", "web", "feat", `W/"one"`)
	if err != nil {
		t.Fatal(err)
	}
	if sent != `W/"one"` {
		t.Fatalf("If-None-Match = %q", sent)
	}
	if !notModified || etag != `W/"one"` || prs != nil {
		t.Fatalf("got %#v %q %v", prs, etag, notModified)
	}
}

func TestListPullRequestsWithNoPullRequestsIsAnEmptyList(t *testing.T) {
	client := pullsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" {
			t.Error("an unconditional request carried If-None-Match")
		}
		_, _ = w.Write([]byte(`[]`))
	})

	prs, _, _, err := ListPullRequests(context.Background(), client, "token", "acme", "web", "feat", "")
	if err != nil || prs == nil || len(prs) != 0 {
		t.Fatalf("got %#v, %v", prs, err)
	}
}

func TestListPullRequestsNamesRejectedTokensAndMissingRepositories(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrTokenRejected},
		{http.StatusNotFound, ErrNotFound},
	} {
		client := pullsServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status) })
		if _, _, _, err := ListPullRequests(context.Background(), client, "token", "acme", "web", "feat", ""); !errors.Is(err, tc.want) {
			t.Fatalf("HTTP %d: err = %v, want %v", tc.status, err, tc.want)
		}
	}
}

func TestListPullRequestsFailsOnServerErrors(t *testing.T) {
	client := pullsServer(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	_, _, _, err := ListPullRequests(context.Background(), client, "token", "acme", "web", "feat", "")
	if err == nil || errors.Is(err, ErrTokenRejected) || errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
