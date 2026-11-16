// Package agent wires the runner supervisor as a subcommand: the workbench runs
// it in the conversation terminal, and it relaunches the runner on escalation
// without disturbing that terminal.
package agent

import (
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
		&cli.StringFlag{Name: runnerFlag, Usage: runnerUsage, Required: true},
		&cli.StringFlag{Name: profileFlag, Usage: profileUsage},
		&cli.StringFlag{Name: workbenchJSONFlag, Usage: workbenchJSONUsage, Required: true},
		&cli.StringFlag{Name: editorJSONFlag, Usage: editorJSONUsage, EnvVars: []string{launch.EditorEnvVar}},
		&cli.BoolFlag{Name: resumeFlag, Usage: resumeUsage},
	},
	Action: func(c *cli.Context) error {
		runner, err := resolveRunner(c.String(runnerFlag), c.String(profileFlag))
		if err != nil {
			return err
		}
		editor, err := launch.ParseEditor(c.String(editorJSONFlag))
		if err != nil {
			return err
		}
		handle, err := workbench.ParseHandle(c.String(workbenchJSONFlag))
		if err != nil {
			return err
		}
		return launch.Supervise(c.String(sessionRootFlag), runner, handle, editor, c.Bool(resumeFlag))
	},
}

// resolveRunner re-resolves the runner and its profile, which cross the exec
// boundary as identifiers, against fresh config so overrides and profiles apply.
func resolveRunner(id, profile string) (launch.Runner, error) {
	cfg, err := config.Load()
	if err != nil {
		return launch.Runner{}, err
	}
	runner, err := launch.ByID(cfg, id)
	if err != nil {
		return launch.Runner{}, err
	}
	return launch.WithProfile(cfg, runner, profile)
}
