package mcpserver

import (
	"context"
	"encoding/json"
	"time"

	"github.com/kieranajp/qrouton/internal/vault"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerVault(server *mcp.Server, host workbench.WindowHost) {
	port, ok := host.(workbench.VaultHost)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status, err := port.VaultStatus(ctx)
	if err != nil || len(status.Scope.Profiles) == 0 {
		return
	}
	mcp.AddTool(server, &mcp.Tool{Name: toolReadVault, Description: descReadVault}, func(ctx context.Context, _ *mcp.CallToolRequest, input vault.Reference) (*mcp.CallToolResult, vault.ReadResult, error) {
		out, err := port.ReadVault(ctx, input)
		if err != nil {
			return nil, out, err
		}
		return textResult(out.Content), out, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: toolSearchVault, Description: descSearchVault}, func(ctx context.Context, _ *mcp.CallToolRequest, input workbench.VaultSearchRequest) (*mcp.CallToolResult, workbench.VaultSearchResult, error) {
		out, err := port.SearchVault(ctx, input)
		if err != nil {
			return nil, out, err
		}
		body, err := json.Marshal(out)
		if err != nil {
			return nil, out, err
		}
		return textResult(string(body)), out, nil
	})
}
