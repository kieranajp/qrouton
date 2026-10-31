package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"html_url"`
}

// ListPullRequests names the pull requests opened from one branch of a
// repository. With etag set the request is conditional, and a true notModified
// means the caller's last answer still stands.
func ListPullRequests(ctx context.Context, client *http.Client, token, owner, repo, branch, etag string) (prs []PullRequest, nextETag string, notModified bool, err error) {
	requestURL := githubAPIBase + fmt.Sprintf(pullsQuery,
		url.PathEscape(owner), url.PathEscape(repo), url.QueryEscape(owner), url.QueryEscape(branch), pageSize)
	headers := http.Header{}
	if etag != "" {
		headers.Set(ifNoneMatchHeader, etag)
	}
	prs = []PullRequest{}
	status, answer, err := GetJSON(ctx, client, requestURL, token, headers, &prs)
	if err != nil {
		return nil, "", false, fmt.Errorf("github: listing %s/%s pull requests from %s: %w", owner, repo, branch, err)
	}
	if status == http.StatusNotModified {
		return nil, etag, true, nil
	}
	return prs, answer.Get(etagHeader), false, nil
}
