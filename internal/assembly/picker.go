package assembly

import (
	"time"

	"github.com/kieranajp/qrouton/internal/session"
)

// Confirm adds the picked repositories to a live session: the composed
// repositories and the work's details land in one atomic manifest write, after a
// take-up that has already recorded itself. Escalating adds RPI mode and the
// confirmed stanza to that same write, so a polling reader never sees repos added
// while the mode still says assistant.
func (a Assembler) Confirm(dir string, d Draft, escalate bool, progress session.ProgressFunc) error {
	return a.confirm(dir, d, escalate, true, progress)
}

// ConfirmForAgent adds repositories the agent asked for itself, and tells nobody.
// The notice exists to hand an agent facts it does not have; this caller is
// blocked on a tool call that returns those same facts, and signalling would kill
// the process waiting for the reply.
func (a Assembler) ConfirmForAgent(dir string, d Draft, progress session.ProgressFunc) error {
	return a.confirm(dir, d, false, false, progress)
}

func (a Assembler) confirm(dir string, d Draft, escalate, notify bool, progress session.ProgressFunc) error {
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
		if !notify {
			return nil
		}
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

// takeUp re-checks out the repositories the session already reads and records
// them, ahead of any clone and in a write of its own. Both halves matter: a
// refusal leaves the session holding no checkout the manifest never learned
// about, and a clone that then fails cannot leave the file calling a checkout
// pinned that is sitting on the session branch.
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

// Cancel records the cancelled outcome — the stanza alone, mode and
// repositories untouched. Only an escalation has a caller waiting on that
// stanza; the add-repos button's cancel is nobody's business.
func Cancel(dir string, escalate bool) error {
	if !escalate {
		return nil
	}
	return session.UpdateManifest(dir, func(m session.Manifest) (session.Manifest, error) {
		m.Escalation = &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()}
		return m, nil
	})
}
