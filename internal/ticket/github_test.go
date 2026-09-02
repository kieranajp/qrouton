package ticket

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	gh "github.com/kieranajp/qrouton/internal/github"
)

func stubGitHubToken(t *testing.T, token string, err error) {
	t.Helper()
	previous := githubToken
	githubToken = func() (string, error) { return token, err }
	t.Cleanup(func() { githubToken = previous })
}

func TestGitHubIssueURLsCanonicalizeWithTheOwnerAndRepoLowercased(t *testing.T) {
	want := "https://github.com/acme/api/issues/42"
	for _, raw := range []string{
		"https://github.com/acme/api/issues/42",
		"https://github.com/Acme/API/issues/42",
		"  https://GITHUB.COM/acme/api/issues/42?utm=x#issuecomment-1  ",
	} {
		t.Run(raw, func(t *testing.T) {
			got, err := Canonical(raw)
			if err != nil || got != want {
				t.Fatalf("Canonical(%q) = %q, %v; want %q", raw, got, err, want)
			}
		})
	}
}

func TestGitHubKeySeedsFromTheRepositoryAndIssueNumber(t *testing.T) {
	if got := Key("https://github.com/Acme/API/issues/42"); got != "api-42" {
		t.Fatalf("Key() = %q, want %q", got, "api-42")
	}
}

func TestGitHubReferencesRejectAnythingOutsideTheIssueContract(t *testing.T) {
	oversized := "https://github.com/acme/api/issues/42?q=" + strings.Repeat("a", githubMaxReferenceBytes)
	// A host no provider claims is a different fix from a github.com URL of the
	// wrong shape, and the hint the user reads says so.
	for _, raw := range []string{
		"",
		"acme/api#42",
		"github.com/acme/api/issues/42",
		"http://github.com/acme/api/issues/42",
		"https://github.acme.com/acme/api/issues/42",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Canonical(raw); !errors.Is(err, ErrUnsupportedProvider) {
				t.Fatalf("Canonical(%q) = %v, want %v", raw, err, ErrUnsupportedProvider)
			}
		})
	}
	for _, raw := range []string{
		"https://user@github.com/acme/api/issues/42",
		"https://github.com:443/acme/api/issues/42",
		"https://github.com/acme/api/pull/42",
		"https://github.com/acme/api/issues",
		"https://github.com/acme/api/issues/0",
		"https://github.com/acme/api/issues/01",
		"https://github.com/acme/api/issues/42/comments",
		"https://github.com/acme/api/issues/４２",
		"https://github.com/acme/api/issues/42;open /tmp/pwned",
		"https://github.com//api/issues/42",
		"https://github.com/acme//issues/42",
		"https://github.com/acme/api/issues/42/../43",
		"https://github.com/acme/ap%69/issues/42",
		"https://github.com/./api/issues/42",
		"https://github.com/../api/issues/42",
		oversized,
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Canonical(raw); !errors.Is(err, ErrNotGitHubIssue) {
				t.Fatalf("Canonical(%q) = %v, want %v", raw, err, ErrNotGitHubIssue)
			}
		})
	}
}

func TestFetchGitHubTicket(t *testing.T) {
	stubGitHubToken(t, "github-token", nil)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/api/issues/42" ||
			r.Header.Get("Authorization") != "Bearer github-token" {
			t.Fatalf("unexpected request: %s %s, auth %q", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		return response(`{"title":"Fix retries","body":"Retry failed requests"}`), nil
	})}
	got, err := Fetch(context.Background(), client, "https://github.com/Acme/API/issues/42")
	if err != nil || got.Title != "Fix retries" || got.Body != "Retry failed requests" {
		t.Fatalf("Fetch() = %#v, %v", got, err)
	}
}

func TestFetchGitHubReportsAPrivateOrMissingIssueAsNotFound(t *testing.T) {
	stubGitHubToken(t, "github-token", nil)
	_, err := Fetch(context.Background(), failingClient(http.StatusNotFound, "404 Not Found"),
		"https://github.com/acme/api/issues/42")
	if err == nil || err.Error() != "github: ticket not found" {
		t.Fatalf("Fetch() = %v", err)
	}
}

func TestFetchGitHubKeepsTheGenericMessageForEveryOtherStatus(t *testing.T) {
	stubGitHubToken(t, "github-token", nil)
	want := "github: loading ticket: request failed: 500 Internal Server Error"
	_, err := Fetch(context.Background(), failingClient(http.StatusInternalServerError, "500 Internal Server Error"),
		"https://github.com/acme/api/issues/42")
	if err == nil || err.Error() != want {
		t.Fatalf("Fetch() = %v, want %q", err, want)
	}
}

func TestFetchGitHubWithoutACredentialAsksForOne(t *testing.T) {
	stubGitHubToken(t, "", gh.ErrNoToken)
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("request issued without a credential")
		return nil, nil
	})}
	if _, err := Fetch(context.Background(), client, "https://github.com/acme/api/issues/42"); err != gh.ErrNoToken {
		t.Fatalf("Fetch() = %v, want %v", err, gh.ErrNoToken)
	}
}
