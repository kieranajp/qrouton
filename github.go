package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Repo struct {
	Name          string `json:"name"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
}

type repoCache struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Org       string    `json:"org"`
	Repos     []Repo    `json:"repos"`
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

// listRepos returns the org's repos, from cache unless stale (24h) or refresh is set.
func listRepos(org string, refresh bool) ([]Repo, error) {
	var c repoCache
	if b, err := os.ReadFile(cachePath()); err == nil && json.Unmarshal(b, &c) == nil &&
		!refresh && c.Org == org && time.Since(c.FetchedAt) < 24*time.Hour {
		return c.Repos, nil
	}

	token, err := githubToken()
	if err != nil {
		return nil, err
	}
	var repos []Repo
	for page := 1; ; page++ {
		req, _ := http.NewRequest("GET",
			fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&page=%d", org, page), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return nil, fmt.Errorf("github: %s listing %s repos (page %d)", resp.Status, org, page)
		}
		var batch []Repo
		err = json.NewDecoder(resp.Body).Decode(&batch)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		repos = append(repos, batch...)
		if len(batch) < 100 {
			break
		}
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })

	os.MkdirAll(filepath.Dir(cachePath()), 0o755)
	b, _ := json.MarshalIndent(repoCache{FetchedAt: time.Now(), Org: org, Repos: repos}, "", "  ")
	os.WriteFile(cachePath(), b, 0o644) // cache write failure is not fatal
	return repos, nil
}
