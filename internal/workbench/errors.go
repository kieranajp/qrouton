package workbench

import "errors"

var (
	ErrHandleIncomplete = errors.New("session handle missing socket or session root")

	// ErrWindowIDUnavailable keeps an unaddressable window out of a caller's
	// registry.
	ErrWindowIDUnavailable = errors.New("workbench returned no window id")

	// ErrAddReposUnanswered is a workbench that reported neither an outcome nor a
	// failure, which must not read as an add that changed nothing.
	ErrAddReposUnanswered = errors.New("workbench returned no repository outcome")

	ErrWorkbenchUnreachable     = errors.New("workbench control socket unreachable")
	ErrInvalidProcessDescriptor = errors.New("invalid active-workbench descriptor")
)
