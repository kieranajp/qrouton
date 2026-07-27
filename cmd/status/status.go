// Package status wires the strip subcommand: the workspace layout runs it as
// the full-width one-row pane at the bottom of every session.
package status

import (
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
	},
	Action: func(c *cli.Context) error {
		return status.Status(c.String(sessionRootFlag))
	},
}
