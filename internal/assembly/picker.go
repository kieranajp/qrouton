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
	composed, err := session.ComposeRepos(a.Cfg, m, d.Repos, branch, progress)
	if err != nil {
		return err
	}
	out, err := session.Load(dir)
	if err != nil {
		return err
	}
	out = session.MergeRepos(out, composed.Repos)
	out.Name, out.Description, out.TicketURL = d.Name, d.Description, d.Ticket
	if !escalate {
		// Adding repositories is not a mode change, and the running agent reads
		// the manifest for itself — relaunching it would cost the user a
		// conversation to tell it something it can already see.
		return session.WriteManifest(dir, out)
	}
	out.Mode = session.ModeRPI
	out.Escalation = &session.EscalationOutcome{Status: session.EscalationConfirmed, At: time.Now()}
	if err := session.WriteManifest(dir, out); err != nil {
		return err
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
	// Reloaded rather than reused: the checkouts took a while, and the workbench
	// keeps writing the window record underneath.
	if m, err = session.Load(dir); err != nil {
		return err
	}
	if m, err = session.ApplyUpgrades(m, d.Upgrades, branch); err != nil {
		return err
	}
	return session.WriteManifest(dir, m)
}

// Cancel records the cancelled outcome — the stanza alone, mode and
// repositories untouched. Only an escalation has a caller waiting on that
// stanza; the add-repos button's cancel is nobody's business.
func Cancel(dir string, escalate bool) error {
	if !escalate {
		return nil
	}
	out, err := session.Load(dir)
	if err != nil {
		return err
	}
	out.Escalation = &session.EscalationOutcome{Status: session.EscalationCancelled, At: time.Now()}
	return session.WriteManifest(dir, out)
}
