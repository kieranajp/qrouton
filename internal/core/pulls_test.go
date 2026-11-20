package core

import (
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/status"
)

// GitHub keeps isDraft set on a closed draft, so checking draft first would draw it as live work.
func TestPullRequestStateFollowsMergedClosedDraftOpen(t *testing.T) {
	for _, tc := range []struct {
		name string
		pr   github.PullRequest
		want string
	}{
		{"open", github.PullRequest{State: github.PullStateOpen}, status.PullRequestStateOpen},
		{"draft", github.PullRequest{State: github.PullStateOpen, IsDraft: true}, status.PullRequestStateDraft},
		{"closed", github.PullRequest{State: github.PullStateClosed}, status.PullRequestStateClosed},
		{"closed draft", github.PullRequest{State: github.PullStateClosed, IsDraft: true}, status.PullRequestStateClosed},
		{"merged", github.PullRequest{State: github.PullStateMerged}, status.PullRequestStateMerged},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := pullRequestRows("acme/web", []github.PullRequest{tc.pr})
			if rows[0].State != tc.want {
				t.Fatalf("state = %q, want %q", rows[0].State, tc.want)
			}
		})
	}
}

func TestPullRequestRowsCarryAuthorReviewChecksAndUpdate(t *testing.T) {
	at := time.Date(2026, 11, 19, 8, 41, 0, 0, time.UTC)
	rows := pullRequestRows("acme/web", []github.PullRequest{
		{Number: 1, Author: "kieran", ReviewDecision: github.ReviewApproved, CheckState: "SUCCESS", UpdatedAt: at},
		{Number: 2, ReviewDecision: github.ReviewChangesRequested, CheckState: github.CheckFailure},
		{Number: 3, ReviewDecision: "REVIEW_REQUIRED", CheckState: github.CheckError},
		{Number: 4, CheckState: "PENDING"},
	})
	want := []struct {
		review  string
		failing bool
	}{
		{status.PullRequestReviewApproved, false},
		{status.PullRequestReviewChangesRequested, true},
		{"", true},
		{"", false},
	}
	for i, w := range want {
		if rows[i].Review != w.review || rows[i].ChecksFailing != w.failing {
			t.Fatalf("row %d = %+v, want review %q failing %v", i, rows[i], w.review, w.failing)
		}
	}
	if rows[0].Author != "kieran" || !rows[0].UpdatedAt.Equal(at) || rows[0].Repo != "acme/web" {
		t.Fatalf("row 0 = %+v", rows[0])
	}
}
