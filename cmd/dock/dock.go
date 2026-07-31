// Package dock wires the permanent dock-anchor subcommand used by the
// workspace layout.
package dock

import (
	"github.com/kieranajp/qrouton/internal/dock"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Action: func(*cli.Context) error {
		return dock.Status()
	},
}
