package repos

import (
	"github.com/kieranajp/qrouton/internal/repos"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
	},
	Action: func(c *cli.Context) error {
		return repos.Status(c.String(sessionRootFlag))
	},
}
