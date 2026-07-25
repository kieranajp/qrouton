package tui

import "errors"

// Sentinel errors the onboarding flow raises itself. Errors that describe the
// launch path (an unavailable or missing runner) come from that package
// instead, so both entry points report them identically.
var (
	errNoRunnerSelected = errors.New("no runner selected")
	errSessionNameEmpty = errors.New("session name is required")
	errNoActiveRepo     = errors.New("include at least one active repository")
)
