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
)

type Issue struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
}

func CreateIssue(ctx context.Context, client *http.Client, token, title, body string) (Issue, error) {
	if strings.TrimSpace(token) == "" {
		return Issue{}, ErrNoToken
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return Issue{}, ErrIssueInvalid
	}
	payload, err := json.Marshal(struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{title, body})
	if err != nil {
		return Issue{}, fmt.Errorf("%w", ErrIssueInvalid)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, issueAPIURL, bytes.NewReader(payload))
	if err != nil {
		return Issue{}, fmt.Errorf("%w", ErrIssueInvalid)
	}
	req.Header.Set(authorizationHeader, bearerPrefix+token)
	req.Header.Set(acceptHeader, acceptGitHubJSON)
	req.Header.Set(issueContentTypeHeader, issueJSONType)
	writer := *client
	writer.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := writer.Do(req)
	if err != nil {
		return Issue{}, fmt.Errorf("%w", ErrIssueUnknown)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode >= 500 || resp.StatusCode < 300 {
			return Issue{}, fmt.Errorf("%w", ErrIssueUnknown)
		}
		return Issue{}, fmt.Errorf(issueRejectedFormat, ErrIssueRejected, resp.StatusCode)
	}
	var issue Issue
	decoder := json.NewDecoder(io.LimitReader(resp.Body, issueResponseLimit))
	if decoder.Decode(&issue) != nil || !ValidIssue(issue) {
		return Issue{}, fmt.Errorf("%w", ErrIssueUnknown)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Issue{}, fmt.Errorf("%w", ErrIssueUnknown)
	}
	return issue, nil
}

func ValidIssue(issue Issue) bool {
	return issue.Number > 0 && issue.URL == IssueListURL+"/"+strconv.Itoa(issue.Number)
}
