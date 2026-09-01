package assembly

import (
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/session"
)

func repositoryNotice(before, after session.Manifest) string {
	held := make(map[string]session.ManifestRepo, len(before.Repos))
	for _, repo := range before.Repos {
		held[repositoryKey(repo)] = repo
	}
	changes := make([]string, 0, len(after.Repos))
	for _, repo := range after.Repos {
		previous, ok := held[repositoryKey(repo)]
		switch {
		case !ok && repo.Role.Effective() == session.RepoRoleReference:
			changes = append(changes, fmt.Sprintf(repositoryNoticeAddedReference, repositoryID(repo), repo.WorktreePath))
		case !ok:
			changes = append(changes, fmt.Sprintf(repositoryNoticeAddedEditing, repositoryID(repo), repo.WorktreePath))
		case previous.Role.Effective() == session.RepoRoleReference && repo.Role.Effective() == session.RepoRoleEditing:
			changes = append(changes, fmt.Sprintf(repositoryNoticePromotedEditing, repositoryID(repo), repo.WorktreePath))
		}
	}
	if len(changes) == 0 {
		return ""
	}
	return fmt.Sprintf(repositoryNoticeFormat, strings.Join(changes, repositoryNoticeSeparator))
}

func repositoryKey(repo session.ManifestRepo) string {
	return strings.ToLower(repositoryID(repo))
}

func repositoryID(repo session.ManifestRepo) string {
	return repo.Org + "/" + repo.Name
}
