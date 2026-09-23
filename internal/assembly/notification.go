package assembly

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kieranajp/qrouton/internal/config"
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

// unroutedNotice warns a session writing to a shared root about added repos
// whose orgs do not map there. The thoughts stay where they are.
func unroutedNotice(cfg *config.Config, before, after session.Manifest) string {
	if after.ThoughtsRoot == "" || after.ThoughtsRoot == config.DefaultThoughtsID {
		return ""
	}
	var unrouted []string
	for _, repo := range after.Repos {
		if !slices.ContainsFunc(before.Repos, func(r session.ManifestRepo) bool { return repositoryKey(r) == repositoryKey(repo) }) &&
			cfg.RouteThoughts([]string{repo.Org}).ID != after.ThoughtsRoot {
			unrouted = append(unrouted, repositoryID(repo))
		}
	}
	if len(unrouted) == 0 {
		return ""
	}
	return fmt.Sprintf(unroutedNoticeFormat, after.ThoughtsRoot, strings.Join(unrouted, repositoryNoticeSeparator))
}

func repositoryKey(repo session.ManifestRepo) string {
	return strings.ToLower(repositoryID(repo))
}

func repositoryID(repo session.ManifestRepo) string {
	return repo.Org + "/" + repo.Name
}
