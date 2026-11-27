package api

import (
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
)

func dashboardStatus(s core.DashboardStatus) *qroutonv1.DashboardStatus {
	sessions := make(map[string]*qroutonv1.DashboardSession, len(s.Sessions))
	for slug, session := range s.Sessions {
		sessions[slug] = &qroutonv1.DashboardSession{
			PullRequests: each(session.PullRequests, pullRequest),
			PullState:    session.PullState,
			Repos:        each(session.Repos, repoStat),
		}
	}
	return &qroutonv1.DashboardStatus{
		Viewer:      s.Viewer,
		CheckedAt:   timestamp(s.CheckedAt),
		Refreshing:  s.Refreshing,
		Failed:      s.Failed,
		SignedOut:   s.SignedOut,
		PausedUntil: timestamp(s.PausedUntil),
		Sessions:    sessions,
	}
}
