package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const manifestName = "qrouton.json"

type Manifest struct {
	SchemaVersion int            `json:"schemaVersion"`
	Name          string         `json:"name"`
	Slug          string         `json:"slug"`
	Description   string         `json:"description"`
	TicketURL     string         `json:"ticketUrl,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	Repos         []ManifestRepo `json:"repos"`
}

type ManifestRepo struct {
	Name          string `json:"name"`
	Org           string `json:"org"`
	Branch        string `json:"branch"`
	DefaultBranch string `json:"defaultBranch"`
	WorktreePath  string `json:"worktreePath"`
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// scanSessions: a session is any direct child of root containing a qrouton.json.
func scanSessions(root string) ([]Manifest, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, e.Name(), manifestName))
		if err != nil {
			continue
		}
		var m Manifest
		if json.Unmarshal(b, &m) == nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// createSession assembles mirrors + worktrees, then writes the manifest last —
// a half-assembled dir with no manifest never shows up in resume.
func createSession(cfg *Config, name, desc, ticket, prefix string, repos []Repo) (string, error) {
	slug := slugify(name)
	dir := filepath.Join(cfg.Root, slug)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", err
	}

	m := Manifest{SchemaVersion: 1, Name: name, Slug: slug, Description: desc,
		TicketURL: ticket, CreatedAt: time.Now()}
	for _, r := range repos {
		branch := prefix + "/" + slug
		if err := ensureMirror(cfg.Root, cfg.Org, r.Name, sshURL(cfg.Org, r)); err != nil {
			return "", err
		}
		if err := addWorktree(mirrorPath(cfg.Root, cfg.Org, r.Name),
			filepath.Join(dir, r.Name), branch, "origin/"+r.DefaultBranch); err != nil {
			return "", err
		}
		m.Repos = append(m.Repos, ManifestRepo{Name: r.Name, Org: cfg.Org, Branch: branch,
			DefaultBranch: r.DefaultBranch, WorktreePath: r.Name})
	}

	// doc scaffold the onetech RPI commands expect
	for _, d := range []string{"research", "plans", "specs"} {
		if err := os.MkdirAll(filepath.Join(dir, "thoughts", "shared", d), 0o755); err != nil {
			return "", err
		}
	}

	b, _ := json.MarshalIndent(m, "", "  ")
	return dir, os.WriteFile(filepath.Join(dir, manifestName), b, 0o644)
}

// ensureWorktrees re-materialises any pruned worktrees on resume (fresh fetch either way).
func ensureWorktrees(cfg *Config, m Manifest) error {
	dir := filepath.Join(cfg.Root, m.Slug)
	for _, r := range m.Repos {
		if err := ensureMirror(cfg.Root, r.Org, r.Name,
			fmt.Sprintf("git@github.com:%s/%s.git", r.Org, r.Name)); err != nil {
			return err
		}
		wt := filepath.Join(dir, r.WorktreePath)
		if _, err := os.Stat(wt); err == nil {
			continue
		}
		if err := addWorktree(mirrorPath(cfg.Root, r.Org, r.Name), wt, r.Branch,
			"origin/"+r.DefaultBranch); err != nil {
			return err
		}
	}
	return nil
}

func sshURL(org string, r Repo) string {
	if r.SSHURL != "" {
		return r.SSHURL
	}
	return fmt.Sprintf("git@github.com:%s/%s.git", org, r.Name)
}
