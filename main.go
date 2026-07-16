// qrouton — assemble a multi-repo session (worktrees off local mirrors) and launch an agent runner in it.
// Spec: thoughts/shared/specs/S001-2026-07-15-workspace-harness.md
package main

import (
	"fmt"
	"os"

	agentscmd "github.com/kieranajp/qrouton/cmd/agents"
	mcpcmd "github.com/kieranajp/qrouton/cmd/mcp"
	reposcmd "github.com/kieranajp/qrouton/cmd/repos"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/tui"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "qrouton",
		Usage: "assemble a multi-repo session and launch an agent runner in it",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "refresh", Usage: "refresh the cached org repo list"},
			&cli.StringFlag{Name: "runner", Usage: "coding agent to launch (claude, codex, or opencode)"},
		},
		Commands: []*cli.Command{mcpcmd.Command, agentscmd.Command, agentscmd.EventCommand, reposcmd.Command},
		Action:   onboard,
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "qrouton:", err)
		os.Exit(1)
	}
}

// onboard is the default action: pick or create a session in the TUI, then launch its runner.
func onboard(c *cli.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	sessions, err := session.Scan(cfg.Root)
	if err != nil {
		return err
	}
	request, err := tui.Run(cfg, sessions, c.String("runner"), c.Bool("refresh"))
	if err != nil || request == nil {
		return err
	}
	return launchRunner(cfg, request.Dir, request.Runner, request.Resume)
}

func launchRunner(cfg *config.Config, dir string, r launch.Runner, resume bool) error {
	if err := launch.StampAssets(dir); err != nil {
		return err
	}
	editor, err := launch.ResolveEditor(cfg.Editor)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	return launch.Zellij(dir, r, bin, editor, resume)
}
