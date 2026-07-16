package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

var githubAPIBase = "https://api.github.com"

type Repo struct {
	Name          string    `json:"name"`
	Org           string    `json:"org"`
	SSHURL        string    `json:"ssh_url"`
	DefaultBranch string    `json:"default_branch"`
	PushedAt      time.Time `json:"pushed_at"`
}

type repoCache struct {
	SchemaVersion int       `json:"schemaVersion"`
	FetchedAt     time.Time `json:"fetchedAt"`
	Orgs          []string  `json:"orgs"`
	Repos         []Repo    `json:"repos"`
}

type repoRefreshState string

const (
	repoRefreshStarted   repoRefreshState = "started"
	repoRefreshSucceeded repoRefreshState = "succeeded"
	repoRefreshFailed    repoRefreshState = "failed"
	repoRefreshComplete  repoRefreshState = "complete"
)

// repoRefreshMsg is emitted by refreshRepos once when an owner starts and once
// when it finishes. Complete is the final message and contains the merged,
// activity-sorted result (including cached data for owners which failed).
type repoRefreshMsg struct {
	Owner string
	State repoRefreshState
	Repos []Repo
	Err   error
}

// cachedRepos is deliberately cache-first: unlike listRepos it returns usable
// cached data even when stale. The bool reports whether a matching cache exists.
func cachedRepos(orgs []string) ([]Repo, time.Time, bool) {
	var c repoCache
	b, err := os.ReadFile(cachePath())
	if err != nil || json.Unmarshal(b, &c) != nil || c.SchemaVersion != 2 || !slices.Equal(c.Orgs, orgs) {
		return nil, time.Time{}, false
	}
	repos := slices.Clone(c.Repos)
	sortReposByActivity(repos)
	return repos, c.FetchedAt, true
}

// refreshRepos fetches configured owners concurrently. Successful results are
// merged immediately; failures retain that owner's cached repositories. Calling
// it again (or refreshOwnerRepos directly) provides owner-level retry.
func refreshRepos(ctx context.Context, client *http.Client, token string, orgs []string, cached []Repo) <-chan repoRefreshMsg {
	// This is the maximum possible event count. A full buffer lets the
	// coordinator and workers terminate even if the consumer stops reading.
	out := make(chan repoRefreshMsg, 2*len(orgs)+1)
	go func() {
		defer close(out)
		type result struct {
			owner string
			repos []Repo
			err   error
		}
		results := make(chan result, len(orgs))
		for _, owner := range orgs {
			owner := owner
			out <- repoRefreshMsg{Owner: owner, State: repoRefreshStarted}
			go func() {
				repos, err := refreshOwnerRepos(ctx, client, token, owner)
				results <- result{owner: owner, repos: repos, err: err}
			}()
		}

		merged := slices.Clone(cached)
		for range orgs {
			r := <-results
			if r.err != nil {
				out <- repoRefreshMsg{Owner: r.owner, State: repoRefreshFailed, Err: r.err}
				continue
			}
			merged = replaceOwnerRepos(merged, r.owner, r.repos)
			sortReposByActivity(merged)
			out <- repoRefreshMsg{Owner: r.owner, State: repoRefreshSucceeded, Repos: slices.Clone(merged)}
		}
		sortReposByActivity(merged)
		if ctx.Err() == nil {
			writeRepoCache(orgs, merged)
		}
		out <- repoRefreshMsg{State: repoRefreshComplete, Repos: merged}
	}()
	return out
}

func replaceOwnerRepos(repos []Repo, owner string, replacement []Repo) []Repo {
	merged := make([]Repo, 0, len(repos)+len(replacement))
	for _, repo := range repos {
		if !strings.EqualFold(repo.Org, owner) {
			merged = append(merged, repo)
		}
	}
	return append(merged, replacement...)
}

func writeRepoCache(orgs []string, repos []Repo) {
	_ = os.MkdirAll(filepath.Dir(cachePath()), 0o755)
	b, err := json.MarshalIndent(repoCache{SchemaVersion: 2, FetchedAt: time.Now(), Orgs: orgs, Repos: repos}, "", "  ")
	if err == nil {
		_ = os.WriteFile(cachePath(), b, 0o644) // cache write failure is not fatal
	}
}

