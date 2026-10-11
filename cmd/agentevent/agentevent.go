package agentevent

import (
	"context"
	"os"
	"slices"
	"time"

	eventlog "github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
	"github.com/urfave/cli/v2"
)

var attention = map[string]string{
	hookNotification:           status.ActivityWaiting,
	hookStop:                   status.ActivityTurnEnded,
	eventlog.HookSubagentStart: status.ActivityWorking,
}

var EventCommand = &cli.Command{
	Name:  eventCommandName,
	Usage: eventCommandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: workbenchJSONFlag, Usage: workbenchJSONUsage, EnvVars: []string{eventlog.WorkbenchEnvVar}},
		&cli.Uint64Flag{Name: generationFlag, Usage: generationUsage, EnvVars: []string{eventlog.GenerationEnvVar}, Required: true},
		&cli.StringFlag{Name: providerFlag, Usage: providerUsage, EnvVars: []string{eventlog.ProviderEnvVar}, Required: true},
	},
	Action: func(c *cli.Context) error {
		provider, generation := c.String(providerFlag), c.Uint64(generationFlag)
		// A runner reporting its tool calls gets the whole reduced event; one
		// reporting lifecycle only keeps the lifecycle and attention pair.
		if slices.Contains(launch.RunnerHooks(provider), eventlog.HookPostToolUse) {
			event, err := eventlog.Reduce(provider, os.Stdin)
			if err == nil {
				event.Generation = generation
				send(c.String(workbenchJSONFlag), func(ctx context.Context, h workbench.Handle) {
					_ = h.AgentEvent(ctx, event)
				})
			}
			return err
		}
		event, hook, err := eventlog.Record(provider, os.Stdin)
		send(c.String(workbenchJSONFlag), func(ctx context.Context, h workbench.Handle) {
			lifecycle(ctx, h, generation, event, attention[hook])
		})
		return err
	},
}

// Best-effort: a failed hook is noise in the runner's own output.
func send(marshalled string, call func(context.Context, workbench.Handle)) {
	if marshalled == "" {
		return
	}
	handle, err := workbench.ParseHandle(marshalled)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), signalTimeout)
	defer cancel()
	call(ctx, handle)
}

func lifecycle(ctx context.Context, handle workbench.Handle, generation uint64, event eventlog.Event, activity string) {
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
