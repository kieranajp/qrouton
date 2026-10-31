package reviewer

import (
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:   launch.ReviewerSubcommand,
	Usage:  commandUsage,
	Flags:  []cli.Flag{&cli.StringFlag{Name: launch.ReviewRequestFlag, Usage: requestUsage, Required: true}},
	Action: func(c *cli.Context) error { return launch.RunReviewer(c.String(launch.ReviewRequestFlag)) },
}
