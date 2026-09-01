package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// addReposTimeout bounds the whole add, cloning included. The socket falls back to
// a much shorter timeout only when the caller sets no deadline, so a first clone
// needs this said explicitly or it is cut off partway.
const addReposTimeout = 15 * time.Minute

type addReposInput struct {
	Repos []repoAdditionInput `json:"repos" jsonschema:"Repositories to add to this session's workspace"`
}

type repoAdditionInput struct {
	Name string `json:"name" jsonschema:"Repository as org/name, or a bare name when it is unambiguous across the configured owners"`
	Role string `json:"role,omitempty" jsonschema:"Either editing (on the session branch, changes allowed) or reference (detached at a pinned commit, read-only). Defaults to reference"`
}

// repoEntry is one manifest repository as the agent sees it. Role is always
// spelled out, so a manifest written before the field existed still reads as the
// editing repo it is.
type repoEntry struct {
	Name          string `json:"name"`
	Org           string `json:"org"`
	Role          string `json:"role"`
	Branch        string `json:"branch,omitempty"`
	Revision      string `json:"revision,omitempty"`
	WorktreePath  string `json:"worktree_path"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// repoListing is what list_repos answers with: one line for the agent to read,
// and the same facts structured.
type repoListing struct {
	Message string
	Repos   []repoEntry
	Branch  string
}

func listRepos(root string) (repoListing, error) {
	manifest, err := session.Load(root)
	if err != nil {
		return repoListing{}, fmt.Errorf("%w: %w", ErrManifestUnreadable, err)
	}
	entries := make([]repoEntry, 0, len(manifest.Repos))
	lines := make([]string, 0, len(manifest.Repos))
	for _, r := range manifest.Repos {
		entry := repoEntry{
			Name:          r.Name,
			Org:           r.Org,
			Role:          string(r.Role.Effective()),
			Branch:        r.Branch,
			Revision:      r.Revision,
			WorktreePath:  r.WorktreePath,
			DefaultBranch: r.DefaultBranch,
		}
		entries = append(entries, entry)
		lines = append(lines, repoLine(entry))
	}
	listing := repoListing{Repos: entries, Branch: manifest.Branch(), Message: noReposHeld}
	if len(entries) > 0 {
		listing.Message = reposHeldPrefix + strings.Join(lines, repoLineJoiner)
	}
	return listing, nil
}

func repoLine(entry repoEntry) string {
	where := entry.Revision
	if entry.Branch != "" {
		where = entry.Branch
	}
	if where == "" {
		where = unknownRepoPosition
	}
	return fmt.Sprintf(repoLineFormat, entry.Org, entry.Name, entry.Role, where, entry.WorktreePath)
}

// addRepos composes the named repositories into this session and blocks until
// they are on disk. The user watches a tab meanwhile; the same tab is replaced
// with the outcome, so a name reused twice reads as one place to look.
func (m *windowManager) addRepos(ctx context.Context, input addReposInput) (string, error) {
	requested := make([]workbench.RepoAddition, 0, len(input.Repos))
	for _, repo := range input.Repos {
		if strings.TrimSpace(repo.Name) == "" {
			return "", ErrRepoNameRequired
		}
		requested = append(requested, workbench.RepoAddition{Name: repo.Name, Role: repo.Role})
	}
	if len(requested) == 0 {
		return "", ErrReposRequired
	}

	names := make([]string, 0, len(requested))
	for _, repo := range requested {
		names = append(names, repo.Name)
	}
	m.reportRepos(ctx, fmt.Sprintf(addingReposFormat, strings.Join(names, repoListJoiner)))

	deadlined, cancel := context.WithTimeout(ctx, addReposTimeout)
	defer cancel()
	result, err := m.host.AddRepos(deadlined, workbench.AddReposRequest{Repos: requested})
	if err != nil {
		m.reportRepos(ctx, fmt.Sprintf(addReposFailedFormat, err))
		return "", fmt.Errorf("add repos: %w", err)
	}
	outcome := addReposOutcome(result)
	m.reportRepos(ctx, outcome)
	return outcome, nil
}

// reportRepos replaces the repos tab. A tab that cannot be drawn is not worth
// failing an add that already happened.
func (m *windowManager) reportRepos(ctx context.Context, text string) {
	_, _ = m.open(ctx, reposWindowName, workbench.WindowOptions{
		Kind:    workbench.KindDocument,
		Label:   reposWindowLabel,
		Content: text,
	})
}

func addReposOutcome(result workbench.AddReposResult) string {
	var parts []string
	say := func(format string, names []string) {
		if len(names) > 0 {
			parts = append(parts, fmt.Sprintf(format, strings.Join(names, repoListJoiner)))
		}
	}
	say(addedReposFormat, result.Added)
	say(promotedReposFormat, result.Promoted)
	say(heldReposFormat, result.Held)
	if len(parts) == 0 {
		return noReposChanged
	}
	return strings.Join(parts, repoOutcomeJoiner)
}
