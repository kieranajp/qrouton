package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// GraphQL POSTs to githubAPIBase + "/graphql" and decodes "data" into out.
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
	if limited := rateLimit(resp.StatusCode, resp.Header, body); limited != nil {
		return nil, limited
	}
	switch {
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
	return body.Errors, nil
}

func rateLimit(status int, header http.Header, body graphQLResponse) *RateLimitError {
	now := clock()
	if budgetSpent(header, body.Errors) {
		if reset, err := strconv.ParseInt(strings.TrimSpace(header.Get(rateLimitResetHeader)), 10, 64); err == nil {
			if until := time.Unix(reset, 0); until.After(now) {
				return &RateLimitError{Until: until}
			}
		}
		return &RateLimitError{Until: retryAfter(now, header)}
	}
	if (status == http.StatusOK || status == http.StatusForbidden || status == http.StatusTooManyRequests) && secondaryLimit(body) {
		return &RateLimitError{Until: retryAfter(now, header)}
	}
	return nil
}

func budgetSpent(header http.Header, errs []GraphQLError) bool {
	if remaining, err := strconv.Atoi(strings.TrimSpace(header.Get(rateLimitRemainingHeader))); err == nil && remaining == 0 {
		return true
	}
	for _, e := range errs {
		if e.Type == rateLimitedType {
			return true
		}
	}
	return false
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
