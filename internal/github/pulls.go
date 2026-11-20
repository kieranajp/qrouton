package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PullRequest is one pull request as GraphQL reports it, flattened.
type PullRequest struct {
	Number     int
	Title      string
	URL        string
	State      string
	IsDraft    bool
	UpdatedAt  time.Time
	Author     string   // empty for a deleted account
	Reviews    []string // each writer's latest APPROVED or CHANGES_REQUESTED, oldest first
	CheckState string
}

type BranchTarget struct{ Owner, Name, Branch string }

type BranchResult struct {
	PullRequests []PullRequest // never nil when Err is nil
	Err          error         // this target only; wraps ErrNotFound for a missing repository
}

type login struct {
	Login string `json:"login"`
}

type pullNode struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	State     string    `json:"state"`
	IsDraft   bool      `json:"isDraft"`
	UpdatedAt time.Time `json:"updatedAt"`
	Author    *login    `json:"author"`
	Reviews   *struct {
		Nodes []*struct {
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
	HeadRepositoryOwner *login `json:"headRepositoryOwner"`
	StatusCheckRollup   *struct {
		State string `json:"state"`
	} `json:"statusCheckRollup"`
}

type pullsRepository struct {
	PullRequests struct {
		Nodes []*pullNode `json:"nodes"`
	} `json:"pullRequests"`
}

// BranchPullRequests asks for every target's pull requests in one request;
// results[i] answers targets[i]. A *RateLimitError comes with results when the
// response that spent the last of the budget still answered.
func BranchPullRequests(ctx context.Context, client *http.Client, token string, targets []BranchTarget) (viewer string, results []BranchResult, err error) {
	if len(targets) == 0 {
		return "", []BranchResult{}, nil
	}
	variables := make([]string, len(targets))
	aliases := make([]string, len(targets))
	index := make(map[string]int, len(targets))
	vars := make(map[string]any, 3*len(targets))
	for i, target := range targets {
		variables[i] = fmt.Sprintf(pullsVariablesFormat, i)
		alias := fmt.Sprintf(aliasFormat, i)
		aliases[i] = fmt.Sprintf(pullsAliasFormat, alias, i, pullsPageSize)
		index[alias] = i
		vars[fmt.Sprintf(ownerVariableFormat, i)] = target.Owner
		vars[fmt.Sprintf(nameVariableFormat, i)] = target.Name
		vars[fmt.Sprintf(branchVariableFormat, i)] = target.Branch
	}
	query := fmt.Sprintf(pullsQueryFormat, strings.Join(variables, variableSeparator), strings.Join(aliases, ""))

	var data map[string]json.RawMessage
	errs, err := GraphQL(ctx, client, token, query, vars, &data)
	var spent error
	if err != nil {
		err = fmt.Errorf("github: listing pull requests: %w", err)
		if !errors.Is(err, ErrRateLimited) || data == nil {
			return "", nil, err
		}
		spent = err
	}

	results = make([]BranchResult, len(targets))
	for _, e := range errs {
		i, ok := aliasIndex(e.Path, index)
		if !ok {
			return "", nil, errors.Join(fmt.Errorf("github: listing pull requests: %w: %s", ErrQuery, e.Message), spent)
		}
		if results[i].Err != nil {
			continue
		}
		target, kind := targets[i], ErrQuery
		if e.Type == notFoundType {
			kind = ErrNotFound
		}
		results[i].Err = fmt.Errorf("github: %s/%s pull requests from %s: %w: %s", target.Owner, target.Name, target.Branch, kind, e.Message)
	}

	var who *login
	if raw, ok := data[viewerField]; ok {
		if err := json.Unmarshal(raw, &who); err != nil {
			return "", nil, errors.Join(fmt.Errorf("github: listing pull requests: %w", err), spent)
		}
	}
	if who != nil {
		viewer = who.Login
	}

	for i, target := range targets {
		if results[i].Err != nil {
			continue
		}
		var repo *pullsRepository
		if raw, ok := data[fmt.Sprintf(aliasFormat, i)]; ok {
			if err := json.Unmarshal(raw, &repo); err != nil {
				return "", nil, errors.Join(fmt.Errorf("github: listing pull requests: %w", err), spent)
			}
		}
		results[i].PullRequests = ownPullRequests(repo, target.Owner)
	}
	return viewer, results, spent
}

func aliasIndex(path []any, index map[string]int) (int, bool) {
	if len(path) == 0 {
		return 0, false
	}
	alias, ok := path[0].(string)
	if !ok {
		return 0, false
	}
	i, ok := index[alias]
	return i, ok
}

// ownPullRequests drops a fork's pull request: headRefName alone matches any head repository.
func ownPullRequests(repo *pullsRepository, owner string) []PullRequest {
	prs := []PullRequest{}
	if repo == nil {
		return prs
	}
	for _, node := range repo.PullRequests.Nodes {
		if node == nil || node.HeadRepositoryOwner == nil || !strings.EqualFold(node.HeadRepositoryOwner.Login, owner) {
			continue
		}
		pr := PullRequest{
			Number:    node.Number,
			Title:     node.Title,
			URL:       node.URL,
			State:     node.State,
			IsDraft:   node.IsDraft,
			UpdatedAt: node.UpdatedAt,
		}
		if node.Reviews != nil {
			for _, review := range node.Reviews.Nodes {
				if review != nil {
					pr.Reviews = append(pr.Reviews, review.State)
				}
			}
		}
		if node.Author != nil {
			pr.Author = node.Author.Login
		}
		if node.StatusCheckRollup != nil {
			pr.CheckState = node.StatusCheckRollup.State
		}
		prs = append(prs, pr)
	}
	return prs
}
