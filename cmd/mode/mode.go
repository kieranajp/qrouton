// Package mode wires the session-mode verb: `qrouton mode assistant` is the
// de-escalation path behind the Alt-n keybinding, writing the manifest and
// signalling the agent supervisor to relaunch with the conversation intact.
package mode

import (
	"fmt"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:      commandName,
	Usage:     commandUsage,
	ArgsUsage: commandArgsUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
		&cli.BoolFlag{Name: shellStackFlag, Usage: shellStackUsage, Hidden: true},
	},
	Action: func(c *cli.Context) error {
		var mode session.SessionMode
		switch c.Args().First() {
		case string(session.ModeAssistant):
			mode = session.ModeAssistant
		case string(session.ModeRPI):
			mode = session.ModeRPI
		default:
			return fmt.Errorf("%w: %q", errUnknownMode, c.Args().First())
		}
		if c.Bool(shellStackFlag) {
			stack, err := mux.CurrentShellStack()
			if err != nil {
				return err
			}
			if _, err := launch.JoinShellStack(c.Context, stack, deescalatingPaneSuffix); err != nil {
				return err
			}
		}
		dir := c.String(sessionRootFlag)
		if err := session.SetMode(dir, mode); err != nil {
			return err
		}
		// Best-effort: with no supervisor running, the mode still takes
		// effect on the next launch.
		launch.SignalSupervisor(dir)
		return nil
	},
}
