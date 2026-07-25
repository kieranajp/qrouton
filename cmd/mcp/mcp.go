package mcp

import (
	"github.com/kieranajp/qrouton/internal/mcpserver"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "mcp",
	Usage: "Serve the qrouton MCP server (editor pane tools) over stdio",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "session-root", Usage: "qrouton session root", Required: true},
		&cli.StringFlag{Name: "editor-json", Usage: "resolved editor configuration", EnvVars: []string{"QROUTON_EDITOR_JSON"}},
		&cli.StringFlag{Name: "mux-json", Usage: "multiplexer handle stamped by the launcher", Required: true},
	},
	Action: func(c *cli.Context) error {
		return mcpserver.Run(c.String("session-root"), c.String("editor-json"), c.String("mux-json"))
	},
}
