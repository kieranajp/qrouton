package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
)

func noPulls(_ context.Context, _ string, previous sessionPulls) sessionPulls { return previous }

type pullAnswer struct {
	prs         []github.PullRequest
	etag        string
	notModified bool
	err         error
}

type pullCall struct{ owner, repo, branch, etag string }

// fakePullAPI answers each repository by name and records what it was asked.
type fakePullAPI struct {
	token   string
	answers map[string]pullAnswer
	calls   []pullCall
}

func (f *fakePullAPI) api() pullRequestAPI {
	return pullRequestAPI{
		credential: func(context.Context) (github.Credential, string) {
			if f.token == "" {
				return github.CredentialSignedOut, ""
			}
			return github.CredentialCLI, f.token
		},
		list: func(_ context.Context, token, owner, repo, branch, etag string) ([]github.PullRequest, string, bool, error) {
			f.calls = append(f.calls, pullCall{owner, repo, branch, etag})
			a := f.answers[repo]
			return a.prs, a.etag, a.notModified, a.err
		},
	}
}

func pullSession(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	m := session.Manifest{Slug: "pulls", Repos: []session.ManifestRepo{
		{Org: "acme", Name: "web", Role: session.RepoRoleEditing, Branch: "feat/x", WorktreePath: "src/web"},
		{Org: "acme", Name: "docs", Role: session.RepoRoleReference, Branch: "feat/x", WorktreePath: "src/docs"},
		{Org: "acme", Name: "unbranched", Role: session.RepoRoleEditing, WorktreePath: "src/unbranched"},
		{Org: "acme", Name: "api", Role: session.RepoRoleEditing, Branch: "feat/x", WorktreePath: "src/api"},
	}}
	if err := session.WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	return dir
}

func pr(n int, title string) github.PullRequest {
	return github.PullRequest{Number: n, Title: title, URL: "https://github.com/acme/x/pull/" + title}
}

func titles(list status.PullRequestList) []string {
	out := []string{}
	for _, item := range list.Items {
		out = append(out, item.Repo+"#"+item.Title)
	}
	return out
}

func TestPullFetchAsksOnlyEditingReposWithABranchInManifestOrder(t *testing.T) {
	dir := pullSession(t)
	fake := &fakePullAPI{token: "t", answers: map[string]pullAnswer{
		"web": {prs: []github.PullRequest{pr(1, "a"), pr(2, "b")}, etag: "w1"},
		"api": {prs: []github.PullRequest{pr(3, "c")}, etag: "a1"},
	}}

	got := fake.api().fetch(context.Background(), dir, sessionPulls{})

	want := []pullCall{{"acme", "web", "feat/x", ""}, {"acme", "api", "feat/x", ""}}
	if !reflect.DeepEqual(fake.calls, want) {
		t.Fatalf("calls = %+v, want %+v", fake.calls, want)
	}
	if rows := titles(got.list()); !reflect.DeepEqual(rows, []string{"acme/web#a", "acme/web#b", "acme/api#c"}) {
		t.Fatalf("rows = %v", rows)
	}
	if item := got.list().Items[0]; item.Number != 1 || item.URL != "https://github.com/acme/x/pull/a" {
		t.Fatalf("first row = %+v", item)
	}
}

func TestPullFetchKeepsRowsOnNotModifiedAndSendsTheLastETag(t *testing.T) {
	dir := pullSession(t)
	fake := &fakePullAPI{token: "t", answers: map[string]pullAnswer{
		"web": {prs: []github.PullRequest{pr(1, "a")}, etag: "w1"},
	}}
	first := fake.api().fetch(context.Background(), dir, sessionPulls{})

	fake.calls = nil
	fake.answers = map[string]pullAnswer{"web": {notModified: true, etag: "w1"}, "api": {notModified: true}}
	second := fake.api().fetch(context.Background(), dir, first)

	if fake.calls[0].etag != "w1" {
		t.Fatalf("web was asked with etag %q", fake.calls[0].etag)
	}
	if rows := titles(second.list()); !reflect.DeepEqual(rows, []string{"acme/web#a"}) {
		t.Fatalf("rows after 304 = %v", rows)
	}
}

func TestPullFetchWithoutACredentialIsSignedOut(t *testing.T) {
	dir := pullSession(t)
	fake := &fakePullAPI{}
	got := fake.api().fetch(context.Background(), dir, sessionPulls{})
	if !got.list().SignedOut || len(fake.calls) != 0 {
		t.Fatalf("list = %+v, calls = %+v", got.list(), fake.calls)
	}
}

func TestPullFetchWithARejectedTokenIsSignedOut(t *testing.T) {
	dir := pullSession(t)
	fake := &fakePullAPI{token: "t", answers: map[string]pullAnswer{
		"web": {err: fmt.Errorf("listing: %w", github.ErrTokenRejected)},
	}}
	if got := fake.api().fetch(context.Background(), dir, sessionPulls{}); !got.list().SignedOut {
		t.Fatalf("list = %+v", got.list())
	}
}

func TestPullFetchKeepsTheLastRowsOnServerErrors(t *testing.T) {
	dir := pullSession(t)
	fake := &fakePullAPI{token: "t", answers: map[string]pullAnswer{
		"web": {prs: []github.PullRequest{pr(1, "a")}, etag: "w1"},
	}}
	first := fake.api().fetch(context.Background(), dir, sessionPulls{})

	fake.answers = map[string]pullAnswer{"web": {err: errors.New("502 Bad Gateway")}}
	second := fake.api().fetch(context.Background(), dir, first)

	if rows := titles(second.list()); !reflect.DeepEqual(rows, []string{"acme/web#a"}) || second.list().SignedOut {
		t.Fatalf("rows after 5xx = %v, signed out %v", rows, second.list().SignedOut)
	}
}

func TestPullFetchLogsARepeatedFailureOnce(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := pullSession(t)
	fake := &fakePullAPI{token: "t", answers: map[string]pullAnswer{"web": {err: errors.New("502 Bad Gateway")}}}

	pulls := sessionPulls{}
	for range 3 {
		pulls = fake.api().fetch(context.Background(), dir, pulls)
	}

	if got := strings.Count(logged.String(), "502 Bad Gateway"); got != 1 {
		t.Fatalf("logged the failure %d times:\n%s", got, logged.String())
	}
}

func TestPullFetchDropsOnlyTheRepositoryGitHubCannotFind(t *testing.T) {
	dir := pullSession(t)
	fake := &fakePullAPI{token: "t", answers: map[string]pullAnswer{
		"web": {prs: []github.PullRequest{pr(1, "a")}, etag: "w1"},
		"api": {prs: []github.PullRequest{pr(3, "c")}, etag: "a1"},
	}}
	first := fake.api().fetch(context.Background(), dir, sessionPulls{})

	fake.answers = map[string]pullAnswer{
		"web": {err: github.ErrNotFound},
		"api": {notModified: true, etag: "a1"},
	}
	second := fake.api().fetch(context.Background(), dir, first)

	if rows := titles(second.list()); !reflect.DeepEqual(rows, []string{"acme/api#c"}) || second.list().SignedOut {
		t.Fatalf("rows after 404 = %v, signed out %v", rows, second.list().SignedOut)
	}
}
