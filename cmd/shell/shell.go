// Package shell wires the session's user shell: the workbench opens one
// alongside the conversation, rooted in the session directory.
package shell

import (
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:   commandName,
	Usage:  commandUsage,
	Hidden: true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
	},
	Action: func(c *cli.Context) error {
		return launch.Shell(c.Context, c.String(sessionRootFlag))
	},
}
