package status

import "time"

// The strip's copy: mode and phase labels, the chords each mode advertises,
// and the separators that lay the single line out.

const (
	// refreshInterval bounds how stale the strip can be after an escalation.
	refreshInterval = 2 * time.Second

	manifestUnavailable = "no session manifest"

	modeAssistantLabel = "ASSISTANT"
	modeRPILabel       = "RPI"

	phaseScratch   = "scratch"
	phaseResearch  = "Research"
	phasePlan      = "Plan"
	phaseImplement = "Implement"

	// Only chords that exist are advertised.
	assistantChords = "Alt-e escalate · Alt-g terminal · Alt-? keys"
	rpiChords       = "Alt-n de-escalate · Alt-g terminal · Alt-? keys"

	labelSeparator = " · "
	fieldSeparator = "   "
)
