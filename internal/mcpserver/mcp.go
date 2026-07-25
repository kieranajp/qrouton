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

// textResult wraps a message as both an MCP text block and a structured payload,
// matching the shape callers already expect from open_file.
func textResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}

func newMCPServer(root string, editor launch.EditorCommand, host mux.PaneHost) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "qrouton", Version: "1"}, &mcp.ServerOptions{
		Instructions: "Drive the user's qrouton workspace. Panes you open are floating, pinned, and leave focus on the agent, so the user can watch them while chatting. Use open_file to show a document (especially after creating one); run_command to run long-lived or noisy work (dev servers, watchers, builds, logs) in a visible pane instead of your own shell; read_pane to inspect that output; show_diff to display a repo's changes for review; notify to get the user's attention when you finish or need them; close_pane/list_panes to manage them. All paths and working directories must belong to this session.",
	})
	pane := newPaneManager(root, editor, host)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_file",
		Description: "Open an existing session file in the user's configured terminal editor pane. The pane stays open for reference while the user keeps chatting with the agent. Use this after creating a document when showing it to the user is helpful.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input openFileInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.openFile(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "run_command",
		Description: "Run a shell command in a visible workspace pane instead of your own shell. Ideal for long-running or noisy processes (dev servers, test watchers, builds, log tails) the user should see live. The pane is floating and pinned, focus stays on the agent, and reusing a name replaces that pane. Read its output later with read_pane.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input runCommandInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.run(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_pane",
		Description: "Capture the current output of a pane opened with run_command (or open_file) and return it as text. Use this to check on a command you started — for example to confirm a dev server booted or to read a test run's failures. Set full to include the scrollback.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input readPaneInput) (*mcp.CallToolResult, any, error) {
		text, err := pane.read(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(text), map[string]any{"output": text}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "show_diff",
		Description: "Show a repo's git diff in a workspace pane for the user to review. Give repo as a worktree path within the session (e.g. src/app), or omit it to diff every session repo. Use base to compare against a ref (e.g. the default branch) or staged for index changes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input showDiffInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.showDiff(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "notify",
		Description: "Get the user's attention with an on-screen toast, the terminal bell, and a sound. Use this sparingly — when you finish a long task, need a decision, or are blocked — since the user may have stepped away while work runs.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input notifyInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.notify(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "close_pane",
		Description: "Close a pane previously opened with run_command or open_file, by name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input paneNameInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.closePane(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return textResult(message), map[string]any{"message": message}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_panes",
		Description: "List the panes qrouton is currently managing for you, by name.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		names := pane.list()
		message := "No qrouton-managed panes are open."
		if len(names) > 0 {
			message = "Open panes: " + strings.Join(names, ", ") + "."
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
		return fmt.Errorf("mcp: invalid inherited editor configuration")
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
