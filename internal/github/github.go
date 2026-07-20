package github

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

	"github.com/kieranajp/qrouton/internal/config"
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

type RefreshState string

const (
	RefreshStarted   RefreshState = "started"
	RefreshSucceeded RefreshState = "succeeded"
	RefreshFailed    RefreshState = "failed"
	RefreshComplete  RefreshState = "complete"
)

// RefreshMsg is emitted by RefreshRepos once when an owner starts and once
// when it finishes. Complete is the final message and contains the merged,
// activity-sorted result (including cached data for owners which failed).
type RefreshMsg struct {
	Owner string
	State RefreshState
	Repos []Repo
	Err   error
}

// CachedRepos is deliberately cache-first: it returns usable cached data even
// when stale. The bool reports whether a matching cache exists.
func CachedRepos(orgs []string) ([]Repo, time.Time, bool) {
	var c repoCache
	b, err := os.ReadFile(config.CachePath())
	if err != nil || json.Unmarshal(b, &c) != nil || c.SchemaVersion != 2 || !slices.Equal(c.Orgs, orgs) {
		return nil, time.Time{}, false
	}
	repos := slices.Clone(c.Repos)
	SortReposByActivity(repos)
	return repos, c.FetchedAt, true
}

// RefreshRepos fetches configured owners concurrently. Successful results are
// merged immediately; failures retain that owner's cached repositories. Calling
// it again (or RefreshOwnerRepos directly) provides owner-level retry.
func RefreshRepos(ctx context.Context, client *http.Client, token string, orgs []string, cached []Repo) <-chan RefreshMsg {
	// This is the maximum possible event count. A full buffer lets the
	// coordinator and workers terminate even if the consumer stops reading.
	out := make(chan RefreshMsg, 2*len(orgs)+1)
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
			out <- RefreshMsg{Owner: owner, State: RefreshStarted}
			go func() {
				repos, err := RefreshOwnerRepos(ctx, client, token, owner)
				results <- result{owner: owner, repos: repos, err: err}
			}()
		}

		merged := slices.Clone(cached)
		for range orgs {
			r := <-results
			if r.err != nil {
				out <- RefreshMsg{Owner: r.owner, State: RefreshFailed, Err: r.err}
				continue
			}
			merged = ReplaceOwnerRepos(merged, r.owner, r.repos)
			SortReposByActivity(merged)
			out <- RefreshMsg{Owner: r.owner, State: RefreshSucceeded, Repos: slices.Clone(merged)}
		}
		SortReposByActivity(merged)
		if ctx.Err() == nil {
			WriteRepoCache(orgs, merged)
		}
		out <- RefreshMsg{State: RefreshComplete, Repos: merged}
	}()
	return out
}

func ReplaceOwnerRepos(repos []Repo, owner string, replacement []Repo) []Repo {
	merged := make([]Repo, 0, len(repos)+len(replacement))
	for _, repo := range repos {
		if !strings.EqualFold(repo.Org, owner) {
			merged = append(merged, repo)
		}
	}
	return append(merged, replacement...)
}

func WriteRepoCache(orgs []string, repos []Repo) {
	_ = os.MkdirAll(filepath.Dir(config.CachePath()), 0o755)
	b, err := json.MarshalIndent(repoCache{SchemaVersion: 2, FetchedAt: time.Now(), Orgs: orgs, Repos: repos}, "", "  ")
	if err == nil {
		_ = os.WriteFile(config.CachePath(), b, 0o644) // cache write failure is not fatal
	}
}

// token: gh auth token → GITHUB_TOKEN → error. gh owns keychain/hosts.yml resolution.
func Token() (string, error) {
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

func RefreshOwnerRepos(ctx context.Context, client *http.Client, token, owner string) ([]Repo, error) {
	login := ""
	return fetchOwnerReposContext(ctx, client, token, owner, &login)
}

// FetchRepo resolves a single owner/repo directly, so an ad-hoc launch can name
// any repository the token can see — including ones outside the configured
// owners the picker lists.
func FetchRepo(ctx context.Context, client *http.Client, token, owner, name string) (Repo, error) {
	var payload struct {
		Name          string    `json:"name"`
		SSHURL        string    `json:"ssh_url"`
		DefaultBranch string    `json:"default_branch"`
		PushedAt      time.Time `json:"pushed_at"`
		Owner         struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	endpoint := githubAPIBase + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	if err := githubJSONContext(ctx, client, token, endpoint, &payload); err != nil {
		return Repo{}, fmt.Errorf("github: fetching %s/%s: %w", owner, name, err)
	}
	// Prefer GitHub's canonical owner casing; fall back to what the caller typed.
	org := payload.Owner.Login
	if org == "" {
		org = owner
	}
	return Repo{Name: payload.Name, Org: org, SSHURL: payload.SSHURL, DefaultBranch: payload.DefaultBranch, PushedAt: payload.PushedAt}, nil
}

func SortReposByActivity(repos []Repo) {
	sort.SliceStable(repos, func(i, j int) bool {
		if !repos[i].PushedAt.Equal(repos[j].PushedAt) {
			return repos[i].PushedAt.After(repos[j].PushedAt)
		}
		return repos[i].ID() < repos[j].ID()
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

func (r Repo) ID() string { return r.Org + "/" + r.Name }
