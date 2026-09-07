package mcpserver

import (
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/session"
)

type repoRow struct {
	Name     string `json:"name"`
	Org      string `json:"org"`
	Role     string `json:"role"`
	Branch   string `json:"branch,omitempty"`
	Revision string `json:"revision,omitempty"`
	Worktree string `json:"worktree"`
}

// sessionRepos reads the manifest fresh on every call, so a repo added or
// promoted since this server started shows up on the next call rather than
// needing a restart.
func sessionRepos(root string) ([]repoRow, error) {
	manifest, err := session.Load(root)
	if err != nil {
		return nil, err
	}
	return reposFrom(manifest), nil
}

func reposFrom(manifest session.Manifest) []repoRow {
	rows := make([]repoRow, len(manifest.Repos))
	for i, r := range manifest.Repos {
		rows[i] = repoRow{
			Name:     r.Name,
			Org:      r.Org,
			Role:     string(r.Role.Effective()),
			Branch:   r.Branch,
			Revision: r.Revision,
			Worktree: r.WorktreePath,
		}
	}
	return rows
}

func reposMessage(rows []repoRow) string {
	if len(rows) == 0 {
		return noRepos
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		ref := r.Branch
		if ref == "" {
			ref = r.Revision
		}
		if ref == "" {
			lines[i] = fmt.Sprintf(repoLineFormat, r.Org, r.Name, r.Role, r.Worktree)
			continue
		}
		lines[i] = fmt.Sprintf(repoLineRefFormat, r.Org, r.Name, r.Role, ref, r.Worktree)
	}
	return fmt.Sprintf(reposHeaderFormat, len(rows), strings.Join(lines, repoLineJoiner))
}
