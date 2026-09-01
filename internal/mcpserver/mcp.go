package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type openFileInput struct {
	Path       string `json:"path" jsonschema:"Path to an existing file in the qrouton session"`
	Line       int    `json:"line,omitempty" jsonschema:"One-based line number to draw the user's eye to; defaults to 1"`
	Through    int    `json:"through,omitempty" jsonschema:"Last line of the range to mark, when line opens one; defaults to line alone"`
	Foreground *bool  `json:"foreground,omitempty" jsonschema:"Sparse logical-selection override: true selects this tab, false keeps it in the background, and omitted uses the tool default"`
}

type runCommandInput struct {
	Command    string `json:"command" jsonschema:"Shell command to run in a window the user can watch"`
	Name       string `json:"name,omitempty" jsonschema:"Window name; reusing a name replaces that window. Defaults to \"command\""`
	Cwd        string `json:"cwd,omitempty" jsonschema:"Working directory within the session; defaults to the session root"`
	Foreground *bool  `json:"foreground,omitempty" jsonschema:"Sparse logical-selection override: true selects this tab, false keeps it in the background, and omitted uses the tool default"`
}

type readWindowInput struct {
	Name string `json:"name" jsonschema:"Name of a window previously opened via run_command or open_file"`
	Full bool   `json:"full,omitempty" jsonschema:"Include the full scrollback instead of just the last screenful"`
}

type windowNameInput struct {
	Name string `json:"name" jsonschema:"Name of a window previously opened via run_command or open_file"`
}

type showDiffInput struct {
	Repo       string `json:"repo,omitempty" jsonschema:"Repo worktree path within the session (e.g. src/app). Omit to diff every session repo"`
	Staged     bool   `json:"staged,omitempty" jsonschema:"Show staged (index) changes instead of the working tree"`
	Base       string `json:"base,omitempty" jsonschema:"Diff against this git ref (e.g. main) instead of the working tree"`
	Foreground *bool  `json:"foreground,omitempty" jsonschema:"Sparse logical-selection override: true selects this tab, false keeps it in the background, and omitted uses the tool default"`
}

type sharePageInput struct {
	Path string `json:"path" jsonschema:"Path to a markdown document in the qrouton session, relative to its root"`
}

type notifyInput struct {
	Message    string `json:"message" jsonschema:"Short message to surface to the user, e.g. why you need their attention"`
	Foreground *bool  `json:"foreground,omitempty" jsonschema:"Sparse logical-selection override: true selects this tab, false keeps it in the background, and omitted uses the tool default"`
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

// answer is what all but one of qrouton's tools return: a line for the agent to
// read, and sometimes the viewport measured for the document it opened.
type answer[In any] func(context.Context, In) (string, *workbench.DocumentViewport, error)

// addTool registers a tool whose result is one keyed string plus an optional
// viewport. key names that string in the structured payload — every tool says
// "message" except read_window, which is returning output rather than talking.
func addTool[In any](server *mcp.Server, name, description, key string, fn answer[In]) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description},
		func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, any, error) {
			text, viewport, err := fn(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			output := map[string]any{key: text}
			if viewport != nil {
				output["viewport"] = viewport
			}
			return textResult(text), output, nil
		})
}

func messageOnly[In any](fn func(context.Context, In) (string, error)) answer[In] {
	return func(ctx context.Context, input In) (string, *workbench.DocumentViewport, error) {
		message, err := fn(ctx, input)
		return message, nil, err
	}
}

func newMCPServer(root string, editor launch.EditorCommand, host workbench.WindowHost, mode session.SessionMode) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "qrouton", Version: "1"}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})
	windows := newWindowManager(root, editor, host)

	addTool(server, toolOpenFile, descOpenFile, keyMessage, windows.openFile)
	addTool(server, toolSharePage, descSharePage, keyMessage,
		messageOnly(func(_ context.Context, input sharePageInput) (string, error) {
			return sharePage(root, input)
		}))
	addTool(server, toolRunCommand, descRunCommand, keyMessage, messageOnly(windows.run))
	addTool(server, toolReadWindow, descReadWindow, keyOutput, windows.read)
	addTool(server, toolShowDiff, descShowDiff, keyMessage, messageOnly(windows.showDiff))
	addTool(server, toolNotify, descNotify, keyMessage, messageOnly(windows.notify))
	addTool(server, toolCloseWindow, descCloseWindow, keyMessage, messageOnly(windows.closeWindow))

	// Escalation is the assistant's way out of its own mode. An RPI session is
	// already where it leads, so the tool is not offered there at all.
	if mode == session.ModeAssistant {
		addTool(server, toolEscalate, descEscalate, keyMessage, messageOnly(windows.escalate))
	}

	// list_windows is the one tool whose payload is a list rather than a line,
	// so it stays hand-written rather than bending the shape above.
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

	return server
}

// Run serves the qrouton MCP server over stdio. editorJSON is the resolved
// EditorCommand marshalled by the launcher (or inherited via QROUTON_EDITOR_JSON);
// workbenchJSON is the Handle the launcher stamped into our arguments.
func Run(root, editorJSON, workbenchJSON string) error {
	editor, err := launch.ParseEditor(editorJSON)
	if err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	handle, err := workbench.ParseHandle(workbenchJSON)
	if err != nil {
		return fmt.Errorf("mcp: %w", err)
	}
	return newMCPServer(root, editor, handle.WindowHost(), sessionMode(root)).
		Run(context.Background(), &mcp.StdioTransport{})
}

// sessionMode is the mode this server serves, read once at startup: an
// escalation replaces the runner process and this server with it, so the mode
// cannot change under a server that is already running. A manifest that will not
// load reads as RPI, which withholds a tool rather than offering the agent one
// its own prompt says it does not have.
func sessionMode(root string) session.SessionMode {
	manifest, err := session.Load(root)
	if err != nil {
		return session.ModeRPI
	}
	return manifest.EffectiveMode()
}
