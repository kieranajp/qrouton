package evalharness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	args, err := a.args(workspace, mcpLog, prompt, session)
	if err != nil {
		return nil, "", session, err
	}

	cmd := exec.CommandContext(ctx, a.Bin, args...)
	cmd.Dir = workspace
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

func (a Adapter) args(workspace, mcpLog, prompt, session string) ([]string, error) {
	if a.SelfPath == "" {
		return nil, fmt.Errorf("eval executable path is empty")
	}

	switch a.Name {
	case "claude":
		return a.claudeArgs(workspace, mcpLog, prompt, session)
	case "codex":
		return a.codexArgs(workspace, mcpLog, prompt, session)
	default:
		return nil, fmt.Errorf("unknown runner %q", a.Name)
	}
}

func (a Adapter) claudeArgs(workspace, mcpLog, prompt, session string) ([]string, error) {
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
	// --mcp-config accepts a variadic list, so terminate option parsing before
	// appending the positional prompt. Otherwise Claude treats the prompt as an
	// additional MCP config path.
	return append(args, "--", prompt), nil
}

func (a Adapter) codexArgs(workspace, mcpLog, prompt, session string) ([]string, error) {
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
	return append(args, prompt), nil
}

func Normalize(provider string, data []byte, turn int) ([]Event, string, string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var events []Event
	var final string
	var session string
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}

		var value map[string]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return events, final, session, fmt.Errorf(
				"%s event line %d: %w",
				provider,
				lineNumber,
				err,
			)
		}
		sanitized, visible := sanitizeObservable(value)
		if !visible {
			continue
		}
		value = sanitized.(map[string]any)

		typeName := firstString(value, "type", "event", "kind")
		if id := firstString(value, "session_id", "thread_id"); id != "" {
			session = id
		}
		text := extractText(value)
		name := firstString(value, "tool_name", "name")
		kind := normalizedKind(typeName, name)
		if text != "" && (kind == "assistant" || kind == "result") {
			final = text
		}

		arguments, err := json.Marshal(value)
		if err != nil {
			return events, final, session, fmt.Errorf("normalize %s event: %w", provider, err)
		}
		events = append(events, Event{
			Time:      time.Now().UTC(),
			Kind:      kind,
			Turn:      turn,
			Role:      provider,
			Name:      name,
			Text:      text,
			Arguments: arguments,
			RawType:   typeName,
		})
	}
	if err := scanner.Err(); err != nil {
		return events, final, session, err
	}
	if lineNumber == 0 {
		return nil, "", session, fmt.Errorf("%s produced no JSON events", provider)
	}
	return events, final, session, nil
}

func sanitizeObservable(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		typeName, _ := typed["type"].(string)
		lowerType := strings.ToLower(typeName)
		if strings.Contains(lowerType, "thinking") || strings.Contains(lowerType, "reasoning") {
			return nil, false
		}
		cleaned := make(map[string]any, len(typed))
		for key, nested := range typed {
			item, visible := sanitizeObservable(nested)
			if key == "item" && !visible {
				return nil, false
			}
			if visible {
				cleaned[key] = item
			}
		}
		return cleaned, true
	case []any:
		cleaned := make([]any, 0, len(typed))
		for _, nested := range typed {
			item, visible := sanitizeObservable(nested)
			if visible {
				cleaned = append(cleaned, item)
			}
		}
		return cleaned, true
	default:
		return value, true
	}
}

func normalizedKind(typeName, name string) string {
	value := strings.ToLower(typeName + " " + name)
	switch {
	case strings.Contains(value, "tool"), strings.Contains(value, "function"):
		return "tool_call"
	case strings.Contains(value, "agent"), strings.Contains(value, "task"):
		return "delegation"
	case strings.Contains(value, "assistant"), strings.Contains(value, "message"):
		return "assistant"
	case strings.Contains(value, "result"), strings.Contains(value, "completed"):
		return "result"
	default:
		return "provider_event"
	}
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := value[key].(string); ok {
			return text
		}
	}
	return ""
}

func extractText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		for index := len(typed) - 1; index >= 0; index-- {
			if text := extractText(typed[index]); text != "" {
				return text
			}
		}
	case map[string]any:
		keys := []string{"result", "final_output", "text", "content", "message", "item"}
		for _, key := range keys {
			item, ok := typed[key]
			if !ok {
				continue
			}
			if text := extractText(item); text != "" {
				return text
			}
		}
	}
	return ""
}

func ReadMCPEvents(path string, turn int) []Event {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var events []Event
	for _, line := range strings.Split(string(content), "\n") {
		if line == "" {
			continue
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		event.Turn = turn
		events = append(events, event)
	}
	return events
}

func AppendJSONL(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = file.Write(append(encoded, '\n'))
	return err
}
