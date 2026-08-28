package agentevent

import (
	"context"
	"os"
	"time"

	eventlog "github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

var attention = map[string]string{
	hookNotification:           status.ActivityWaiting,
	eventlog.HookSubagentStart: status.ActivityWorking,
}

var EventCommand = &cli.Command{
	Name:  eventCommandName,
	Usage: eventCommandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, EnvVars: []string{eventlog.SessionRootEnvVar}, Required: true},
		&cli.StringFlag{Name: workbenchJSONFlag, Usage: workbenchJSONUsage, EnvVars: []string{eventlog.WorkbenchEnvVar}},
		&cli.Uint64Flag{Name: generationFlag, Usage: generationUsage, EnvVars: []string{eventlog.GenerationEnvVar}, Required: true},
		&cli.StringFlag{Name: providerFlag, Usage: providerUsage, EnvVars: []string{eventlog.ProviderEnvVar}, Required: true},
	},
	Action: func(c *cli.Context) error {
		// A log that could not be written must not also cost the window the
		// only signal it gets: the hook name survives the failure.
		event, hook, err := eventlog.Record(c.String(sessionRootFlag), c.String(providerFlag), os.Stdin)
		signal(c.String(workbenchJSONFlag), c.Uint64(generationFlag), event, attention[hook])
		return err
	},
}

// Best-effort: a failed hook is noise in the runner's own output.
func signal(marshalled string, generation uint64, event eventlog.Event, activity string) {
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
		case eventlog.HookSubagentStart:
			kind = workbench.LifecycleStart
		case eventlog.HookSubagentStop:
			kind = workbench.LifecycleStop
		}
		if kind != "" {
			at, _ := time.Parse(time.RFC3339Nano, event.Timestamp)
			_ = handle.DelegatedLifecycle(ctx, workbench.DelegatedLifecycleRequest{
				Provider: event.Provider, Generation: generation, Kind: kind,
				ID: event.AgentID, Type: event.AgentType, ParentID: event.ParentID, Timestamp: at,
			})
		}
	}
	if activity != "" {
		_ = handle.Attention(ctx, activity, generation)
	}
}
