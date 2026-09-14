package workbench

import "errors"

var (
	ErrWorkbenchRefused          = errors.New("workbench refused the request")
	ErrBugReportUnavailable      = errors.New("workbench returned no valid bug report outcome")
	ErrImageSelectionUnavailable = errors.New("workbench returned no valid image selection")
	ErrHandleIncomplete          = errors.New("session handle missing socket or session root")

	// ErrWindowIDUnavailable keeps an unaddressable window out of a caller's
	// registry.
	ErrWindowIDUnavailable = errors.New("workbench returned no window id")

	ErrWorkbenchUnreachable     = errors.New("workbench control socket unreachable")
	ErrInvalidProcessDescriptor = errors.New("invalid active-workbench descriptor")
)
