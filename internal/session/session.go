package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

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

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slugify(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), slugSeparator), slugSeparator)
}

func SessionSlug(name, entropy string) string {
	base := Slugify(name)
	suffix := Slugify(entropy)
	if base == "" || suffix == "" {
		return base
	}
	return base + slugSeparator + suffix
}

func NewEntropy() string {
	b := make([]byte, sessionEntropyBytes)
	_, _ = rand.Read(b) // crypto/rand never fails
	return hex.EncodeToString(b)
}

// CreateRequest is the work a session is being assembled for. Only Name is
// required: a scratch session has no repositories, no ticket and no prefix.
type CreateRequest struct {
	Name        string
	Slug        string
	Description string
	// Ticket is the external issue this work came from, if any.
	Ticket string
	// Prefix is the branch prefix editing repositories are cut with.
	Prefix string
	// Mode selects the runner's starting system prompt.
	Mode SessionMode
	// Runner is the coding agent this session is assembled with, so every later
	// boot starts the one that was chosen. Empty takes the workbench's own.
	Runner string
	Repos  []RepoSelection
	// InitialPrompt is an external request the first runner launch receives. It
	// is carried in a private one-shot file rather than becoming durable
	// manifest state, so the runner consumes it exactly once.
	InitialPrompt string
}

// Create is the role-aware assembly entry point. Progress reports the start and
// outcome of each real mirror, worktree, scaffold, and manifest operation.
func Create(cfg *config.Config, req CreateRequest, progress ProgressFunc) (string, error) {
	slug := Slugify(req.Slug)
	if slug == "" {
		slug = Slugify(req.Name)
	}
	dir := filepath.Join(cfg.Root, slug)
	if err := os.Mkdir(dir, dirMode); err != nil {
		// Reclaim only directories our own interrupted assembly left behind —
		// never a user's directory that merely shares the slug.
		if !os.IsExist(err) || !Abandoned(dir) {
			return "", err
		}
		if err := os.RemoveAll(dir); err != nil {
			return "", err
		}
		if err := os.Mkdir(dir, dirMode); err != nil {
			return "", err
		}
	}
	manifestComplete := false
	defer func() {
		if !manifestComplete {
			_ = os.RemoveAll(dir)
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, assemblingMarker), nil, fileMode); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.InitialPrompt) != "" {
		if err := os.MkdirAll(sessionpaths.Dir(dir), dirMode); err != nil {
			return "", err
		}
		if err := os.WriteFile(sessionpaths.InitialPrompt(dir), []byte(req.InitialPrompt), privateFileMode); err != nil {
			return "", err
		}
	}

	m := Manifest{SchemaVersion: manifestSchemaVersion, Name: req.Name, Slug: slug, Description: req.Description,
		TicketURL: req.Ticket, Mode: req.Mode.effective(), Runner: req.Runner, CreatedAt: time.Now()}
	var err error
	if m, err = ComposeRepos(cfg, m, req.Repos, req.Prefix+branchSeparator+slug, progress); err != nil {
		return "", err
	}

	steps := reporter{fn: progress}
	home := thoughtsHome(cfg.Root, slug)
	if err := steps.step(ProgressScaffold, func(func(string, int)) error {
		for _, d := range scaffoldDirs {
			if err := os.MkdirAll(filepath.Join(home, sessionpaths.SharedDirName, d), dirMode); err != nil {
				return err
			}
		}
		return os.Symlink(filepath.Join("..", sessionpaths.ThoughtsDirName, slug),
			filepath.Join(dir, sessionpaths.ThoughtsDirName))
	}); err != nil {
		return "", err
	}

	if err := steps.step(ProgressManifest, func(func(string, int)) error {
		if err := WriteManifest(dir, m); err != nil {
			return err
		}
		manifestComplete = true
		_ = os.Remove(filepath.Join(dir, assemblingMarker))
		return nil
	}); err != nil {
		return "", err
	}
	return dir, nil
}

// thoughtsHome is where a session's documents actually live: under the root,
// outside the session directory, so deleting the session keeps them.
func thoughtsHome(root, slug string) string {
	return filepath.Join(root, sessionpaths.ThoughtsDirName, slug)
}

