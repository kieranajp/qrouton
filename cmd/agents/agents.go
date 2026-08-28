package agents

import (
	"context"
	"os"
	"time"

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
		&cli.Uint64Flag{Name: generationFlag, Usage: generationUsage, Required: true},
	},
	Action: func(c *cli.Context) error {
		// A log that could not be written must not also cost the window the
		// only signal it gets: the hook name survives the failure.
		event, hook, err := agents.RecordEvent(c.String(sessionRootFlag), os.Stdin)
		signal(c.String(workbenchJSONFlag), c.Uint64(generationFlag), event, attention[hook])
		return err
	},
}

// Best-effort: a failed hook is noise in the runner's own output.
func signal(marshalled string, generation uint64, event agents.Event, activity string) {
	if marshalled == "" {
		return
	}
	handle, err := workbench.ParseHandle(marshalled)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), signalTimeout)
	defer cancel()
	if event.AgentID != "" {
		kind := ""
		switch event.HookEventName {
		case hookSubagentStart:
			kind = workbench.LifecycleStart
		case hookSubagentStop:
			kind = workbench.LifecycleStop
		}
		if kind != "" {
			at, _ := time.Parse(time.RFC3339Nano, event.Timestamp)
			_ = handle.DelegatedLifecycle(ctx, workbench.DelegatedLifecycleRequest{
				Provider: providerClaude, Generation: generation, Kind: kind,
				ID: event.AgentID, Type: event.AgentType, ParentID: event.ParentID, Timestamp: at,
			})
		}
	}
	if activity != "" {
		_ = handle.Attention(ctx, activity, generation)
	}
}
