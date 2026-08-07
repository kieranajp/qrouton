package workbench

import "errors"

var (
	ErrHandleIncomplete = errors.New("session handle missing socket or session root")

	// ErrWindowIDUnavailable keeps an unaddressable window out of a caller's
	// registry.
	ErrWindowIDUnavailable = errors.New("workbench returned no window id")

	ErrWorkbenchUnreachable = errors.New("workbench control socket unreachable")
)
