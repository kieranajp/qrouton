package github

import (
	"context"
	"encoding/json"
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

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/config"
)

var githubAPIBase = apiBaseDefault

type Repo struct {
	Name          string    `json:"name"`
	Org           string    `json:"org"`
	SSHURL        string    `json:"ssh_url"`
	DefaultBranch string    `json:"default_branch"`
	PushedAt      time.Time `json:"pushed_at"`
}

// OwnerRepos is one owner's cached repositories and the moment they were
// fetched. Each owner carries its own timestamp so an owner qrouton failed to
// reach never presents as freshly fetched.
type OwnerRepos struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Repos     []Repo    `json:"repos"`
}

type repoCache struct {
	SchemaVersion int                   `json:"schemaVersion"`
	Orgs          []string              `json:"orgs"`
	Owners        map[string]OwnerRepos `json:"owners"`
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
// when stale. The bool reports whether a matching cache exists. The timestamp is
// the oldest owner's, which is how fresh the whole list is; an owner the cache
// has never held counts as never fetched.
func CachedRepos(orgs []string) ([]Repo, time.Time, bool) {
	owners, ok := CachedOwnerRepos(orgs)
	if !ok {
		return nil, time.Time{}, false
	}
	var repos []Repo
	var oldest time.Time
	for i, owner := range orgs {
		entry := owners[cacheKey(owner)]
		repos = append(repos, entry.Repos...)
		if i == 0 || entry.FetchedAt.Before(oldest) {
			oldest = entry.FetchedAt
		}
	}
	SortReposByActivity(repos)
	return repos, oldest, true
}

// CachedOwnerRepos answers per owner, so a caller can say which owners are stale
// and by how much. Keys are lowercased owners; an owner the cache has never held
// is absent.
func CachedOwnerRepos(orgs []string) (map[string]OwnerRepos, bool) {
	var c repoCache
	b, err := os.ReadFile(config.CachePath())
	if err != nil || json.Unmarshal(b, &c) != nil || c.SchemaVersion != cacheSchemaVersion || !slices.Equal(c.Orgs, orgs) {
		return nil, false
	}
	owners := make(map[string]OwnerRepos, len(c.Owners))
	for owner, entry := range c.Owners {
		owners[cacheKey(owner)] = entry
	}
	return owners, true
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
			out <- RefreshMsg{Owner: owner, State: RefreshStarted}
			go func() {
				repos, err := RefreshOwnerRepos(ctx, client, token, owner)
				results <- result{owner: owner, repos: repos, err: err}
			}()
		}

		merged := OwnedBy(cached, orgs)
		fresh := map[string]bool{}
		for range orgs {
			r := <-results
			if r.err != nil {
				out <- RefreshMsg{Owner: r.owner, State: RefreshFailed, Err: r.err}
				continue
			}
			merged = ReplaceOwnerRepos(merged, r.owner, r.repos)
			SortReposByActivity(merged)
			fresh[cacheKey(r.owner)] = true
			out <- RefreshMsg{Owner: r.owner, State: RefreshSucceeded, Repos: slices.Clone(merged)}
		}
		var err error
		if ctx.Err() == nil && len(fresh) > 0 {
			err = writeOwnerCaches(orgs, merged, fresh)
		}
		out <- RefreshMsg{State: RefreshComplete, Repos: merged, Err: err}
	}()
	return out
}

