package agentevent

import (
	"encoding/json"
	"os"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/urfave/cli/v2"
)

type noticeOutput struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// NoticeCommand hands the queued notices to the main agent as context for the
// event that fired it. A subagent's event leaves the queue for the main agent.
var NoticeCommand = &cli.Command{
	Name:  noticeCommandName,
	Usage: noticeCommandUsage,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: sessionRootFlag, Usage: sessionRootUsage, Required: true},
	},
	Action: func(c *cli.Context) error {
		var input struct {
			HookEventName string `json:"hook_event_name"`
			AgentID       string `json:"agent_id"`
		}
		if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
			return err
		}
		if input.AgentID != "" {
			return nil
		}
		notices, err := session.TakeAgentNotices(c.String(sessionRootFlag))
		if err != nil || len(notices) == 0 {
			return err
		}
		var out noticeOutput
		out.HookSpecificOutput.HookEventName = input.HookEventName
		out.HookSpecificOutput.AdditionalContext = session.JoinAgentNotices(notices)
		return json.NewEncoder(c.App.Writer).Encode(out)
	},
}
