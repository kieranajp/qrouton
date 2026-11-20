package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

var graphQLNow = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

func graphQLServer(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	oldBase, oldClock := githubAPIBase, clock
	githubAPIBase = server.URL
	clock = func() time.Time { return graphQLNow }
	t.Cleanup(func() { githubAPIBase, clock = oldBase, oldClock })
	return server.Client()
}

func answer(status int, headers map[string]string, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for name, value := range headers {
			w.Header().Set(name, value)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func rateLimitUntil(t *testing.T, err error) time.Time {
	t.Helper()
	var limited *RateLimitError
	if !errors.Is(err, ErrRateLimited) || !errors.As(err, &limited) {
		t.Fatalf("err = %v, want a *RateLimitError", err)
	}
	return limited.Until
}

func TestGraphQLPostsTheQueryWithVariablesAndToken(t *testing.T) {
	var method, path, auth, contentType string
	var sent graphQLRequest
	client := graphQLServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		auth, contentType = r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&sent)
		_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"octo"}}}`))
	})

	var out struct{ Viewer login }
	errs, err := GraphQL(context.Background(), client, "token", "query($o: String!) { viewer { login } }", map[string]any{"o": "acme"}, &out)
	if err != nil || errs != nil {
		t.Fatalf("errs = %v, err = %v", errs, err)
	}
	if method != http.MethodPost || path != "/graphql" || auth != "Bearer token" || contentType != "application/json" {
		t.Fatalf("got %s %s, auth %q, content type %q", method, path, auth, contentType)
	}
	if sent.Query != "query($o: String!) { viewer { login } }" || sent.Variables["o"] != "acme" {
		t.Fatalf("sent %#v", sent)
	}
	if out.Viewer.Login != "octo" {
		t.Fatalf("out = %#v", out)
	}
}

// A partial error is the caller's to judge per alias, so it must arrive with its path and beside the data.
func TestGraphQLReturnsPerAliasErrorsWithTheirPathBesideTheData(t *testing.T) {
	client := graphQLServer(t, answer(http.StatusOK, nil,
		`{"data":{"r0":null,"r1":{"ok":true}},"errors":[{"type":"NOT_FOUND","message":"gone","path":["r0"]}]}`))

	var out map[string]json.RawMessage
	errs, err := GraphQL(context.Background(), client, "token", "q", nil, &out)
	if err != nil {
		t.Fatal(err)
	}
	want := []GraphQLError{{Type: "NOT_FOUND", Message: "gone", Path: []any{"r0"}}}
	if !reflect.DeepEqual(errs, want) {
		t.Fatalf("errs = %#v", errs)
	}
	if string(out["r1"]) != `{"ok":true}` {
		t.Fatalf("out = %s", out)
	}
}

func TestGraphQLAcceptsNullData(t *testing.T) {
	client := graphQLServer(t, answer(http.StatusOK, nil, `{"data":null,"errors":[{"message":"boom"}]}`))
	var out map[string]json.RawMessage
	errs, err := GraphQL(context.Background(), client, "token", "q", nil, &out)
	if err != nil || len(errs) != 1 || out != nil {
		t.Fatalf("errs = %v, out = %v, err = %v", errs, out, err)
	}
}

// A spent budget can ride on any status, so the headers must be read before the status is judged.
func TestGraphQLRateLimitHeadersAreReadOnEveryStatus(t *testing.T) {
	reset := graphQLNow.Add(17 * time.Minute)
	headers := map[string]string{
		"x-ratelimit-remaining": "0",
		"x-ratelimit-reset":     strconv.FormatInt(reset.Unix(), 10),
	}
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusForbidden, http.StatusBadGateway} {
		client := graphQLServer(t, answer(status, headers, `{"message":"whatever"}`))
		_, err := GraphQL(context.Background(), client, "token", "q", nil, nil)
		if until := rateLimitUntil(t, err); !until.Equal(reset) {
			t.Fatalf("HTTP %d: until = %v, want %v", status, until, reset)
		}
	}
}

func TestGraphQLRateLimitedErrorBodyOnA200IsRateLimited(t *testing.T) {
	reset := graphQLNow.Add(5 * time.Minute)
	client := graphQLServer(t, answer(http.StatusOK,
		map[string]string{"x-ratelimit-remaining": "3", "x-ratelimit-reset": strconv.FormatInt(reset.Unix(), 10)},
		`{"errors":[{"type":"RATE_LIMITED","message":"API rate limit exceeded"}]}`))
	_, err := GraphQL(context.Background(), client, "token", "q", nil, nil)
	if until := rateLimitUntil(t, err); !until.Equal(reset) {
		t.Fatalf("until = %v, want %v", until, reset)
	}
}

func TestGraphQLSpentBudgetWithAPastResetFallsBackToOneMinute(t *testing.T) {
	client := graphQLServer(t, answer(http.StatusOK,
		map[string]string{"x-ratelimit-remaining": "0", "x-ratelimit-reset": strconv.FormatInt(graphQLNow.Add(-time.Minute).Unix(), 10)},
		`{"data":null}`))
	_, err := GraphQL(context.Background(), client, "token", "q", nil, nil)
	if until := rateLimitUntil(t, err); !until.Equal(graphQLNow.Add(time.Minute)) {
		t.Fatalf("until = %v", until)
	}
}

// The reset header names the primary window, so a secondary limit must not pause until it.
func TestGraphQLSecondaryLimitWaitsForRetryAfterAndOneMinuteWithoutIt(t *testing.T) {
	primaryReset := strconv.FormatInt(graphQLNow.Add(time.Hour).Unix(), 10)
	for _, tc := range []struct {
		name   string
		status int
		header map[string]string
		body   string
		want   time.Time
	}{
		{"200 with retry-after", http.StatusOK,
			map[string]string{"retry-after": "30", "x-ratelimit-remaining": "4000", "x-ratelimit-reset": primaryReset},
			`{"errors":[{"message":"You have exceeded a Secondary Rate Limit."}]}`, graphQLNow.Add(30 * time.Second)},
		{"403 with retry-after", http.StatusForbidden,
			map[string]string{"retry-after": "90", "x-ratelimit-remaining": "4000", "x-ratelimit-reset": primaryReset},
			`{"message":"You have exceeded a secondary rate limit. Please wait."}`, graphQLNow.Add(90 * time.Second)},
		{"429 with retry-after", http.StatusTooManyRequests,
			map[string]string{"retry-after": "45", "x-ratelimit-remaining": "4000", "x-ratelimit-reset": primaryReset},
			`{"message":"You have exceeded a secondary rate limit."}`, graphQLNow.Add(45 * time.Second)},
		{"429 without retry-after", http.StatusTooManyRequests, nil,
			`{"message":"You have exceeded a secondary rate limit."}`, graphQLNow.Add(time.Minute)},
		{"200 without retry-after", http.StatusOK, nil,
			`{"errors":[{"message":"secondary rate limit"}]}`, graphQLNow.Add(time.Minute)},
		{"403 without retry-after", http.StatusForbidden, nil,
			`{"message":"You have exceeded a secondary rate limit."}`, graphQLNow.Add(time.Minute)},
	} {
		client := graphQLServer(t, answer(tc.status, tc.header, tc.body))
		_, err := GraphQL(context.Background(), client, "token", "q", nil, nil)
		if until := rateLimitUntil(t, err); !until.Equal(tc.want) {
			t.Fatalf("%s: until = %v, want %v", tc.name, until, tc.want)
		}
	}
}

func TestGraphQL401WithoutASpentBudgetIsATokenRejection(t *testing.T) {
	client := graphQLServer(t, answer(http.StatusUnauthorized, map[string]string{"x-ratelimit-remaining": "59"}, `{"message":"Bad credentials"}`))
	_, err := GraphQL(context.Background(), client, "token", "q", nil, nil)
	if !errors.Is(err, ErrTokenRejected) || errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v", err)
	}
}

func TestGraphQLPlain403And502CarryTheirStatusAndAreNotRateLimits(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusBadGateway} {
		client := graphQLServer(t, answer(status, map[string]string{"x-ratelimit-remaining": "10"}, `<html>bad gateway</html>`))
		_, err := GraphQL(context.Background(), client, "token", "q", nil, nil)
		if !errors.Is(err, ErrUnexpectedStatus) || errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTokenRejected) {
			t.Fatalf("HTTP %d: err = %v", status, err)
		}
		if want := strconv.Itoa(status); !strings.Contains(err.Error(), want) {
			t.Fatalf("HTTP %d: err = %v does not name the status", status, err)
		}
	}
}

func TestGraphQLReturnsTransportErrors(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("dial refused") })}
	if _, err := GraphQL(context.Background(), client, "token", "q", nil, nil); err == nil {
		t.Fatal("a transport failure returned no error")
	}
}
