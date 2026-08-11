// Package status derives the session identity the workbench chrome shows —
// which mode the session is in, which macro-phase it has reached, and what the
// work is called. It only reads the manifest, so the picker's escalation write
// shows up on the next read: that is the escalation confirmation.
package status

import (
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/session"
)

// Fields is the session identity the workbench chrome shows.
type Fields struct {
	Mode     string
	Phase    string
	Identity string
}

// Read reports the session's identity, or false when the manifest cannot be
// loaded — the caller decides what to render in its place.
func Read(root string) (Fields, bool) {
	m, err := session.Load(root)
	if err != nil {
		return Fields{}, false
	}
	fields := Fields{Mode: modeAssistantLabel, Phase: phase(root, m), Identity: m.Slug}
	if m.EffectiveMode() == session.ModeRPI {
		fields.Mode, fields.Identity = modeRPILabel, m.Name
		if branch := activeBranch(m); branch != "" {
			fields.Identity += labelSeparator + branch
		}
	}
	return fields, true
}

// phase is the macro-phase only: scratch until repositories exist, then the
// stage the session's durable documents put it in — a research doc means
// planning, a plan means implementing.
func phase(root string, m session.Manifest) string {
	if len(m.Repos) == 0 {
		return phaseScratch
	}
	ws := session.Status(filepath.Dir(root), m)
	switch {
	case ws.Plan:
		return phaseImplement
	case ws.Research:
		return phasePlan
	default:
		return phaseResearch
	}
}

// activeBranch is the session branch: the first active repo's branch.
func activeBranch(m session.Manifest) string {
	for _, r := range m.Repos {
		if r.Branch != "" {
			return r.Branch
		}
	}
	return ""
}
