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
	mcpConfig := map[string]any{
		mcpServersKey: map[string]any{
			mcpServerName: map[string]any{
				mcpTypeKey:    mcpStdioType,
				mcpCommandKey: a.SelfPath,
				mcpArgsKey:    mockMCPArgs(mcpLog, workspace),
			},
		},
	}
	encodedConfig, err := json.Marshal(mcpConfig)
	if err != nil {
		return nil, fmt.Errorf("encode Claude MCP config: %w", err)
	}

	args := append(append([]string(nil), claudeBaseArgs...), claudeMCPConfigFlag, string(encodedConfig))
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
	command, err := json.Marshal(a.SelfPath)
	if err != nil {
		return nil, fmt.Errorf("encode Codex MCP command: %w", err)
	}
	mcpArgs, err := json.Marshal(mockMCPArgs(mcpLog, workspace))
	if err != nil {
		return nil, fmt.Errorf("encode Codex MCP args: %w", err)
	}

	args := []string{codexExecCmd}
	if session != "" {
		args = append(args, codexResumeCmd)
	}
	args = append(args, codexBaseArgs...)
	args = append(args,
		codexConfigFlag, codexMCPCommandKey+string(command),
		codexConfigFlag, codexMCPArgsKey+string(mcpArgs),
	)
	if a.Model != "" {
		args = append(args, modelFlag, a.Model)
	}
	if session != "" {
		args = append(args, session)
	}
	// "-" makes codex exec read the prompt from stdin (see RunTurn).
	return append(args, "-"), nil
}

// mockMCPArgs invokes the harness binary as the mock qrouton MCP server.
func mockMCPArgs(mcpLog, workspace string) []string {
	return []string{mockMCPSubcommand, mockMCPLogFlag, mcpLog, mockMCPRootFlag, workspace}
}
