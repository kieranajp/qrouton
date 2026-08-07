// Package agent wires the runner supervisor as a subcommand: the workbench runs
// it in the conversation terminal, and it relaunches the runner on escalation
// without disturbing that terminal.
package agent

import (
	"encoding/json"

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
		&cli.StringFlag{Name: workbenchJSONFlag, Usage: workbenchJSONUsage, Required: true},
		&cli.StringFlag{Name: editorJSONFlag, Usage: editorJSONUsage, EnvVars: []string{launch.EditorEnvVar}},
		&cli.BoolFlag{Name: resumeFlag, Usage: resumeUsage},
	},
	Action: func(c *cli.Context) error {
		// The runner crosses the exec boundary as an identifier and is
		// re-resolved against config, so launch overrides still apply.
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		runner, err := launch.ByID(cfg, c.String(runnerFlag))
		if err != nil {
			return err
		}
		var editor launch.EditorCommand
		if err := json.Unmarshal([]byte(c.String(editorJSONFlag)), &editor); err != nil || len(editor.Argv) == 0 {
			return errInvalidEditor
		}
		handle, err := workbench.ParseHandle(c.String(workbenchJSONFlag))
		if err != nil {
			return err
		}
		return launch.Supervise(c.String(sessionRootFlag), runner, handle, editor, c.Bool(resumeFlag))
	},
}
