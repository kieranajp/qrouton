package desktop

import (
	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// AgentRequest is one session's supervisor, asked for: the session to run it in,
// the control socket the workbench will serve that session on, and the agent the
// session was assembled with. An empty RunnerID means the workbench's own.
type AgentRequest struct {
	SessionRoot string
	Socket      string
	RunnerID    string
	Resume      bool
}

// AgentCommand is that supervisor. RunnerID is the agent actually selected,
// which an empty request resolves to.
type AgentCommand struct {
	Argv     []string
	Env      []string
	RunnerID string
}

// Launcher builds what the workbench runs. It is launch's, reached through an
// interface because desktop must not import it: everything desktop imports is
// linked into the workbench, and launch pulls in no webview.
//
// An empty Shell or Reveal argv is a workbench that cannot do that thing, and
// says so through the matching sentinel error.
type Launcher interface {
	Agent(AgentRequest) (AgentCommand, error)
	Shell(sessionRoot string) []string
	Reveal(sessionRoot string) []string
	Document(sessionRoot, name string) (workbench.WindowOptions, error)
	Runners() ([]assembly.Runner, error)
	Signal(sessionRoot string)
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
	Relaunch(linearIssue func() (ticket, prompt string)) error
}
