package mcpserver

// The MCP tool surface served to the agent over stdio: tool schemas,
// descriptions, and registration. The pane mechanics live in panes.go.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type openFileInput struct {
	Path string `json:"path" jsonschema:"Path to an existing file in the qrouton session"`
	Line int    `json:"line,omitempty" jsonschema:"One-based line number; defaults to 1"`
}

type runCommandInput struct {
	Command     string `json:"command" jsonschema:"Shell command to run in a workspace pane the user can watch"`
	Name        string `json:"name,omitempty" jsonschema:"Pane label; reusing a name replaces that pane. Defaults to \"command\""`
	Cwd         string `json:"cwd,omitempty" jsonschema:"Working directory within the session; defaults to the session root"`
	CloseOnExit bool   `json:"close_on_exit,omitempty" jsonschema:"Close the pane automatically when the command exits (default: keep it open)"`
}

type readPaneInput struct {
	Name string `json:"name" jsonschema:"Name of a pane previously opened via run_command or open_file"`
	Full bool   `json:"full,omitempty" jsonschema:"Include the full scrollback instead of just the visible screen"`
}

type paneNameInput struct {
	Name string `json:"name" jsonschema:"Name of a pane previously opened via run_command or open_file"`
}

type showDiffInput struct {
	Repo   string `json:"repo,omitempty" jsonschema:"Repo worktree path within the session (e.g. src/app). Omit to diff every session repo"`
	Staged bool   `json:"staged,omitempty" jsonschema:"Show staged (index) changes instead of the working tree"`
	Base   string `json:"base,omitempty" jsonschema:"Diff against this git ref (e.g. main) instead of the working tree"`
}

type notifyInput struct {
	Message string `json:"message" jsonschema:"Short message to surface to the user, e.g. why you need their attention"`
}

// textResult wraps a message as an MCP text block. Each tool pairs it with its
// own structured payload, which is the second value AddTool handlers return.
func textResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}

func newMCPServer(root string, editor launch.EditorCommand, host mux.PaneHost) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "qrouton", Version: "1"}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})
	pane := newPaneManager(root, editor, host)

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolOpenFile,
		Description: descOpenFile,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input openFileInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.openFile(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolRunCommand,
		Description: descRunCommand,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input runCommandInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.run(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolReadPane,
		Description: descReadPane,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input readPaneInput) (*mcp.CallToolResult, any, error) {
		text, err := pane.read(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(text), map[string]any{"output": text}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolShowDiff,
		Description: descShowDiff,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input showDiffInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.showDiff(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolNotify,
		Description: descNotify,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input notifyInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.notify(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolClosePane,
		Description: descClosePane,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input paneNameInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.closePane(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        toolListPanes,
		Description: descListPanes,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		names := pane.list()
		message := noPanesOpen
		if len(names) > 0 {
			message = openPanesPrefix + strings.Join(names, paneNameJoiner) + openPanesSuffix
		}
		return textResult(message), map[string]any{"panes": names}, nil
	})

	return server
}

// Run serves the qrouton MCP server over stdio. editorJSON is the resolved
// EditorCommand marshalled by the launcher (or inherited via QROUTON_EDITOR_JSON);
// muxJSON is the multiplexer Handle the launcher stamped into our arguments.
func Run(root, editorJSON, muxJSON string) error {
	var editor launch.EditorCommand
	if err := json.Unmarshal([]byte(editorJSON), &editor); err != nil || len(editor.Argv) == 0 {
		return ErrInvalidEditor
	}
	handle, err := mux.ParseHandle(muxJSON)
	if err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	host, err := handle.PaneHost()
	if err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	return newMCPServer(root, editor, host).Run(context.Background(), &mcp.StdioTransport{})
}
