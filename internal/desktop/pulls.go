package desktop

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
)

// pullRequestAPI is the GitHub work a fetch does, as fields so a test drives
// the error rules without a token or a network.
type pullRequestAPI struct {
	credential func(ctx context.Context) (github.Credential, string)
	list       func(ctx context.Context, token, owner, repo, branch, etag string) ([]github.PullRequest, string, bool, error)
}

func livePullRequestAPI() pullRequestAPI {
	return pullRequestAPI{
		credential: func(ctx context.Context) (github.Credential, string) {
			ctx, cancel := context.WithTimeout(ctx, pullRequestTimeout)
			defer cancel()
			return github.ResolveCredential(ctx)
		},
		list: func(ctx context.Context, token, owner, repo, branch, etag string) ([]github.PullRequest, string, bool, error) {
			ctx, cancel := context.WithTimeout(ctx, pullRequestTimeout)
			defer cancel()
			return github.ListPullRequests(ctx, http.DefaultClient, token, owner, repo, branch, etag)
		},
	}
}

// sessionPulls is one session's cached pull requests, per repository in
// manifest order. A fetch builds a new value and never mutates the one it read.
type sessionPulls struct {
	order     []string
	repos     map[string]repoPulls
	signedOut bool
}

type repoPulls struct {
	rows []status.PullRequest
	etag string
	// failure is the last logged error, so a dead network logs once.
	failure string
}

func (p sessionPulls) list() status.PullRequestList {
	out := status.PullRequestList{Items: []status.PullRequest{}, SignedOut: p.signedOut}
	for _, id := range p.order {
		out.Items = append(out.Items, p.repos[id].rows...)
	}
	return out
}

func (api pullRequestAPI) fetch(ctx context.Context, root string, previous sessionPulls) sessionPulls {
	m, err := session.Load(root)
	if err != nil {
		return previous
	}
	next := sessionPulls{repos: map[string]repoPulls{}}
	var targets []session.ManifestRepo
	for _, repo := range m.Repos {
		if !repo.Role.IsEditing() || repo.Branch == "" {
			continue
		}
		id := pullRequestRepoID(repo)
		next.order = append(next.order, id)
		next.repos[id] = previous.repos[id]
		targets = append(targets, repo)
	}
	if len(targets) == 0 {
		return next
	}
	_, token := api.credential(ctx)
	if token == "" {
		next.signedOut = true
		return next
	}
	for _, repo := range targets {
		id := pullRequestRepoID(repo)
		cached := next.repos[id]
		prs, etag, notModified, err := api.list(ctx, token, repo.Org, repo.Name, repo.Branch, cached.etag)
		if ctx.Err() != nil {
			return previous
		}
		switch {
		case err == nil && notModified:
			cached.failure = ""
		case err == nil:
			cached = repoPulls{rows: pullRequestRows(id, prs), etag: etag}
		case errors.Is(err, github.ErrNotFound):
			cached = repoPulls{}
		case errors.Is(err, github.ErrTokenRejected):
			next.signedOut = true
			return next
		default:
			if text := err.Error(); text != cached.failure {
				log.Printf(pullRequestLogFormat, id, err)
				cached.failure = text
			}
		}
		next.repos[id] = cached
	}
	return next
}

func pullRequestRepoID(repo session.ManifestRepo) string {
	return github.Repo{Org: repo.Org, Name: repo.Name}.ID()
}

func pullRequestRows(id string, prs []github.PullRequest) []status.PullRequest {
	rows := make([]status.PullRequest, 0, len(prs))
	for _, pr := range prs {
		rows = append(rows, status.PullRequest{Repo: id, Number: pr.Number, Title: pr.Title, URL: pr.URL})
	}
	return rows
}
