// Package agent wires the agent-pane supervisor as a subcommand: the workspace
// layout runs it in the agent pane, and it launches (and, when signalled,
// relaunches) the actual runner so escalation can hand off to a fresh process
// without touching pane geometry.
package agent

import (
	"encoding/json"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/mux"
	"github.com/urfave/cli/v2"
)

var Command = &cli.Command{
	Name:  commandName,
	Usage: commandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
		&cli.StringFlag{Name: runnerFlag, Usage: runnerUsage, Required: true},
		&cli.StringFlag{Name: muxJSONFlag, Usage: muxJSONUsage, Required: true},
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
		handle, err := mux.ParseHandle(c.String(muxJSONFlag))
		if err != nil {
			return err
		}
		return launch.Supervise(c.String(sessionRootFlag), runner, handle, editor, c.Bool(resumeFlag))
	},
}
