package agents

import (
	"context"
	"os"

	"github.com/kieranajp/qrouton/internal/agents"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

// Notification is the only signal Claude gives that it is blocked on the user.
// No other runner has an equivalent.
var attention = map[string]string{
	hookNotification:  status.ActivityWaiting,
	hookSubagentStart: status.ActivityWorking,
}

var EventCommand = &cli.Command{
	Name:  eventCommandName,
	Usage: eventCommandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
		&cli.StringFlag{Name: workbenchJSONFlag, Usage: workbenchJSONUsage},
	},
	Action: func(c *cli.Context) error {
		hook, err := agents.RecordEvent(c.String(sessionRootFlag), os.Stdin)
		if err != nil {
			return err
		}
		signal(c.String(workbenchJSONFlag), attention[hook])
		return nil
	},
}

// Best-effort: a failed hook is noise in the runner's own output.
func signal(marshalled, activity string) {
	if marshalled == "" || activity == "" {
		return
	}
	handle, err := workbench.ParseHandle(marshalled)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), signalTimeout)
	defer cancel()
	_ = handle.Attention(ctx, activity)
}
