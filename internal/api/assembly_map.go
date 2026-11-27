package api

import (
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
	"github.com/kieranajp/qrouton/internal/github"
)

func repositoryRefresh(e core.RefreshEvent) *qroutonv1.RepositoryRefresh {
	return &qroutonv1.RepositoryRefresh{
		Generation: int32(e.Generation),
		Owner:      e.Owner,
		State:      e.State,
		Repos:      each(e.Repos, repository),
		Error:      e.Error,
	}
}

func repository(r github.Repo) *qroutonv1.Repository {
	return &qroutonv1.Repository{Name: r.Name, Org: r.Org, SshUrl: r.SSHURL, DefaultBranch: r.DefaultBranch, PushedAt: timestamp(r.PushedAt)}
}

func assemblyProgress(e core.ProgressEvent) *qroutonv1.AssemblyProgress {
	return &qroutonv1.AssemblyProgress{
		Session: e.Session,
		Step:    e.Step,
		Status:  e.Status,
		Repo:    e.Repo,
		Role:    e.Role,
		Phase:   e.Phase,
		Percent: int32(e.Percent),
		Error:   e.Error,
	}
}
