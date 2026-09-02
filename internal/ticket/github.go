package ticket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	gh "github.com/kieranajp/qrouton/internal/github"
)

// Overridable in tests. The credential is the one the repository picker already
// holds: `gh auth token`, then GITHUB_TOKEN.
var (
	githubAPI   = githubAPIDefault
	githubToken = gh.Token
)

var (
	githubNamePattern   = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	githubNumberPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
)

type github struct{}

func (github) Name() string { return githubProvider }

func (github) Hosts() []string { return []string{githubHost} }

// Parse accepts only an issue URL. A pull request is a different kind of thing
// to name a session after, and an enterprise host would need its own API base.
func (github) Parse(u *url.URL) (Reference, error) {
	if len(u.String()) > githubMaxReferenceBytes || u.User != nil || u.Port() != "" {
		return Reference{}, ErrNotGitHubIssue
	}
	escaped := u.EscapedPath()
	if !strings.HasPrefix(escaped, pathSeparator) {
		return Reference{}, ErrNotGitHubIssue
	}
	parts := strings.Split(strings.TrimPrefix(escaped, pathSeparator), pathSeparator)
	if len(parts) != githubIssueSegments || parts[githubIssuesIndex] != githubIssuesSegment {
		return Reference{}, ErrNotGitHubIssue
	}
	owner, repo, number := parts[githubOwnerIndex], parts[githubRepoIndex], parts[githubNumberIndex]
	if !githubName(owner) || !githubName(repo) || !githubNumberPattern.MatchString(number) {
		return Reference{}, ErrNotGitHubIssue
	}
	// GitHub resolves an owner and repository case-insensitively, so dedupe must
	// not read Acme/API and acme/api as two tickets.
	owner, repo = strings.ToLower(owner), strings.ToLower(repo)
	return Reference{
		provider:  github{},
		ID:        fmt.Sprintf(githubIssuePathFormat, owner, repo, number),
		Canonical: fmt.Sprintf(githubCanonicalFormat, owner, repo, number),
		Key:       fmt.Sprintf(githubKeyFormat, repo, number),
	}, nil
}

func (g github) Fetch(ctx context.Context, client *http.Client, ref Reference) (Ticket, error) {
	token, err := githubToken()
	if err != nil {
		return Ticket{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI+ref.ID, nil)
	if err != nil {
		return Ticket{}, err
	}
	req.Header.Set(authorizationHeader, bearerPrefix+token)
	req.Header.Set(acceptHeader, acceptGitHubJSON)
	var response struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := doJSON(client, req, &response); err != nil {
		// GitHub 404s a private issue rather than admitting it exists, so "not
		// found" is the truthful answer to both.
		var status statusError
		if errors.As(err, &status) && status.code == http.StatusNotFound {
			return Ticket{}, notFound(g.Name())
		}
		return Ticket{}, providerError(g.Name(), err)
	}
	if response.Title == "" {
		return Ticket{}, notFound(g.Name())
	}
	return Ticket{Title: response.Title, Body: response.Body}, nil
}

func githubName(segment string) bool {
	if segment == "." || segment == ".." {
		return false
	}
	return githubNamePattern.MatchString(segment)
}
