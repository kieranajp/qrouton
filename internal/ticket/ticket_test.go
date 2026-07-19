package ticket

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func response(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestFetchLinearTicket(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "linear-token")
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "linear-token" {
			t.Fatalf("unexpected request: %s, auth %q", r.Method, r.Header.Get("Authorization"))
		}
		return response(`{"data":{"issue":{"title":"Fix retries","description":"Retry failed requests"}}}`), nil
	})}
	got, err := Fetch(context.Background(), client, "https://linear.app/acme/issue/API-42/fix-retries")
	if err != nil || got.Title != "Fix retries" || got.Body != "Retry failed requests" {
		t.Fatalf("Fetch() = %#v, %v", got, err)
	}
}

func TestFetchAsanaTicket(t *testing.T) {
	t.Setenv("ASANA_ACCESS_TOKEN", "asana-token")
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/1.0/tasks/456" || r.Header.Get("Authorization") != "Bearer asana-token" {
			t.Fatalf("unexpected request: %s, auth %q", r.URL, r.Header.Get("Authorization"))
		}
		return response(`{"data":{"name":"Fix retries","notes":"Retry failed requests"}}`), nil
	})}
	got, err := Fetch(context.Background(), client, "https://app.asana.com/0/123/456")
	if err != nil || got.Title != "Fix retries" || got.Body != "Retry failed requests" {
		t.Fatalf("Fetch() = %#v, %v", got, err)
	}
}

func TestParseURLRejectsGitHub(t *testing.T) {
	if _, err := ParseURL("https://github.com/acme/api/issues/42"); err == nil {
		t.Fatal("GitHub ticket URL was accepted")
	}
}
