package agents

// Claude subagent lifecycle hook event names. These two are the whole of what
// the collector records; anything else the hook fires is ignored.
const (
	hookSubagentStart = "SubagentStart"
	hookSubagentStop  = "SubagentStop"
)
