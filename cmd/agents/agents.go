package agents

import (
	"os"

	"github.com/kieranajp/qrouton/internal/agents"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
		&cli.StringFlag{Name: runnerFlag, Usage: runnerUsage},
	},
	Action: func(c *cli.Context) error {
		return agents.Status(c.String(sessionRootFlag), c.String(runnerFlag))
	},
}

var EventCommand = &cli.Command{
	Name:  eventCommandName,
	Usage: eventCommandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
	},
	Action: func(c *cli.Context) error {
		return agents.RecordEvent(c.String(sessionRootFlag), os.Stdin)
	},
}
