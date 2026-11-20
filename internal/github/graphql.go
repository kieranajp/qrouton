package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var clock = time.Now

type GraphQLError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Path    []any  `json:"path"`
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphQLResponse also reads message, which a 403 secondary limit carries at the top level.
type graphQLResponse struct {
	Data    json.RawMessage `json:"data"`
	Errors  []GraphQLError  `json:"errors"`
	Message string          `json:"message"`
}

// GraphQL POSTs to githubAPIBase + "/graphql" and decodes "data" into out. A
// 200 that spends the last of the budget still decodes, and returns its
// *RateLimitError beside the errors.
func GraphQL(ctx context.Context, client *http.Client, token, query string, vars map[string]any, out any) ([]GraphQLError, error) {
	payload, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubAPIBase+graphQLPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set(authorizationHeader, bearerPrefix+token)
	}
	req.Header.Set(contentTypeHeader, jsonContentType)
	req.Header.Set(acceptHeader, acceptGitHubJSON)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body graphQLResponse
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, graphQLResponseLimit)).Decode(&body)
	spent := lastPointSpent(resp.Header)
	if limited := refusedForLimit(resp.StatusCode, resp.Header, body); limited != nil {
		if spent != nil && spent.Until.After(limited.Until) {
			return nil, spent
		}
		return nil, limited
	}
	switch {
	case spent != nil && (decodeErr != nil || resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices):
		return nil, spent
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("%w: %s", ErrTokenRejected, resp.Status)
	case resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices:
		return nil, fmt.Errorf("%w: %s", ErrUnexpectedStatus, resp.Status)
	case decodeErr != nil:
		return nil, decodeErr
	}
	if out != nil && len(body.Data) > 0 {
		if err := json.Unmarshal(body.Data, out); err != nil {
			return nil, err
		}
	}
	if spent != nil {
		return body.Errors, spent
	}
	return body.Errors, nil
}

// refusedForLimit is a response whose body says GitHub refused it for a rate limit.
func refusedForLimit(status int, header http.Header, body graphQLResponse) *RateLimitError {
	for _, e := range body.Errors {
		if e.Type == rateLimitedType {
			return &RateLimitError{Until: resetAt(clock(), header)}
		}
	}
	if (status == http.StatusOK || status == http.StatusForbidden || status == http.StatusTooManyRequests) && secondaryLimit(body) {
		return &RateLimitError{Until: retryAfter(clock(), header)}
	}
	return nil
}

// lastPointSpent is a response that left no budget for the next request.
func lastPointSpent(header http.Header) *RateLimitError {
	if remaining, err := strconv.Atoi(strings.TrimSpace(header.Get(rateLimitRemainingHeader))); err == nil && remaining == 0 {
		return &RateLimitError{Until: resetAt(clock(), header)}
	}
	return nil
}

func resetAt(now time.Time, header http.Header) time.Time {
	if reset, err := strconv.ParseInt(strings.TrimSpace(header.Get(rateLimitResetHeader)), 10, 64); err == nil {
		if until := time.Unix(reset, 0); until.After(now) {
			return until
		}
	}
	return retryAfter(now, header)
}

func secondaryLimit(body graphQLResponse) bool {
	if mentionsSecondaryLimit(body.Message) {
		return true
	}
	for _, e := range body.Errors {
		if mentionsSecondaryLimit(e.Message) {
			return true
		}
	}
	return false
}

func mentionsSecondaryLimit(message string) bool {
	return strings.Contains(strings.ToLower(message), secondaryLimitPhrase)
}

func retryAfter(now time.Time, header http.Header) time.Time {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header.Get(retryAfterHeader))); err == nil && seconds >= 0 {
		return now.Add(time.Duration(seconds) * time.Second)
	}
	return now.Add(rateLimitFallback)
}

var requestID = regexp.MustCompile(requestIDPattern)

// WithoutRequestID is message less GitHub's per-request id, so two reports of one failure compare equal.
func WithoutRequestID(message string) string {
	return requestID.ReplaceAllString(message, "")
}
