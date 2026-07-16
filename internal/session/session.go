package session

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
)

const manifestName = "qrouton.json"

const manifestSchemaVersion = 2

// assemblingMarker is written first during assembly and removed after the
// manifest lands, so a directory it survives in was abandoned mid-assembly
// (e.g. the process was killed) and is safe to reclaim.
const assemblingMarker = ".qrouton-assembling"

// Abandoned reports whether dir is a half-assembled session directory left
// behind by an interrupted run: it carries the assembly marker but no manifest.
func Abandoned(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, manifestName)); err == nil {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, assemblingMarker))
	return err == nil
}

type RepoRole string

const (
	RepoRoleActive    RepoRole = "active"
	RepoRoleReference RepoRole = "reference"
)

// RepoSelection pairs repository metadata with its role in a session.
type RepoSelection struct {
	Repo github.Repo
	Role RepoRole
}

type ProgressStep string
type ProgressStatus string

const (
	ProgressMirror   ProgressStep = "mirror"
	ProgressWorktree ProgressStep = "worktree"
	ProgressScaffold ProgressStep = "scaffold"
	ProgressManifest ProgressStep = "manifest"

	ProgressStarted   ProgressStatus = "started"
	ProgressCompleted ProgressStatus = "completed"
	ProgressFailed    ProgressStatus = "failed"
)

type Progress struct {
	Step   ProgressStep
	Status ProgressStatus
	Repo   *github.Repo
	Role   RepoRole
	Err    error
}

type ProgressFunc func(Progress)

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
	SSHURL        string   `json:"sshUrl,omitempty"` // clone URL for mirror re-creation on resume
}

func (r ManifestRepo) effectiveRole() RepoRole {
	// Schema 1 did not record roles; every repository had a session branch.
	if r.Role == "" {
		return RepoRoleActive
	}
	return r.Role
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// Scan: a session is any direct child of root containing a qrouton.json.
func Scan(root string) ([]Manifest, error) {
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

// createSessionWithRoles creates branches only for active repositories and pins
// references to the default-branch revision resolved at creation time. It writes the
// manifest last, so a half-assembled directory without one never shows up in resume.
func createSessionWithRoles(cfg *config.Config, name, desc, ticket, prefix string, repos []RepoSelection) (string, error) {
	return Create(cfg, name, desc, ticket, prefix, repos, nil)
}

// Create is the role-aware assembly entry point. Progress
// reports the start and outcome of each real mirror, worktree, scaffold, and manifest operation.
func Create(cfg *config.Config, name, desc, ticket, prefix string, repos []RepoSelection, progress ProgressFunc) (string, error) {
	slug := Slugify(name)
	dir := filepath.Join(cfg.Root, slug)
	if err := os.Mkdir(dir, 0o755); err != nil {
		// Reclaim only directories our own interrupted assembly left behind —
		// never a user's directory that merely shares the slug.
		if !os.IsExist(err) || !Abandoned(dir) {
			return "", err
		}
		if err := os.RemoveAll(dir); err != nil {
			return "", err
		}
		if err := os.Mkdir(dir, 0o755); err != nil {
			return "", err
		}
	}
	manifestComplete := false
	defer func() {
		if !manifestComplete {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, assemblingMarker), nil, 0o644); err != nil {
		return "", err
	}

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
			worktreePath = filepath.Join("src", Slugify(r.Org+"-"+r.Name))
		}
		url := sshURL(r.Org, r)
		emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressStarted, Repo: &r, Role: role})
		if err := ensureMirror(cfg.Root, r.Org, r.Name, url); err != nil {
			emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
			return "", err
		}
		emitProgress(progress, Progress{Step: ProgressMirror, Status: ProgressCompleted, Repo: &r, Role: role})
		mr := ManifestRepo{Name: r.Name, Org: r.Org, Role: role,
			DefaultBranch: r.DefaultBranch, WorktreePath: worktreePath, SSHURL: url}
		mirror := mirrorPath(cfg.Root, r.Org, r.Name)
		emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressStarted, Repo: &r, Role: role})
		if role == RepoRoleReference {
			revision, err := resolveRevision(mirror, "origin/"+r.DefaultBranch)
			if err != nil {
				emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
				return "", err
			}
			mr.Revision = revision
			if err := addDetachedWorktree(mirror, filepath.Join(dir, worktreePath), revision); err != nil {
				emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
				return "", err
			}
		} else {
			mr.Branch = prefix + "/" + slug
			if err := addWorktree(mirror, filepath.Join(dir, worktreePath), mr.Branch,
				"origin/"+r.DefaultBranch); err != nil {
				emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressFailed, Repo: &r, Role: role, Err: err})
				return "", err
			}
		}
		emitProgress(progress, Progress{Step: ProgressWorktree, Status: ProgressCompleted, Repo: &r, Role: role})
		m.Repos = append(m.Repos, mr)
	}

	// doc scaffold the onetech RPI commands expect
	emitProgress(progress, Progress{Step: ProgressScaffold, Status: ProgressStarted})
	for _, d := range []string{"research", "plans", "specs"} {
		if err := os.MkdirAll(filepath.Join(dir, "thoughts", "shared", d), 0o755); err != nil {
			emitProgress(progress, Progress{Step: ProgressScaffold, Status: ProgressFailed, Err: err})
			return "", err
		}
	}
	emitProgress(progress, Progress{Step: ProgressScaffold, Status: ProgressCompleted})

	emitProgress(progress, Progress{Step: ProgressManifest, Status: ProgressStarted})
	b, err := json.MarshalIndent(m, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, manifestName), b, 0o644)
	}
	if err != nil {
		emitProgress(progress, Progress{Step: ProgressManifest, Status: ProgressFailed, Err: err})
		return "", err
	}
	manifestComplete = true
	_ = os.Remove(filepath.Join(dir, assemblingMarker))
	emitProgress(progress, Progress{Step: ProgressManifest, Status: ProgressCompleted})
	return dir, nil
}

func emitProgress(progress ProgressFunc, event Progress) {
	if progress != nil {
		progress(event)
	}
}

// EnsureWorktrees re-materialises any pruned worktrees on resume (fresh fetch either way).
func EnsureWorktrees(cfg *config.Config, m Manifest) error {
	dir := filepath.Join(cfg.Root, m.Slug)
	for _, r := range m.Repos {
		url := r.SSHURL
		if url == "" {
			// Manifests written before the URL was recorded; assume github.com.
			url = fmt.Sprintf("git@github.com:%s/%s.git", r.Org, r.Name)
		}
		if err := ensureMirror(cfg.Root, r.Org, r.Name, url); err != nil {
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

func sshURL(org string, r github.Repo) string {
	if r.SSHURL != "" {
		return r.SSHURL
	}
	return fmt.Sprintf("git@github.com:%s/%s.git", org, r.Name)
}
