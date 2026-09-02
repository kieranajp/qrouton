package assembly

import (
	"time"

	"github.com/kieranajp/qrouton/internal/session"
)

// Confirm records repositories, work details, mode, and escalation atomically.
func (a Assembler) Confirm(dir string, d Draft, escalate bool, progress session.ProgressFunc) error {
	// Loaded here, not carried in: a picker can sit open for half an hour while
	// the workbench keeps rewriting the manifest underneath it.
	m, err := session.Load(dir)
	if err != nil {
		return err
	}
	branch := branchFor(m, d)
	if err := a.takeUp(dir, d, branch, progress); err != nil {
		return err
	}
	// Composed outside the update: cloning takes minutes, and no other process
	// may be kept off the manifest for them.
	composed, err := session.ComposeRepos(a.Cfg, m, d.Repos, branch, progress)
	if err != nil {
		return err
	}
	var updated session.Manifest
	if err := session.UpdateManifest(dir, func(out session.Manifest) (session.Manifest, error) {
		out = session.MergeRepos(out, composed.Repos)
		out.Name, out.Description, out.TicketURL = d.Name, d.Description, d.Ticket
		if escalate {
			out.Mode = session.ModeRPI
			out.Escalation = &session.EscalationOutcome{Status: session.EscalationConfirmed, At: time.Now()}
		}
		updated = out
		return out, nil
	}); err != nil {
		return err
	}
	if !escalate {
		notice := repositoryNotice(m, updated)
		if notice != "" && session.QueueAgentNotice(dir, notice) == nil && a.Signal != nil {
			a.Signal(dir)
		}
		return nil
	}
	if a.Signal != nil {
		// Best-effort: the supervisor replaces the assistant with a fresh
		// orchestrator; with no supervisor, the mode takes effect next launch.
		a.Signal(dir)
	}
	return nil
}

// takeUp records upgraded checkouts before cloning additions can fail.
func (a Assembler) takeUp(dir string, d Draft, branch string, progress session.ProgressFunc) error {
	if len(d.Upgrades) == 0 {
		return nil
	}
	m, err := session.Load(dir)
	if err != nil {
		return err
	}
	if err := session.UpgradeRepos(a.Cfg, m, d.Upgrades, branch, progress); err != nil {
		return err
	}
	return session.UpdateManifest(dir, func(m session.Manifest) (session.Manifest, error) {
		return session.ApplyUpgrades(m, d.Upgrades, branch)
	})
}

// Cancel records only an escalation outcome; add-repository cancellation needs no record.
func Cancel(dir string, escalate bool) error {
	if !escalate {
		return nil
	}
	return session.UpdateManifest(dir, func(m session.Manifest) (session.Manifest, error) {
		m.Escalation = &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()}
		return m, nil
	})
}
