package evalharness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kieranajp/qrouton/internal/mcpserver"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RunMockMCP(ctx context.Context, root, logPath string) error {
	return newMockMCP(root, logPath).Run(ctx, &mcp.StdioTransport{})
}

func newMockMCP(root, logPath string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: mockMCPName, Version: "1"}, &mcp.ServerOptions{Instructions: mockMCPInstructions})
	mcpserver.AddWorkflowTools(server, root, func(_ context.Context, input mcpserver.OpenFileInput) (string, *workbench.DocumentViewport, error) {
		path, err := resolveWorkspacePath(root, input.Path)
		if err != nil {
			return "", nil, err
		}
		if _, err := os.Stat(path); err != nil {
			return "", nil, err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", nil, err
		}
		event := Event{Kind: kindToolCall, Name: mcpserver.ToolOpenFile, Path: filepath.ToSlash(rel)}
		if err := AppendJSONL(logPath, event); err != nil {
			return "", nil, err
		}
		return fmt.Sprintf(mockOpenFileFormat, mcpserver.ToolOpenFile, filepath.ToSlash(rel)), nil, nil
	})
	return server
}

func resolveWorkspacePath(root, requested string) (string, error) {
	if requested == "" {
		return "", fmt.Errorf("path is required")
	}
	path := requested
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = filepath.Clean(path)
	if _, inside := sessionpaths.Within(root, path); !inside {
		return "", fmt.Errorf("path %q is outside the evaluation workspace", requested)
	}
	return path, nil
}
