package core

import (
	"context"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// AgentRequest asks for one session's supervisor. An empty RunnerID means the
// workbench's own agent, and an empty ProfileID that agent's default profile.
type AgentRequest struct {
	SessionRoot string
	Socket      string
	RunnerID    string
	ProfileID   string
	Resume      bool
}

// AgentCommand is that supervisor. RunnerID is the agent actually selected,
// which an empty request resolves to.
type AgentCommand struct {
	Argv      []string
	Env       []string
	RunnerID  string
	ProfileID string
}

// Launcher builds the commands the core runs on the host. An empty Shell
// command reports ErrNoShellCommand.
type Launcher interface {
	Agent(AgentRequest) (AgentCommand, error)
	Shell(sessionRoot string) []string
	Document(sessionRoot, name string) (workbench.WindowOptions, error)
	Runners() ([]assembly.Runner, error)
	Signal(sessionRoot string)
	// SignInCommand and ProbeSignIn take the zero profile as the runner's default.
	SignInCommand(runnerID string, profile config.RunnerProfile) ([]string, error)
	ProbeSignIn(ctx context.Context, runnerID string, profile config.RunnerProfile) launch.SignIn
}
