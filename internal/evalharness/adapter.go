package evalharness

// Runner adapters: how the harness invokes the Claude and Codex CLIs — MCP
// wiring, session continuation, and prompt delivery. Output parsing lives in
// normalize.go.

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/launch"
)

type Adapter struct {
	Name     string
	Bin      string
	Model    string
	SelfPath string
}

func (a Adapter) Version(ctx context.Context) string {
	output, err := exec.CommandContext(ctx, a.Bin, versionFlag).CombinedOutput()
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(string(output))
}

func (a Adapter) RunTurn(
	ctx context.Context,
	workspace string,
	mcpLog string,
	prompt string,
	session string,
	turn int,
) ([]Event, string, string, error) {
	args, err := a.args(workspace, mcpLog, session)
	if err != nil {
		return nil, "", session, err
	}

	cmd := exec.CommandContext(ctx, a.Bin, args...)
	cmd.Dir = workspace
	// The prompt travels over stdin, not argv: judge prompts embed candidate
	// artifacts and diffs, and a single exec argument caps out at ~128KiB on
	// Linux (MAX_ARG_STRLEN), which large runs would exceed.
	cmd.Stdin = strings.NewReader(prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()
	events, final, newSession, parseErr := Normalize(a.Name, stdout.Bytes(), turn)
	if newSession == "" {
		newSession = session
	}
	if runErr != nil {
		return events, final, newSession, fmt.Errorf(
			"%s: %w: %s",
			a.Name,
			runErr,
			strings.TrimSpace(stderr.String()),
		)
	}
	if parseErr != nil {
		return events, final, newSession, parseErr
	}

	events = append(events, Event{
		Time: time.Now().UTC(),
		Kind: kindDuration,
		Turn: turn,
		Text: time.Since(started).String(),
	})
	return events, final, newSession, nil
}

func (a Adapter) args(workspace, mcpLog, session string) ([]string, error) {
	if a.SelfPath == "" {
		return nil, ErrNoSelfPath
	}

	switch a.Name {
	case runnerClaude:
		return a.claudeArgs(workspace, mcpLog, session)
	case runnerCodex:
		return a.codexArgs(workspace, mcpLog, session)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownRunner, a.Name)
	}
}

func (a Adapter) claudeArgs(workspace, mcpLog, session string) ([]string, error) {
	mcp, err := a.mcpWiring(runnerClaude, workspace, mcpLog)
	if err != nil {
		return nil, err
	}

	args := append(append([]string(nil), claudeBaseArgs...), mcp.Args...)
	if a.Model != "" {
		args = append(args, modelFlag, a.Model)
	}
	if session != "" {
		args = append(args, claudeResumeFlag, session)
	}
	// No positional prompt: --print reads it from stdin (see RunTurn). This also
	// keeps the variadic --mcp-config from swallowing a trailing positional.
	return args, nil
}

func (a Adapter) codexArgs(workspace, mcpLog, session string) ([]string, error) {
	mcp, err := a.mcpWiring(runnerCodex, workspace, mcpLog)
	if err != nil {
		return nil, err
	}

	args := []string{codexExecCmd}
	if session != "" {
		args = append(args, codexResumeCmd)
	}
	args = append(args, codexBaseArgs...)
	args = append(args, mcp.Args...)
	if a.Model != "" {
		args = append(args, modelFlag, a.Model)
	}
	if session != "" {
		args = append(args, session)
	}
	// "-" makes codex exec read the prompt from stdin (see RunTurn).
	return append(args, "-"), nil
}

// mcpWiring points the runner at the harness binary as its qrouton MCP server,
// through the launch path's own wiring: a graded run has to reach the tool
// surface the way a launched agent does, or it grades a different agent.
func (a Adapter) mcpWiring(runner, workspace, mcpLog string) (launch.MCPWiring, error) {
	wiring, err := launch.RunnerMCPWiring(runner, a.SelfPath, mockMCPArgs(mcpLog, workspace))
	if err != nil {
		return launch.MCPWiring{}, fmt.Errorf("%s: %w", a.Name, err)
	}
	return wiring, nil
}

// mockMCPArgs invokes the harness binary as the mock qrouton MCP server.
func mockMCPArgs(mcpLog, workspace string) []string {
	return []string{mockMCPSubcommand, mockMCPLogFlag, mcpLog, mockMCPRootFlag, workspace}
}
