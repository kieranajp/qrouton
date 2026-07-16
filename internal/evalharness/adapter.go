package evalharness

// Runner adapters: how the harness invokes the Claude and Codex CLIs — MCP
// wiring, session continuation, and prompt delivery. Output parsing lives in
// normalize.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Adapter struct {
	Name     string
	Bin      string
	Model    string
	SelfPath string
}

func (a Adapter) Version(ctx context.Context) string {
	output, err := exec.CommandContext(ctx, a.Bin, "--version").CombinedOutput()
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
		Kind: "duration",
		Turn: turn,
		Text: time.Since(started).String(),
	})
	return events, final, newSession, nil
}

func (a Adapter) args(workspace, mcpLog, session string) ([]string, error) {
	if a.SelfPath == "" {
		return nil, fmt.Errorf("eval executable path is empty")
	}

	switch a.Name {
	case "claude":
		return a.claudeArgs(workspace, mcpLog, session)
	case "codex":
		return a.codexArgs(workspace, mcpLog, session)
	default:
		return nil, fmt.Errorf("unknown runner %q", a.Name)
	}
}

func (a Adapter) claudeArgs(workspace, mcpLog, session string) ([]string, error) {
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"qrouton": map[string]any{
				"type":    "stdio",
				"command": a.SelfPath,
				"args":    []string{"mock-mcp", "--log", mcpLog, "--root", workspace},
			},
		},
	}
	encodedConfig, err := json.Marshal(mcpConfig)
	if err != nil {
		return nil, fmt.Errorf("encode Claude MCP config: %w", err)
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--setting-sources", "project",
		"--strict-mcp-config",
		"--mcp-config", string(encodedConfig),
	}
	if a.Model != "" {
		args = append(args, "--model", a.Model)
	}
	if session != "" {
		args = append(args, "--resume", session)
	}
	// No positional prompt: --print reads it from stdin (see RunTurn). This also
	// keeps the variadic --mcp-config from swallowing a trailing positional.
	return args, nil
}

func (a Adapter) codexArgs(workspace, mcpLog, session string) ([]string, error) {
	command, err := json.Marshal(a.SelfPath)
	if err != nil {
		return nil, fmt.Errorf("encode Codex MCP command: %w", err)
	}
	mcpArgs, err := json.Marshal([]string{"mock-mcp", "--log", mcpLog, "--root", workspace})
	if err != nil {
		return nil, fmt.Errorf("encode Codex MCP args: %w", err)
	}

	args := []string{"exec"}
	if session != "" {
		args = append(args, "resume")
	}
	args = append(args,
		"--json",
		"--ephemeral",
		"--ignore-user-config",
		"--enable", "multi_agent",
		"--skip-git-repo-check",
		"--dangerously-bypass-approvals-and-sandbox",
		"-c", "mcp_servers.qrouton.command="+string(command),
		"-c", "mcp_servers.qrouton.args="+string(mcpArgs),
	)
	if a.Model != "" {
		args = append(args, "--model", a.Model)
	}
	if session != "" {
		args = append(args, session)
	}
	// "-" makes codex exec read the prompt from stdin (see RunTurn).
	return append(args, "-"), nil
}
