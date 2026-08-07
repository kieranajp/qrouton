package mcp

import (
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mcpserver"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
		&cli.StringFlag{Name: editorJSONFlag, Usage: editorJSONUsage, EnvVars: []string{launch.EditorEnvVar}},
		&cli.StringFlag{Name: workbenchJSONFlag, Usage: workbenchJSONUsage, Required: true},
	},
	Action: func(c *cli.Context) error {
		return mcpserver.Run(c.String(sessionRootFlag), c.String(editorJSONFlag), c.String(workbenchJSONFlag))
	},
}
