package agents

import (
	"os"

	"github.com/kieranajp/qrouton/internal/agents"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  "agents",
	Usage: "Watch a session's subagent statuses (redraws forever; used by the zellij layout)",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "session-root", Usage: "qrouton session root", Required: true},
		&cli.StringFlag{Name: "runner", Usage: "runner whose agents to scan (claude scans the session log, otherwise codex)"},
	},
	Action: func(c *cli.Context) error {
		return agents.Status(c.String("session-root"), c.String("runner"))
	},
}

var EventCommand = &cli.Command{
	Name:  "agent-event",
	Usage: "Record a Claude subagent hook event from stdin",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "session-root", Usage: "qrouton session root", Required: true},
	},
	Action: func(c *cli.Context) error {
		return agents.RecordEvent(c.String("session-root"), os.Stdin)
	},
}
