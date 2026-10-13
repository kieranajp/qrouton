package mcpserver

import (
	"errors"
	"fmt"
)

// Input errors, returned to the agent so it can correct its own call.
var (
	ErrBugReportInput       = errors.New("report_bug requires a nonblank title and body")
	ErrBugReportUnsupported = errors.New("this workbench does not support bug report review")
	ErrArtefactTitle        = errors.New("title is required")
	ErrCommandRequired      = errors.New("command is required")
	ErrMessageRequired      = errors.New("message is required")
	ErrNameRequired         = errors.New("name is required")
	ErrSummaryRequired      = errors.New("summary is required")
	ErrSummaryStage         = errors.New("stage must be research, plan or implement")
	ErrDigestRequired       = errors.New("digest is required")

	ErrReservedWindowName = fmt.Errorf("%q is reserved for the editor window; pick another name", editorWindowName)
)

func noSuchWindow(name string) error {
	return fmt.Errorf("no open window named %q", name)
}

// windowGone distinguishes a window that has closed from a name that never
// existed, so the agent reopens it rather than treating the name as wrong.
func windowGone(name string) error {
	return fmt.Errorf("window %q is no longer open (the user closed it, or its command finished); open it again if you still need it", name)
}

func summaryTooLong(count int) error {
	return fmt.Errorf("summary is %d characters; the limit is %d, so shorten it and call again", count, summaryLimit)
}

func digestTooLong(count int) error {
	return fmt.Errorf("digest is %d characters; the limit is %d, so shorten it and call again", count, digestLimit)
}
