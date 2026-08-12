// Package onboard wires the landing list as a subcommand: the workbench runs it
// as the conversation terminal's first child, and it hands that terminal over to
// the agent supervisor in place.
package onboard

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/tui"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:   commandName,
	Usage:  commandUsage,
	Hidden: true,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: socketFlag, Usage: socketUsage, Required: true},
		&cli.StringFlag{Name: runnerFlag, Usage: runnerUsage},
		&cli.BoolFlag{Name: refreshFlag, Usage: refreshUsage},
		&cli.BoolFlag{Name: adoptOnlyFlag, Usage: adoptOnlyUsage},
	},
	Action: func(c *cli.Context) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		sessions, err := session.Scan(cfg.Root)
		if err != nil {
			return err
		}
		request, err := tui.Run(cfg, sessions, c.String(runnerFlag), c.Bool(refreshFlag))
		// Leaving the list without choosing anything is a clean exit, which takes
		// the workbench with it.
		if err != nil || request == nil {
			return err
		}
		return handOver(c.Context, cfg, c.String(socketFlag), *request, c.Bool(adoptOnlyFlag))
	},
}

// handOver replaces this process with the session's agent supervisor, keeping the
// terminal the landing list drew in. adoptOnly exits and the workbench boots it.
func handOver(ctx context.Context, cfg *config.Config, socket string, request tui.LaunchRequest, adoptOnly bool) error {
	editor, err := launch.ResolveEditor(cfg.Editor)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	argv, env, err := launch.Launch(request.Dir, request.Runner, bin, socket, editor, request.Resume)
	if err != nil {
		return err
	}
	host, err := (workbench.Handle{Socket: socket, SessionRoot: request.Dir}).WindowHost()
	if err != nil {
		return err
	}
	// The workbench is waiting to be told which session this is: its chrome, its
	// user shell, and the agent it boots when this process is not the one to.
	if err := host.Adopt(ctx, request.Dir, adoptOnly); err != nil {
		return fmt.Errorf("%w: %w", errNotAdopted, err)
	}
	if adoptOnly {
		return nil
	}
	return handover(argv, env)
}

// handover is a package variable so a test can assert what would have replaced
// this process.
var handover = execHandover

func execHandover(argv, env []string) error {
	return syscall.Exec(argv[0], argv, env)
}
