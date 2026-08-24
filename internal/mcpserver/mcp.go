package mcpserver

// The MCP tool surface served to the agent over stdio: tool schemas,
// descriptions, and registration.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type openFileInput struct {
	Path    string `json:"path" jsonschema:"Path to an existing file in the qrouton session"`
	Line    int    `json:"line,omitempty" jsonschema:"One-based line number to draw the user's eye to; defaults to 1"`
	Through int    `json:"through,omitempty" jsonschema:"Last line of the range to mark, when line opens one; defaults to line alone"`
}

type runCommandInput struct {
	Command string `json:"command" jsonschema:"Shell command to run in a window the user can watch"`
	Name    string `json:"name,omitempty" jsonschema:"Window name; reusing a name replaces that window. Defaults to \"command\""`
	Cwd     string `json:"cwd,omitempty" jsonschema:"Working directory within the session; defaults to the session root"`
}

type readWindowInput struct {
	Name string `json:"name" jsonschema:"Name of a window previously opened via run_command or open_file"`
	Full bool   `json:"full,omitempty" jsonschema:"Include the full scrollback instead of just the last screenful"`
}

type windowNameInput struct {
	Name string `json:"name" jsonschema:"Name of a window previously opened via run_command or open_file"`
}

type showDiffInput struct {
	Repo   string `json:"repo,omitempty" jsonschema:"Repo worktree path within the session (e.g. src/app). Omit to diff every session repo"`
	Staged bool   `json:"staged,omitempty" jsonschema:"Show staged (index) changes instead of the working tree"`
	Base   string `json:"base,omitempty" jsonschema:"Diff against this git ref (e.g. main) instead of the working tree"`
}

type sharePageInput struct {
	Path string `json:"path" jsonschema:"Path to a markdown document in the qrouton session, relative to its root"`
}

type notifyInput struct {
	Message string `json:"message" jsonschema:"Short message to surface to the user, e.g. why you need their attention"`
}

// escalateInput's branch_prefix values are the picker's own list: anything else
// is silently ignored and the picker falls back to its first entry.
type escalateInput struct {
	Name         string `json:"name" jsonschema:"the name for this piece of work"`
	BranchPrefix string `json:"branch_prefix,omitempty" jsonschema:"one of feat, fix, chore, refactor, docs, test"`
}

// textResult wraps a message as an MCP text block. Each tool pairs it with its
// own structured payload, which is the second value AddTool handlers return.
func textResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}

func newMCPServer(root string, editor launch.EditorCommand, host workbench.WindowHost) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "qrouton", Version: "1"}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})
	windows := newWindowManager(root, editor, host)

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolOpenFile,
		Description: descOpenFile,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input openFileInput) (*mcp.CallToolResult, any, error) {
		message, viewport, err := windows.openFile(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		output := map[string]any{"message": message}
		if viewport != nil {
			output["viewport"] = viewport
		}
		return textResult(message), output, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolSharePage,
		Description: descSharePage,
	}, func(_ context.Context, _ *mcp.CallToolRequest, input sharePageInput) (*mcp.CallToolResult, any, error) {
		message, err := sharePage(root, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolRunCommand,
		Description: descRunCommand,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input runCommandInput) (*mcp.CallToolResult, any, error) {
		message, err := windows.run(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolReadWindow,
		Description: descReadWindow,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input readWindowInput) (*mcp.CallToolResult, any, error) {
		text, viewport, err := windows.read(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		output := map[string]any{"output": text}
		if viewport != nil {
			output["viewport"] = viewport
		}
		return textResult(text), output, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolShowDiff,
		Description: descShowDiff,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input showDiffInput) (*mcp.CallToolResult, any, error) {
		message, err := windows.showDiff(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolNotify,
		Description: descNotify,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input notifyInput) (*mcp.CallToolResult, any, error) {
		message, err := windows.notify(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolCloseWindow,
		Description: descCloseWindow,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input windowNameInput) (*mcp.CallToolResult, any, error) {
		message, err := windows.closeWindow(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolListWindows,
		Description: descListWindows,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		names := windows.list(ctx)
		message := noWindowsOpen
		if len(names) > 0 {
			message = openWindowsPrefix + strings.Join(names, windowNameJoiner) + openWindowsSuffix
		}
		return textResult(message), map[string]any{"windows": names}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolEscalate,
		Description: descEscalate,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input escalateInput) (*mcp.CallToolResult, any, error) {
		message, err := windows.escalate(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	return server
}

// Run serves the qrouton MCP server over stdio. editorJSON is the resolved
// EditorCommand marshalled by the launcher (or inherited via QROUTON_EDITOR_JSON);
// workbenchJSON is the Handle the launcher stamped into our arguments.
func Run(root, editorJSON, workbenchJSON string) error {
	var editor launch.EditorCommand
	if err := json.Unmarshal([]byte(editorJSON), &editor); err != nil || len(editor.Argv) == 0 {
		return ErrInvalidEditor
	}
	handle, err := workbench.ParseHandle(workbenchJSON)
	if err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	host, err := handle.WindowHost()
	if err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	return newMCPServer(root, editor, host).Run(context.Background(), &mcp.StdioTransport{})
}
