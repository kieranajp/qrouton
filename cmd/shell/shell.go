// Package shell wires the internal shell-pane command used by the startup
// layout and the Alt-g binding. It is not a general shell launcher: its job is
// to keep every user-created shell inside qrouton's permanent shell stack.
package shell

import (
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mux"
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
		stack, err := mux.CurrentShellStack()
		if err != nil {
			return err
		}
		return launch.Shell(c.Context, c.String(sessionRootFlag), stack)
	},
}