func materialise(cfg *config.Config, dir string, sel RepoSelection, branch, worktreePath string, progress ProgressFunc) (ManifestRepo, error) {
	r := sel.Repo
	role := sel.Role.Effective()
	if role != RepoRoleEditing && role != RepoRoleReference {
		return ManifestRepo{}, invalidRole(role, r.Org, r.Name)
	}
	url := sshURL(r.Org, r)
	rep := reporter{fn: progress, repo: &r, role: role}
	if err := rep.step(ProgressMirror, func(advance func(string, int)) error {
		return ensureMirror(cfg.Root, r.Org, r.Name, url, advance)
	}); err != nil {
		return ManifestRepo{}, err
	}
	mr := ManifestRepo{Name: r.Name, Org: r.Org, Role: role,
		DefaultBranch: r.DefaultBranch, WorktreePath: worktreePath, SSHURL: url}
	mirror := mirrorPath(cfg.Root, r.Org, r.Name)
	err := rep.step(ProgressWorktree, func(func(string, int)) error {
		if role != RepoRoleReference {
			mr.Branch = branch
			return addWorktree(mirror, filepath.Join(dir, worktreePath), branch,
				remoteRefPrefix+r.DefaultBranch)
		}
		revision, err := resolveRevision(mirror, remoteRefPrefix+r.DefaultBranch)
		if err != nil {
			return err
		}
		mr.Revision = revision
		return addDetachedWorktree(mirror, filepath.Join(dir, worktreePath), revision)
	})
	if err != nil {
		return ManifestRepo{}, err
	}
	return mr, nil
}

// ComposeRepos materialises the selected repositories into m — mirrors,
// worktrees, manifest entries — without writing the manifest, so a caller can
// fold repos, mode, and an escalation outcome into one atomic write. branch is
// the session branch editing repositories are cut on.
func ComposeRepos(cfg *config.Config, m Manifest, sels []RepoSelection, branch string, progress ProgressFunc) (Manifest, error) {
	dir := filepath.Join(cfg.Root, m.Slug)
	if err := os.MkdirAll(sessionpaths.Src(dir), dirMode); err != nil {
		return m, err
	}
	nameCounts := make(map[string]int, len(m.Repos)+len(sels))
	for _, r := range m.Repos {
		nameCounts[r.Name]++
	}
	for _, sel := range sels {
		nameCounts[sel.Repo.Name]++
	}
	for _, sel := range sels {
		// A repository already in the session is left exactly as it stands —
		// same worktree, same branch, same uncommitted work. Escalating a
		// session that has been worked in must not disturb what it was working
		// on, and the collision handling below cannot tell a repository from
		// itself: it counts names, so it would dutifully org-qualify a second
		// checkout of the same repo and clone it alongside the first, on a
		// different branch.
		if hasRepo(m.Repos, sel.Repo.Org, sel.Repo.Name) {
			continue
		}
		worktreePath := filepath.Join(sessionpaths.SrcDirName, sel.Repo.Name)
		if nameCounts[sel.Repo.Name] > 1 {
			worktreePath = filepath.Join(sessionpaths.SrcDirName, Slugify(sel.Repo.Org+slugSeparator+sel.Repo.Name))
		}
		mr, err := materialise(cfg, dir, sel, branch, worktreePath, progress)
		if err != nil {
			return m, err
		}
		m.Repos = append(m.Repos, mr)
	}
	return m, nil
}

func MergeRepos(m Manifest, repos []ManifestRepo) Manifest {
	for _, repo := range repos {
		if !hasRepo(m.Repos, repo.Org, repo.Name) {
			m.Repos = append(m.Repos, repo)
		}
	}
	return m
}

func hasRepo(repos []ManifestRepo, org, name string) bool {
	return indexOfRepo(repos, org, name) >= 0
}

func resolveRevision(mirror, ref string) (string, error) {
	out, err := exec.Command(gitBin, dirFlag, mirror, revParseCmd, verifyFlag, ref+commitRefSuffix).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w\n%s", ref, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func addDetachedWorktree(mirror, path, revision string) error {
	if err := git(dirFlag, mirror, worktreeCmd, worktreePrune); err != nil {
		return err
	}
	return git(dirFlag, mirror, worktreeCmd, worktreeAdd, detachFlag, path, revision)
}

func sshURL(org string, r github.Repo) string {
	if r.SSHURL != "" {
		return r.SSHURL
	}
	return fmt.Sprintf(sshURLFormat, org, r.Name)
}
