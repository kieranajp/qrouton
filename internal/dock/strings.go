package dock

import "time"

const (
	// PaneName is both the staged pane title and the runtime lookup key used
	// when the MCP server asks Zellij to stack a floating pane into the dock.
	PaneName        = "dock"
	emptyStateLabel = "Agent panes minimise here"
	emptyStateHint  = "Ask the agent to restore one"
	refreshInterval = time.Hour
)
