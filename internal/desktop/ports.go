package desktop

import "github.com/kieranajp/qrouton/internal/core"

// Launcher keeps launch out of the desktop dependency graph. The argv builders
// beside the core's are the local client's; an empty one reports its matching
// unsupported-operation error.
type Launcher interface {
	core.Launcher
	Reveal(sessionRoot string) []string
	OpenText(path string) []string
	CopyImage(path string) []string
	CopyFile(path string) []string
}

// Validator answers the settings panel, which saves neither an editor nothing
// resolves nor a launch table launch would refuse. A nil Validator accepts both.
type Validator interface {
	ValidateEditor(argv []string) error
	ValidateLaunch(overrides map[string][]string) error
}

// Relauncher replaces this workbench with one reading the config file afresh,
// returning only once the successor is serving. First run needs it because a
// changed sessions root cannot take effect in a running process; the ticket
// supplier is read after the relaunch owns launch serialization.
type Relauncher interface {
	Relaunch(pending func() (url, prompt string)) error
}