// token: gh auth token → GITHUB_TOKEN → error. gh owns keychain/hosts.yml resolution.
func githubToken() (string, error) {
	if out, err := exec.Command("gh", "auth", "token").Output(); err == nil {
		if t := strings.TrimSpace(string(out)); t != "" {
			return t, nil
		}
	}
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t, nil
	}
	return "", errors.New("no GitHub token: run `gh auth login` or set GITHUB_TOKEN")
}

// listRepos returns all configured GitHub owners' repos, from cache unless stale (24h).
func listRepos(orgs []string, refresh bool) ([]Repo, error) {
	cached, fetchedAt, cacheOK := cachedRepos(orgs)
	if cacheOK && !refresh && time.Since(fetchedAt) < 24*time.Hour {
		return cached, nil
	}

	token, err := githubToken()
	if err != nil {
		return nil, err
	}
	var repos []Repo
	var firstErr error
	for msg := range refreshRepos(context.Background(), http.DefaultClient, token, orgs, cached) {
		if msg.State == repoRefreshFailed && firstErr == nil {
			firstErr = msg.Err
		}
		if msg.State == repoRefreshComplete {
			repos = msg.Repos
		}
	}
	if firstErr != nil && len(repos) == 0 {
		return nil, firstErr
	}
	return repos, nil
}

func refreshOwnerRepos(ctx context.Context, client *http.Client, token, owner string) ([]Repo, error) {
	login := ""
	return fetchOwnerReposContext(ctx, client, token, owner, &login)
}

func sortReposByActivity(repos []Repo) {
	sort.SliceStable(repos, func(i, j int) bool {
		if !repos[i].PushedAt.Equal(repos[j].PushedAt) {
			return repos[i].PushedAt.After(repos[j].PushedAt)
		}
		return repoID(repos[i]) < repoID(repos[j])
	})
}

func fetchOwnerRepos(client *http.Client, token, owner string, authenticatedLogin *string) ([]Repo, error) {
	return fetchOwnerReposContext(context.Background(), client, token, owner, authenticatedLogin)
}

func fetchOwnerReposContext(ctx context.Context, client *http.Client, token, owner string, authenticatedLogin *string) ([]Repo, error) {
	var identity struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	if err := githubJSONContext(ctx, client, token, githubAPIBase+"/users/"+url.PathEscape(owner), &identity); err != nil {
		return nil, fmt.Errorf("github: identifying %s: %w", owner, err)
	}

	var endpoint string
	switch identity.Type {
	case "Organization":
		endpoint = githubAPIBase + "/orgs/" + url.PathEscape(owner) + "/repos?type=all"
	case "User":
		if *authenticatedLogin == "" {
			var me struct {
				Login string `json:"login"`
			}
			if err := githubJSONContext(ctx, client, token, githubAPIBase+"/user", &me); err != nil {
				return nil, fmt.Errorf("github: identifying authenticated user: %w", err)
			}
			*authenticatedLogin = me.Login
		}
		if strings.EqualFold(owner, *authenticatedLogin) {
			// The authenticated endpoint includes private repositories owned by this user.
			endpoint = githubAPIBase + "/user/repos?affiliation=owner&visibility=all"
		} else {
			// GitHub exposes only another user's public repositories here.
			endpoint = githubAPIBase + "/users/" + url.PathEscape(owner) + "/repos?type=owner"
		}
	default:
		return nil, fmt.Errorf("github: unsupported owner type %q for %s", identity.Type, owner)
	}

	var repos []Repo
	for page := 1; ; page++ {
		separator := "&"
		if !strings.Contains(endpoint, "?") {
			separator = "?"
		}
		var batch []Repo
		requestURL := fmt.Sprintf("%s%sper_page=100&page=%d", endpoint, separator, page)
		if err := githubJSONContext(ctx, client, token, requestURL, &batch); err != nil {
			return nil, fmt.Errorf("github: listing %s repos (page %d): %w", owner, page, err)
		}
		for i := range batch {
			batch[i].Org = owner
		}
		repos = append(repos, batch...)
		if len(batch) < 100 {
			break
		}
	}
	return repos, nil
}

func githubJSON(client *http.Client, token, requestURL string, dst any) error {
	return githubJSONContext(context.Background(), client, token, requestURL, dst)
}

func githubJSONContext(ctx context.Context, client *http.Client, token, requestURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
