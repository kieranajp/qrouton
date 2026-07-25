package mcp

import (
	"fmt"

	"github.com/kieranajp/qrouton/internal/mcpserver"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "mcp",
	Usage: "Serve the qrouton MCP server (editor pane tools) over stdio",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "session-root", Usage: "qrouton session root", Required: true},
		&cli.StringFlag{Name: "editor-json", Usage: "resolved editor configuration", EnvVars: []string{"QROUTON_EDITOR_JSON"}},
		&cli.StringFlag{Name: "mux-json", Usage: "multiplexer handle stamped by the launcher"},
		// Pre-handle launches recorded these into live sessions' MCP configs;
		// keep them working so attaching after an upgrade doesn't strand the agent.
		&cli.StringFlag{Name: "zellij-session", Usage: "target Zellij session (deprecated: use --mux-json)"},
		&cli.StringFlag{Name: "socket-dir", Usage: "Zellij socket directory (deprecated: use --mux-json)"},
	},
	Action: func(c *cli.Context) error {
		muxJSON := c.String("mux-json")
		if muxJSON == "" {
			if c.String("zellij-session") == "" {
				return fmt.Errorf("mcp: --mux-json is required")
			}
			muxJSON = mux.Handle{Kind: "zellij", Session: c.String("zellij-session"), SocketDir: c.String("socket-dir")}.Marshal()
		}
		return mcpserver.Run(c.String("session-root"), c.String("editor-json"), muxJSON)
	},
}
