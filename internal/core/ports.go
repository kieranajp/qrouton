package core

import (
	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// AgentRequest asks for one session's supervisor. An empty RunnerID means the
// workbench's own agent.
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

// Launcher builds the commands the core runs on the host. An empty Shell
// command reports ErrNoShellCommand.
type Launcher interface {
	Agent(AgentRequest) (AgentCommand, error)
	Shell(sessionRoot string) []string
	Document(sessionRoot, name string) (workbench.WindowOptions, error)
	Runners() ([]assembly.Runner, error)
	Signal(sessionRoot string)
}
