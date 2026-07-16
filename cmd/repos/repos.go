package repos

import (
	"github.com/kieranajp/qrouton/internal/repos"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "repos",
	Usage: "Watch a session's repo branches and dirty state (redraws forever; used by the zellij layout)",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "session-root", Usage: "qrouton session root", Required: true},
	},
	Action: func(c *cli.Context) error {
		return repos.Status(c.String("session-root"))
	},
}
