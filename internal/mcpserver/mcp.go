package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type openFileInput struct {
	Path string `json:"path" jsonschema:"Path to an existing file in the qrouton session"`
	Line int    `json:"line,omitempty" jsonschema:"One-based line number; defaults to 1"`
}

type editorPane struct {
	root, zellij, session string
	editor                launch.EditorCommand
	mu                    sync.Mutex
	paneID                string
}

var commandContext = exec.CommandContext

func (p *editorPane) open(ctx context.Context, input openFileInput) (string, error) {
	path, err := launch.ResolveSessionFile(p.root, input.Path)
	if err != nil {
		return "", err
	}
	if input.Line < 1 {
		input.Line = 1
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.paneID != "" {
		_ = commandContext(ctx, p.zellij, "--session", p.session, "action", "close-pane", "--pane-id", p.paneID).Run()
		p.paneID = ""
	}
	editorArgs := p.editor.Args(path, input.Line)
	args := []string{"--session", p.session, "action", "new-pane", "--floating", "--close-on-exit", "--x", "66%", "--width", "33%", "--y", "3%", "--height", "94%", "--name", "Editor — exit editor or Alt-x to close", "--cwd", p.root, "--"}
	args = append(args, editorArgs...)
	out, err := commandContext(ctx, p.zellij, args...).Output()
	if err != nil {
		return "", fmt.Errorf("open editor pane: %w", err)
	}
	p.paneID = strings.TrimSpace(string(out))
	rel, _ := filepath.Rel(p.root, path)
	return fmt.Sprintf("Opened %s at line %d in the editor pane.", rel, input.Line), nil
}

func newMCPServer(root string, editor launch.EditorCommand, zellij, session string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "qrouton", Version: "1"}, &mcp.ServerOptions{
		Instructions: "Open files for the user in qrouton's editor pane when doing so is useful, especially after creating a document. Paths must belong to this session.",
	})
	pane := &editorPane{root: root, editor: editor, zellij: zellij, session: session}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "open_file",
		Description: "Open an existing session file in the user's configured terminal editor pane. Use this after creating a document when showing it to the user is helpful.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input openFileInput) (*mcp.CallToolResult, any, error) {
		message, err := pane.open(ctx, input)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}}, map[string]any{"message": message}, nil
	})
	return server
}

// Run serves the qrouton MCP server over stdio. editorJSON is the resolved
// EditorCommand marshalled by the launcher (or inherited via QROUTON_EDITOR_JSON).
func Run(root, editorJSON, zellijSession, socketDir string) error {
	os.Setenv("ZELLIJ_SOCKET_DIR", socketDir)
	var editor launch.EditorCommand
	if err := json.Unmarshal([]byte(editorJSON), &editor); err != nil || len(editor.Argv) == 0 {
		return fmt.Errorf("mcp: invalid inherited editor configuration")
	}
	zellij, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("mcp: zellij is unavailable")
	}
	return newMCPServer(root, editor, zellij, zellijSession).Run(context.Background(), &mcp.StdioTransport{})
}
