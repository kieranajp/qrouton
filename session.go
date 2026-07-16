package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const manifestName = "qrouton.json"

const manifestSchemaVersion = 2

type RepoRole string

const (
	RepoRoleActive    RepoRole = "active"
	RepoRoleReference RepoRole = "reference"
)

// RepoSelection pairs repository metadata with its role in a session.
type RepoSelection struct {
	Repo Repo
	Role RepoRole
}

type SessionProgressStep string
type SessionProgressStatus string

const (
	SessionProgressMirror   SessionProgressStep = "mirror"
	SessionProgressWorktree SessionProgressStep = "worktree"
	SessionProgressScaffold SessionProgressStep = "scaffold"
	SessionProgressManifest SessionProgressStep = "manifest"

	SessionProgressStarted   SessionProgressStatus = "started"
	SessionProgressCompleted SessionProgressStatus = "completed"
	SessionProgressFailed    SessionProgressStatus = "failed"
)

type SessionProgress struct {
	Step   SessionProgressStep
	Status SessionProgressStatus
	Repo   *Repo
	Role   RepoRole
	Err    error
}

type SessionProgressFunc func(SessionProgress)

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
	Name          string   `json:"name"`
	Org           string   `json:"org"`
	Role          RepoRole `json:"role,omitempty"`
	Branch        string   `json:"branch,omitempty"`
	DefaultBranch string   `json:"defaultBranch,omitempty"`
	Revision      string   `json:"revision,omitempty"`
	WorktreePath  string   `json:"worktreePath"`
}

func (r ManifestRepo) effectiveRole() RepoRole {
	// Schema 1 did not record roles; every repository had a session branch.
	if r.Role == "" {
		return RepoRoleActive
	}
	return r.Role
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
	selected := make([]RepoSelection, len(repos))
	for i, repo := range repos {
		selected[i] = RepoSelection{Repo: repo, Role: RepoRoleActive}
	}
	return createSessionWithRoles(cfg, name, desc, ticket, prefix, selected)
}

// createSessionWithRoles creates branches only for active repositories and pins
// references to the default-branch revision resolved at creation time.
func createSessionWithRoles(cfg *Config, name, desc, ticket, prefix string, repos []RepoSelection) (string, error) {
	return createSessionWithRolesProgress(cfg, name, desc, ticket, prefix, repos, nil)
}

