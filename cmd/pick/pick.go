// Package pick wires the escalation picker as a subcommand, so the Alt-e
// keybinding and the escalate MCP tool can float it over a live session.
package pick

import (
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/tui"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
		&cli.StringFlag{Name: nameFlag, Usage: nameUsage},
		&cli.StringFlag{Name: prefixFlag, Usage: prefixUsage},
	},
	Action: func(c *cli.Context) error {
		// Unlike the other pane subcommands, the picker needs the configured
		// owners for its repository list, so it loads config like onboard does.
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		return tui.RunPicker(cfg, c.String(sessionRootFlag), c.String(nameFlag), c.String(prefixFlag))
	},
}
