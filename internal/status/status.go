// Package status draws the session strip — the one-line bottom pane that says
// which mode the session is in, which macro-phase it has reached, and which
// chords operate it. It re-reads the manifest every tick, so the picker's
// escalation write flips the strip within one poll: that repaint is the
// escalation confirmation.
package status

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kieranajp/qrouton/internal/paneui"
	"github.com/kieranajp/qrouton/internal/session"
)

// Status redraws the strip forever (redraws in place; used by the workspace
// layout as a full-width one-row borderless pane).
func Status(root string) error {
	for {
		fmt.Print(paneui.Frame(statusLines(root)))
		time.Sleep(refreshInterval)
	}
}

func statusLines(root string) []string {
	m, err := session.Load(root)
	if err != nil {
		return []string{paneui.Muted(manifestUnavailable)}
	}
	mode, identity, chords := modeAssistantLabel, m.Slug, assistantChords
	if m.EffectiveMode() == session.ModeRPI {
		mode, identity, chords = modeRPILabel, m.Name, rpiChords
		if branch := activeBranch(m); branch != "" {
			identity += labelSeparator + branch
		}
	}
	return []string{paneui.Bold(mode+labelSeparator+phase(root, m)) + fieldSeparator +
		identity + fieldSeparator + paneui.Muted(chords)}
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