// createSessionWithRolesProgress is the role-aware assembly entry point. Progress
// reports the start and outcome of each real mirror, worktree, scaffold, and manifest operation.
func createSessionWithRolesProgress(cfg *Config, name, desc, ticket, prefix string, repos []RepoSelection, progress SessionProgressFunc) (string, error) {
	slug := slugify(name)
	dir := filepath.Join(cfg.Root, slug)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", err
	}
	manifestComplete := false
	defer func() {
		if !manifestComplete {
			_ = os.RemoveAll(dir)
		}
	}()

	m := Manifest{SchemaVersion: manifestSchemaVersion, Name: name, Slug: slug, Description: desc,
		TicketURL: ticket, CreatedAt: time.Now()}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		return "", err
	}
	nameCounts := make(map[string]int, len(repos))
	for _, selected := range repos {
		nameCounts[selected.Repo.Name]++
	}
	for _, selected := range repos {
		r := selected.Repo
		role := selected.Role
		if role == "" {
			role = RepoRoleActive
		}
		if role != RepoRoleActive && role != RepoRoleReference {
			return "", fmt.Errorf("invalid role %q for %s/%s", role, r.Org, r.Name)
		}
		worktreePath := filepath.Join("src", r.Name)
		if nameCounts[r.Name] > 1 {
			worktreePath = filepath.Join("src", slugify(r.Org+"-"+r.Name))
		}
		emitProgress(progress, SessionProgress{Step: SessionProgressMirror, Status: SessionProgressStarted, Repo: &r, Role: role})
		if err := ensureMirror(cfg.Root, r.Org, r.Name, sshURL(r.Org, r)); err != nil {
			emitProgress(progress, SessionProgress{Step: SessionProgressMirror, Status: SessionProgressFailed, Repo: &r, Role: role, Err: err})
			return "", err
		}
		emitProgress(progress, SessionProgress{Step: SessionProgressMirror, Status: SessionProgressCompleted, Repo: &r, Role: role})
		mr := ManifestRepo{Name: r.Name, Org: r.Org, Role: role,
			DefaultBranch: r.DefaultBranch, WorktreePath: worktreePath}
		mirror := mirrorPath(cfg.Root, r.Org, r.Name)
		emitProgress(progress, SessionProgress{Step: SessionProgressWorktree, Status: SessionProgressStarted, Repo: &r, Role: role})
		if role == RepoRoleReference {
			revision, err := resolveRevision(mirror, "origin/"+r.DefaultBranch)
			if err != nil {
				emitProgress(progress, SessionProgress{Step: SessionProgressWorktree, Status: SessionProgressFailed, Repo: &r, Role: role, Err: err})
				return "", err
			}
			mr.Revision = revision
			if err := addDetachedWorktree(mirror, filepath.Join(dir, worktreePath), revision); err != nil {
				emitProgress(progress, SessionProgress{Step: SessionProgressWorktree, Status: SessionProgressFailed, Repo: &r, Role: role, Err: err})
				return "", err
			}
		} else {
			mr.Branch = prefix + "/" + slug
			if err := addWorktree(mirror, filepath.Join(dir, worktreePath), mr.Branch,
				"origin/"+r.DefaultBranch); err != nil {
				emitProgress(progress, SessionProgress{Step: SessionProgressWorktree, Status: SessionProgressFailed, Repo: &r, Role: role, Err: err})
				return "", err
			}
		}
		emitProgress(progress, SessionProgress{Step: SessionProgressWorktree, Status: SessionProgressCompleted, Repo: &r, Role: role})
		m.Repos = append(m.Repos, mr)
	}

	// doc scaffold the onetech RPI commands expect
	emitProgress(progress, SessionProgress{Step: SessionProgressScaffold, Status: SessionProgressStarted})
	for _, d := range []string{"research", "plans", "specs"} {
		if err := os.MkdirAll(filepath.Join(dir, "thoughts", "shared", d), 0o755); err != nil {
			emitProgress(progress, SessionProgress{Step: SessionProgressScaffold, Status: SessionProgressFailed, Err: err})
			return "", err
		}
	}
	emitProgress(progress, SessionProgress{Step: SessionProgressScaffold, Status: SessionProgressCompleted})

	emitProgress(progress, SessionProgress{Step: SessionProgressManifest, Status: SessionProgressStarted})
	b, err := json.MarshalIndent(m, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, manifestName), b, 0o644)
	}
	if err != nil {
		emitProgress(progress, SessionProgress{Step: SessionProgressManifest, Status: SessionProgressFailed, Err: err})
		return "", err
	}
	manifestComplete = true
	emitProgress(progress, SessionProgress{Step: SessionProgressManifest, Status: SessionProgressCompleted})
	return dir, nil
}

func emitProgress(progress SessionProgressFunc, event SessionProgress) {
	if progress != nil {
		progress(event)
	}
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
		mirror := mirrorPath(cfg.Root, r.Org, r.Name)
		var err error
		if r.effectiveRole() == RepoRoleReference {
			if r.Revision == "" {
				return fmt.Errorf("reference %s/%s has no pinned revision", r.Org, r.Name)
			}
			err = addDetachedWorktree(mirror, wt, r.Revision)
		} else {
			err = addWorktree(mirror, wt, r.Branch, "origin/"+r.DefaultBranch)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveRevision(mirror, ref string) (string, error) {
	out, err := exec.Command("git", "-C", mirror, "rev-parse", "--verify", ref+"^{commit}").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w\n%s", ref, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func addDetachedWorktree(mirror, path, revision string) error {
	if err := git("-C", mirror, "worktree", "prune"); err != nil {
		return err
	}
	return git("-C", mirror, "worktree", "add", "--detach", path, revision)
}

func sshURL(org string, r Repo) string {
	if r.SSHURL != "" {
		return r.SSHURL
	}
	return fmt.Sprintf("git@github.com:%s/%s.git", org, r.Name)
}
