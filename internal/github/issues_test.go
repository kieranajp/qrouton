package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCreateIssueExactRequestAndOutcomes(t *testing.T) {
	for _, tt := range []struct {
		name string
		code int
		body string
		want error
	}{
		{"created", 201, `{"number":42,"html_url":"https://github.com/kieranajp/qrouton/issues/42"}`, nil},
		{"malformed", 201, `{`, ErrIssueUnknown},
		{"missing", 201, `{"number":42}`, ErrIssueUnknown},
		{"foreign", 201, `{"number":42,"html_url":"https://github.com/other/repo/issues/42"}`, ErrIssueUnknown},
		{"http", 201, `{"number":42,"html_url":"http://github.com/kieranajp/qrouton/issues/42"}`, ErrIssueUnknown},
		{"wrong number", 201, `{"number":41,"html_url":"https://github.com/kieranajp/qrouton/issues/42"}`, ErrIssueUnknown},
		{"trailing garbage", 201, `{"number":42,"html_url":"https://github.com/kieranajp/qrouton/issues/42"}x`, ErrIssueUnknown},
		{"redirect", 307, "", ErrIssueRejected},
		{"unauthorized", 401, "token secret", ErrIssueRejected},
		{"forbidden", 403, "", ErrIssueRejected},
		{"missing repo", 404, "", ErrIssueRejected},
		{"gone", 410, "", ErrIssueRejected},
		{"invalid", 422, "", ErrIssueRejected},
		{"rate limit", 429, "", ErrIssueRejected},
		{"server", 500, "", ErrIssueUnknown},
		{"unexpected success", 200, "", ErrIssueUnknown},
		{"network", 0, "", ErrIssueUnknown},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "POST" || r.URL.String() != "https://api.github.com/repos/kieranajp/qrouton/issues" {
					t.Errorf("request: %s %s", r.Method, r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("headers: %v", r.Header)
				}
				var payload map[string]string
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if len(payload) != 2 || payload["title"] != "title" || payload["body"] != "\n body <img> \n" {
					t.Errorf("payload: %#v", payload)
				}
				if tt.code == 0 {
					return nil, errors.New("network secret")
				}
				return &http.Response{StatusCode: tt.code, Header: http.Header{"Location": {"https://example.com/steal"}}, Body: io.NopCloser(strings.NewReader(tt.body)), Request: r}, nil
			})}
			issue, err := CreateIssue(context.Background(), client, "secret", "title", "\n body <img> \n")
			if !errors.Is(err, tt.want) || calls != 1 {
				t.Fatalf("issue=%+v err=%v calls=%d", issue, err, calls)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe error: %v", err)
			}
			if client.CheckRedirect != nil {
				t.Fatal("mutated caller client")
			}
		})
	}
}

func TestCreateIssueRejectsBeforeTransportAndDoesNotRetryCancellation(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { calls++; return nil, r.Context().Err() })}
	if _, err := CreateIssue(context.Background(), client, "", "title", "body"); !errors.Is(err, ErrNoToken) {
		t.Fatal(err)
	}
	if _, err := CreateIssue(context.Background(), client, "token", " ", "body"); !errors.Is(err, ErrIssueInvalid) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal(calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CreateIssue(ctx, client, "token", "title", "body"); !errors.Is(err, ErrIssueUnknown) {
		t.Fatal(err)
	}
	if calls > 1 {
		t.Fatal(calls)
	}
}
