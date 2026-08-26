package ticket

import (
	"context"
	"encoding/json"
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

func TestLinearReferencesCanonicalizeToTheWorkspaceFreeURL(t *testing.T) {
	want := "https://linear.app/issue/LIF-2841"
	for _, raw := range []string{
		"LIF-2841",
		"lif-2841",
		"  lif-2841  ",
		"https://linear.app/issue/LIF-2841",
		"https://linear.app/issue/lif-2841/fix-retries?source=custom#work",
		"https://linear.app/lifesum/issue/LIF-2841",
		"https://linear.app/lifesum/issue/lif-2841/fix-retries?source=custom#work",
	} {
		t.Run(raw, func(t *testing.T) {
			got, err := CanonicalLinearURL(raw)
			if err != nil || got != want {
				t.Fatalf("CanonicalLinearURL(%q) = %q, %v; want %q", raw, got, err, want)
			}
		})
	}
}

func TestLinearReferencesRejectAnythingOutsideTheIdentifierContract(t *testing.T) {
	oversized := strings.Repeat("A", linearMaxReferenceBytes+1)
	longID := strings.Repeat("A", linearMaxIdentifierBytes) + "-1"
	for _, raw := range []string{
		"",
		"   ",
		"LIF-0",
		"LIF-01",
		"-2841",
		"LIF_2841",
		"LIF 2841",
		"LÍF-2841",
		"LIF-２８４１",
		"LIF-2841\nwhoami",
		`"LIF-2841"`,
		"LIF-2841;open /tmp/pwned",
		"$(open /tmp/pwned)",
		oversized,
		longID,
		"http://linear.app/issue/LIF-2841",
		"https://example.com/issue/LIF-2841",
		"https://user@linear.app/issue/LIF-2841",
		"https://linear.app:443/issue/LIF-2841",
		"https://linear.app/LIF-2841",
		"https://linear.app/issue",
		"https://linear.app/issue/LIF-0",
		"https://linear.app//issue/LIF-2841",
		"https://linear.app/issue//LIF-2841",
		"https://linear.app/issue/LIF%2D2841",
		"https://linear.app/acme/not-issue/LIF-2841",
		"https://linear.app/acme/issue/LIF-2841/../OTHER-2",
	} {
		t.Run(raw, func(t *testing.T) {
			if got, err := CanonicalLinearURL(raw); err == nil {
				t.Fatalf("CanonicalLinearURL(%q) = %q, want an error", raw, got)
			}
		})
	}
}

func TestBothLinearURLShapesFetchTheSameCanonicalIdentifier(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "linear-token")
	ids := make(chan string, 3)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Variables map[string]string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		ids <- payload.Variables[linearIDVar]
		return response(`{"data":{"issue":{"title":"Fix retries","description":"Retry failed requests"}}}`), nil
	})}
	for _, raw := range []string{
		"https://linear.app/issue/lif-2841/fix-retries",
		"https://linear.app/lifesum/issue/LIF-2841/fix-retries",
		"https://LINEAR.APP/issue/Lif-2841?source=custom#work",
	} {
		if _, err := Fetch(context.Background(), client, raw); err != nil {
			t.Fatalf("Fetch(%q) = %v", raw, err)
		}
		if got := <-ids; got != "LIF-2841" {
			t.Fatalf("Fetch(%q) queried id %q", raw, got)
		}
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

func TestParseURLKeepsAsanaAndAcceptsBothLinearShapes(t *testing.T) {
	for _, raw := range []string{
		"https://linear.app/issue/API-42",
		"https://linear.app/acme/issue/API-42/fix-retries",
		"https://app.asana.com/0/123/456",
	} {
		if _, err := ParseURL(raw); err != nil {
			t.Fatalf("ParseURL(%q) = %v", raw, err)
		}
	}
}
