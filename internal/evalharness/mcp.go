package evalharness

import (
	"context"
	"fmt"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type openFileInput struct {
	Path string `json:"path" jsonschema:"Path to a file inside the evaluation workspace"`
	Line int    `json:"line,omitempty" jsonschema:"One-based line number"`
}

func RunMockMCP(ctx context.Context, root, logPath string) error {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "qrouton-eval", Version: "1"},
		&mcp.ServerOptions{
			Instructions: "Record open_file calls during prompt evaluation.",
		},
	)
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "open_file",
			Description: "Open a completed document for the user.",
		},
		func(
			_ context.Context,
			_ *mcp.CallToolRequest,
			input openFileInput,
		) (*mcp.CallToolResult, any, error) {
			path, err := resolveWorkspacePath(root, input.Path)
			if err != nil {
				return nil, nil, err
			}
			if _, err := os.Stat(path); err != nil {
				return nil, nil, err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil, nil, err
			}
			event := Event{
				Kind: "tool_call",
				Name: "open_file",
				Path: filepath.ToSlash(rel),
			}
			if err := AppendJSONL(logPath, event); err != nil {
				return nil, nil, err
			}
			message := fmt.Sprintf("Recorded open_file for %s", filepath.ToSlash(rel))
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: message}},
			}, map[string]any{"message": message}, nil
		},
	)
	return server.Run(ctx, &mcp.StdioTransport{})
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
