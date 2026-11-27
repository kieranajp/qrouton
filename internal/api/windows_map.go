package api

import (
	qroutonv1 "github.com/kieranajp/qrouton/internal/api/gen/qrouton/v1"
	"github.com/kieranajp/qrouton/internal/core"
)

func surfaces(s core.Surfaces) *qroutonv1.Surfaces {
	out := &qroutonv1.Surfaces{Session: s.Session, Selected: s.Selected}
	for _, tab := range s.Tabs {
		out.Tabs = append(out.Tabs, &qroutonv1.Tab{
			Id:       tab.ID,
			Label:    tab.Label,
			Badge:    tab.Badge,
			Icon:     string(tab.Icon),
			Kind:     tab.Kind,
			Status:   tab.Status,
			Pinned:   tab.Pinned,
			Artifact: tab.Artifact,
		})
	}
	return out
}
