package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func pullsAnswer(t *testing.T, body string, sent *graphQLRequest) *http.Client {
	t.Helper()
	return graphQLServer(t, func(w http.ResponseWriter, r *http.Request) {
		if sent != nil {
			_ = json.NewDecoder(r.Body).Decode(sent)
		}
		_, _ = w.Write([]byte(body))
	})
}

const ownNode = `{"number":7,"title":"Add pulls","url":"https://github.com/acme/web/pull/7","state":"OPEN","isDraft":false,` +
	`"updatedAt":"2026-11-19T10:00:00Z","author":{"login":"octo"},"latestOpinionatedReviews":{"nodes":[{"state":"CHANGES_REQUESTED"},{"state":"APPROVED"}]},` +
	`"headRepositoryOwner":{"login":"ACME"},"statusCheckRollup":{"state":"FAILURE"}}`

func TestBranchPullRequestsPassesTargetsOnlyAsVariables(t *testing.T) {
	hostile := `x") { evil }`
	var sent graphQLRequest
	client := pullsAnswer(t, `{"data":{"viewer":{"login":"octo"},"r0":{"pullRequests":{"nodes":[]}},"r1":{"pullRequests":{"nodes":[]}}}}`, &sent)

	targets := []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat/pulls"}, {Owner: "evil-org", Name: "evil-repo", Branch: hostile}}
	if _, _, err := BranchPullRequests(context.Background(), client, "token", targets); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{hostile, "evil", "acme", "web", "feat/pulls"} {
		if strings.Contains(sent.Query, s) {
			t.Fatalf("query text carries %q: %s", s, sent.Query)
		}
	}
	want := map[string]any{"o0": "acme", "n0": "web", "b0": "feat/pulls", "o1": "evil-org", "n1": "evil-repo", "b1": hostile}
	if !reflect.DeepEqual(sent.Variables, want) {
		t.Fatalf("variables = %#v", sent.Variables)
	}
	for _, s := range []string{"$o1: String!", "r1: repository(owner: $o1, name: $n1)", "headRefName: $b1", "first: 50", "viewer { login }", "latestOpinionatedReviews(first: 20, writersOnly: true) { nodes { state } }"} {
		if !strings.Contains(sent.Query, s) {
			t.Fatalf("query lacks %q: %s", s, sent.Query)
		}
	}
}

// A field the flattening missed would draw every row untagged.
func TestBranchPullRequestsFlattensAuthorReviewChecksAndUpdatedAt(t *testing.T) {
	client := pullsAnswer(t, `{"data":{"viewer":{"login":"octo"},"r0":{"pullRequests":{"nodes":[`+ownNode+`,`+
		`{"number":8,"title":"Ghost","url":"u8","state":"MERGED","isDraft":true,"updatedAt":"2026-11-18T09:30:00Z",`+
		`"author":null,"latestOpinionatedReviews":null,"headRepositoryOwner":{"login":"acme"},"statusCheckRollup":null}]}}}}`, nil)

	viewer, results, err := BranchPullRequests(context.Background(), client, "token", []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}})
	if err != nil {
		t.Fatal(err)
	}
	if viewer != "octo" {
		t.Fatalf("viewer = %q", viewer)
	}
	want := []BranchResult{{PullRequests: []PullRequest{
		{Number: 7, Title: "Add pulls", URL: "https://github.com/acme/web/pull/7", State: PullStateOpen,
			UpdatedAt: time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC), Author: "octo", Reviews: []string{ReviewChangesRequested, ReviewApproved}, CheckState: CheckFailure},
		{Number: 8, Title: "Ghost", URL: "u8", State: PullStateMerged, IsDraft: true,
			UpdatedAt: time.Date(2026, 11, 18, 9, 30, 0, 0, time.UTC)},
	}}}
	if !reflect.DeepEqual(results, want) {
		t.Fatalf("results = %#v", results)
	}
}

