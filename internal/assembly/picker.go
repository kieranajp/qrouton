package assembly

import (
	"time"

	"github.com/kieranajp/qrouton/internal/session"
)

// Confirm adds the picked repositories to a live session: the composed
// repositories and the work's details land in one atomic manifest write.
// Escalating adds RPI mode and the confirmed stanza to that same write, so a
// polling reader never sees repos added while the mode still says assistant.
func (a Assembler) Confirm(dir string, d Draft, escalate bool, progress session.ProgressFunc) error {
	// Loaded here, not carried in: a picker can sit open for half an hour while
	// the workbench keeps rewriting the manifest underneath it.
	m, err := session.Load(dir)
	if err != nil {
		return err
	}
	composed, err := session.ComposeRepos(a.Cfg, m, d.Repos, branchFor(m, d), progress)
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
