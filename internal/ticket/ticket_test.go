package ticket

import (
	"context"
	"encoding/json"
	"errors"
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
			got, err := Canonical(raw)
			if err != nil || got != want {
				t.Fatalf("Canonical(%q) = %q, %v; want %q", raw, got, err, want)
			}
		})
	}
}

func TestKeyKeepsOnlyAValidatedLinearIdentifier(t *testing.T) {
	if got := Key("https://linear.app/lifesum/issue/lif-2841/title"); got != "LIF-2841" {
		t.Fatalf("Key() = %q", got)
	}
	for _, raw := range []string{"", "not-a-ticket", "https://app.asana.com/0/123/456"} {
		if got := Key(raw); got != "" {
			t.Fatalf("Key(%q) = %q", raw, got)
		}
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
			if got, err := Canonical(raw); err == nil {
				t.Fatalf("Canonical(%q) = %q, want an error", raw, got)
			}
		})
	}
}

func TestLinearRejectsAReferenceOverTheByteLimit(t *testing.T) {
	raw := "https://linear.app/issue/LIF-2841?q=" + strings.Repeat("a", linearMaxReferenceBytes)
	if got, err := Canonical(raw); err == nil {
		t.Fatalf("Canonical(oversized) = %q, want an error", got)
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

func TestValidateAcceptsEveryProvidersLinkShape(t *testing.T) {
	for _, raw := range []string{
		"https://linear.app/issue/API-42",
		"https://linear.app/acme/issue/API-42/fix-retries",
		"https://app.asana.com/0/123/456",
		"https://github.com/acme/api/issues/42",
	} {
		if err := Validate(raw); err != nil {
			t.Fatalf("Validate(%q) = %v", raw, err)
		}
	}
}

func statusResponse(code int, status string) *http.Response {
	return &http.Response{StatusCode: code, Status: status, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}
}

func failingClient(code int, status string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return statusResponse(code, status), nil
	})}
}

func bodyClient(body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(body), nil
	})}
}

func TestFetchWithoutACredentialNamesTheEnvironmentVariable(t *testing.T) {
	for _, tc := range []struct {
		env  string
		raw  string
		want error
	}{
		{"LINEAR_API_KEY", "https://linear.app/issue/LIF-2841", ErrNoLinearToken},
		{"ASANA_ACCESS_TOKEN", "https://app.asana.com/0/123/456", ErrNoAsanaToken},
	} {
		t.Run(tc.env, func(t *testing.T) {
			t.Setenv(tc.env, "")
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("request issued without a credential")
				return nil, nil
			})}
			if _, err := Fetch(context.Background(), client, tc.raw); !errors.Is(err, tc.want) {
				t.Fatalf("Fetch() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestFetchReportsANon2xxWithItsStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    string
		raw    string
		code   int
		status string
		want   string
	}{
		{"linear", "LINEAR_API_KEY", "https://linear.app/issue/LIF-2841",
			http.StatusInternalServerError, "500 Internal Server Error",
			"linear: loading ticket: request failed: 500 Internal Server Error"},
		{"asana", "ASANA_ACCESS_TOKEN", "https://app.asana.com/0/123/456",
			http.StatusNotFound, "404 Not Found",
			"asana: loading ticket: request failed: 404 Not Found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.env, "token")
			_, err := Fetch(context.Background(), failingClient(tc.code, tc.status), tc.raw)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("Fetch() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestFetchLinearSurfacesAGraphQLErrorVerbatim(t *testing.T) {
	t.Setenv("LINEAR_API_KEY", "linear-token")
	client := bodyClient(`{"errors":[{"message":"Entity not found"}]}`)
	_, err := Fetch(context.Background(), client, "https://linear.app/issue/LIF-2841")
	if err == nil || err.Error() != "linear: loading ticket: Entity not found" {
		t.Fatalf("Fetch() = %v", err)
	}
}

func TestFetchTreatsAnEmptyTitleAsANoSuchTicket(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		raw  string
		body string
		want string
	}{
		{"linear", "LINEAR_API_KEY", "https://linear.app/issue/LIF-2841",
			`{"data":{"issue":{"title":"","description":"body"}}}`, "linear: ticket not found"},
		{"asana", "ASANA_ACCESS_TOKEN", "https://app.asana.com/0/123/456",
			`{"data":{"name":"","notes":"body"}}`, "asana: ticket not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.env, "token")
			_, err := Fetch(context.Background(), bodyClient(tc.body), tc.raw)
			if !errors.Is(err, ErrTicketNotFound) || err.Error() != tc.want {
				t.Fatalf("Fetch() = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAsanaCanonicalizesToTheLinkAsGivenAndSeedsNoSlug(t *testing.T) {
	raw := "  https://app.asana.com/0/123/456  "
	got, err := Canonical(raw)
	if err != nil || got != strings.TrimSpace(raw) {
		t.Fatalf("Canonical(%q) = %q, %v", raw, got, err)
	}
	if key := Key(raw); key != "" {
		t.Fatalf("Key(%q) = %q, want empty", raw, key)
	}
}