// headRefName matches any head repository, so a fork's same-named branch would otherwise show.
func TestBranchPullRequestsDropsAForksPullRequest(t *testing.T) {
	client := pullsAnswer(t, `{"data":{"viewer":{"login":"octo"},"r0":{"pullRequests":{"nodes":[`+ownNode+`,`+
		`{"number":9,"state":"OPEN","headRepositoryOwner":{"login":"forker"}},`+
		`{"number":10,"state":"OPEN","headRepositoryOwner":null}]}}}}`, nil)

	_, results, err := BranchPullRequests(context.Background(), client, "token", []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || len(results[0].PullRequests) != 1 || results[0].PullRequests[0].Number != 7 {
		t.Fatalf("results = %#v", results)
	}
}

func TestBranchPullRequestsNotFoundEmptiesOnlyItsOwnTarget(t *testing.T) {
	client := pullsAnswer(t, `{"data":{"viewer":{"login":"octo"},"r0":null,"r1":{"pullRequests":{"nodes":[`+ownNode+`]}}},`+
		`"errors":[{"type":"NOT_FOUND","path":["r0"],"message":"Could not resolve to a Repository with the name 'acme/gone'."}]}`, nil)

	_, results, err := BranchPullRequests(context.Background(), client, "token",
		[]BranchTarget{{Owner: "acme", Name: "gone", Branch: "feat"}, {Owner: "acme", Name: "web", Branch: "feat"}})
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(results[0].Err, ErrNotFound) || results[0].PullRequests != nil {
		t.Fatalf("r0 = %#v", results[0])
	}
	if results[1].Err != nil || len(results[1].PullRequests) != 1 {
		t.Fatalf("r1 = %#v", results[1])
	}
}

func TestBranchPullRequestsOtherAliasErrorStaysOnItsAlias(t *testing.T) {
	client := pullsAnswer(t, `{"data":{"viewer":{"login":"octo"},"r0":{"pullRequests":{"nodes":[`+ownNode+`]}},"r1":null},`+
		`"errors":[{"type":"FORBIDDEN","path":["r1","pullRequests"],"message":"Resource not accessible by integration"}]}`, nil)

	viewer, results, err := BranchPullRequests(context.Background(), client, "token",
		[]BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}, {Owner: "acme", Name: "secret", Branch: "feat"}})
	if err != nil {
		t.Fatal(err)
	}
	if viewer != "octo" || results[0].Err != nil || len(results[0].PullRequests) != 1 {
		t.Fatalf("r0 = %#v, viewer %q", results[0], viewer)
	}
	if results[1].Err == nil || errors.Is(results[1].Err, ErrNotFound) || !strings.Contains(results[1].Err.Error(), "Resource not accessible by integration") {
		t.Fatalf("r1 err = %v", results[1].Err)
	}
}

func TestBranchPullRequestsErrorWithNoAliasPathFailsTheWholeCall(t *testing.T) {
	for _, path := range []string{`[]`, `["viewer"]`, `["r7"]`, `null`} {
		client := pullsAnswer(t, `{"data":null,"errors":[{"type":"INTERNAL","path":`+path+`,"message":"boom"}]}`, nil)
		_, results, err := BranchPullRequests(context.Background(), client, "token", []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}})
		if err == nil || results != nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("path %s: results = %#v, err = %v", path, results, err)
		}
	}
}

func TestBranchPullRequestsPassesRateLimitsAndTokenRejectionsThrough(t *testing.T) {
	client := graphQLServer(t, answer(http.StatusOK, map[string]string{"x-ratelimit-remaining": "0"}, `{"data":null}`))
	// Results here would hold an empty list per target and wipe every cached row.
	viewer, results, err := BranchPullRequests(context.Background(), client, "token", []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}})
	rateLimitUntil(t, err)
	if viewer != "" || results != nil {
		t.Fatalf("a spent budget with no data answered viewer %q, results %#v", viewer, results)
	}

	client = graphQLServer(t, answer(http.StatusUnauthorized, nil, `{}`))
	if _, _, err := BranchPullRequests(context.Background(), client, "token", []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}}); !errors.Is(err, ErrTokenRejected) {
		t.Fatalf("err = %v", err)
	}
}

func TestBranchPullRequestsWithNoTargetsMakesNoRequest(t *testing.T) {
	client := graphQLServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("a request was made") })
	viewer, results, err := BranchPullRequests(context.Background(), client, "token", nil)
	if err != nil || viewer != "" || results == nil || len(results) != 0 {
		t.Fatalf("viewer %q, results %#v, err %v", viewer, results, err)
	}
}

func TestBranchPullRequestsEmptyBranchIsAnEmptyNonNilList(t *testing.T) {
	client := pullsAnswer(t, `{"data":{"viewer":{"login":"octo"},"r0":{"pullRequests":{"nodes":[]}}}}`, nil)
	_, results, err := BranchPullRequests(context.Background(), client, "token", []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}})
	if err != nil || results[0].Err != nil || results[0].PullRequests == nil || len(results[0].PullRequests) != 0 {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
}

// The response that spends the last point still answers every target, so its rows must reach the caller.
func TestBranchPullRequestsReturnsTheRowsOfAResponseThatSpendsTheLastPoint(t *testing.T) {
	client := graphQLServer(t, answer(http.StatusOK, map[string]string{"x-ratelimit-remaining": "0"},
		`{"data":{"viewer":{"login":"octo"},"r0":{"pullRequests":{"nodes":[`+ownNode+`]}}}}`))
	viewer, results, err := BranchPullRequests(context.Background(), client, "token", []BranchTarget{{Owner: "acme", Name: "web", Branch: "feat"}})
	rateLimitUntil(t, err)
	if viewer != "octo" || len(results) != 1 || results[0].Err != nil || len(results[0].PullRequests) != 1 {
		t.Fatalf("viewer %q, results %#v", viewer, results)
	}
}
