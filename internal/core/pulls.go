package core

import (
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
)

func pullRequestRepoID(repo session.ManifestRepo) string {
	return github.Repo{Org: repo.Org, Name: repo.Name}.ID()
}

func pullRequestRows(id string, prs []github.PullRequest) []status.PullRequest {
	rows := make([]status.PullRequest, 0, len(prs))
	for _, pr := range prs {
		rows = append(rows, status.PullRequest{
			Repo:          id,
			Number:        pr.Number,
			Title:         pr.Title,
			URL:           pr.URL,
			State:         pullRequestState(pr),
			Author:        pr.Author,
			Review:        pullRequestReview(pr.ReviewDecision),
			ChecksFailing: pr.CheckState == github.CheckFailure || pr.CheckState == github.CheckError,
			UpdatedAt:     pr.UpdatedAt,
		})
	}
	return rows
}

// pullRequestState checks closed before draft: GitHub keeps a closed draft's isDraft set.
func pullRequestState(pr github.PullRequest) string {
	switch {
	case pr.State == github.PullStateMerged:
		return status.PullRequestStateMerged
	case pr.State == github.PullStateClosed:
		return status.PullRequestStateClosed
	case pr.IsDraft:
		return status.PullRequestStateDraft
	default:
		return status.PullRequestStateOpen
	}
}

func pullRequestReview(decision string) string {
	switch decision {
	case github.ReviewApproved:
		return status.PullRequestReviewApproved
	case github.ReviewChangesRequested:
		return status.PullRequestReviewChangesRequested
	default:
		return ""
	}
}