func OwnedBy(repos []Repo, owners []string) []Repo {
	kept := make([]Repo, 0, len(repos))
	for _, repo := range repos {
		if slices.ContainsFunc(owners, func(owner string) bool { return strings.EqualFold(owner, repo.Org) }) {
			kept = append(kept, repo)
		}
	}
	return kept
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

// WriteRepoCache records repos as the current result for every listed owner, so
// each one is stamped as fetched now.
func WriteRepoCache(orgs []string, repos []Repo) error {
	fresh := make(map[string]bool, len(orgs))
	for _, owner := range orgs {
		fresh[cacheKey(owner)] = true
	}
	return writeOwnerCaches(orgs, repos, fresh)
}

// writeOwnerCaches stamps the owners in fresh as fetched now and leaves every
// other owner's timestamp at whatever the cache already held, so an owner that
// could not be reached keeps saying how old its list really is.
func writeOwnerCaches(orgs []string, repos []Repo, fresh map[string]bool) error {
	previous, _ := CachedOwnerRepos(orgs)
	now := time.Now()
	owners := make(map[string]OwnerRepos, len(orgs))
	for _, owner := range orgs {
		key := cacheKey(owner)
		at := previous[key].FetchedAt
		if fresh[key] {
			at = now
		}
		owners[key] = OwnerRepos{FetchedAt: at, Repos: ownerRepos(repos, owner)}
	}
	if err := os.MkdirAll(filepath.Dir(config.CachePath()), cacheDirMode); err != nil {
		return err
	}
	b, err := json.MarshalIndent(repoCache{SchemaVersion: cacheSchemaVersion, Orgs: orgs, Owners: owners}, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Replace(config.CachePath(), b, cacheFileMode)
}

func ownerRepos(repos []Repo, owner string) []Repo {
	var out []Repo
	for _, repo := range repos {
		if strings.EqualFold(repo.Org, owner) {
			out = append(out, repo)
		}
	}
	return out
}

func cacheKey(owner string) string { return strings.ToLower(owner) }

// Token resolves credentials: gh auth token, then the environment.
func Token() (string, error) {
	if out, err := exec.Command(ghBin, ghAuthCmd, ghTokenCmd).Output(); err == nil {
		if t := strings.TrimSpace(string(out)); t != "" {
			return t, nil
		}
	}
	if t := os.Getenv(tokenEnvVar); t != "" {
		return t, nil
	}
	return "", ErrNoToken
}

// AuthenticatedLogin is the account the token belongs to, which is how a screen
// naming the signed-in user says whose repositories it can see.
func AuthenticatedLogin(ctx context.Context, client *http.Client, token string) (string, error) {
	var me struct {
		Login string `json:"login"`
	}
	if err := githubJSON(ctx, client, token, githubAPIBase+userPath, &me); err != nil {
		return "", fmt.Errorf("github: identifying authenticated user: %w", err)
	}
	return me.Login, nil
}

func RefreshOwnerRepos(ctx context.Context, client *http.Client, token, owner string) ([]Repo, error) {
	login := ""
	return fetchOwnerRepos(ctx, client, token, owner, &login)
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
	endpoint := githubAPIBase + reposPath + repoIDSeparator + url.PathEscape(owner) + repoIDSeparator + url.PathEscape(name)
	if err := githubJSON(ctx, client, token, endpoint, &payload); err != nil {
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

func fetchOwnerRepos(ctx context.Context, client *http.Client, token, owner string, authenticatedLogin *string) ([]Repo, error) {
	var identity struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	if err := githubJSON(ctx, client, token, githubAPIBase+usersPath+url.PathEscape(owner), &identity); err != nil {
		return nil, fmt.Errorf("github: identifying %s: %w", owner, err)
	}

	var endpoint string
	switch identity.Type {
	case ownerTypeOrganization:
		endpoint = githubAPIBase + orgsPath + url.PathEscape(owner) + reposPath + orgReposQuery
	case ownerTypeUser:
		if *authenticatedLogin == "" {
			login, err := AuthenticatedLogin(ctx, client, token)
			if err != nil {
				return nil, err
			}
			*authenticatedLogin = login
		}
		if strings.EqualFold(owner, *authenticatedLogin) {
			endpoint = githubAPIBase + userReposQuery
		} else {
			endpoint = githubAPIBase + usersPath + url.PathEscape(owner) + reposPath + otherUserQuery
		}
	default:
		return nil, unsupportedOwnerType(identity.Type, owner)
	}

	var repos []Repo
	for page := 1; ; page++ {
		separator := querySeparator
		if !strings.Contains(endpoint, queryStart) {
			separator = queryStart
		}
		var batch []Repo
		requestURL := fmt.Sprintf(paginationQuery, endpoint, separator, pageSize, page)
		if err := githubJSON(ctx, client, token, requestURL, &batch); err != nil {
			return nil, fmt.Errorf("github: listing %s repos (page %d): %w", owner, page, err)
		}
		for i := range batch {
			batch[i].Org = owner
		}
		repos = append(repos, batch...)
		if len(batch) < pageSize {
			break
		}
	}
	return repos, nil
}

func githubJSON(ctx context.Context, client *http.Client, token, requestURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, getMethod, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set(authorizationHeader, bearerPrefix+token)
	req.Header.Set(acceptHeader, acceptGitHubJSON)
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

func (r Repo) ID() string { return r.Org + repoIDSeparator + r.Name }
