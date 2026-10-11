package agentevent

import (
	"encoding/json"
	"strings"
)

// Target is the one thing a tool call is about: a path, a command's first
// line, a pattern, a URL, a spawn's description, or an MCP tool's own name.
func Target(tool string, input json.RawMessage) string {
	if rest, ok := strings.CutPrefix(tool, mcpToolPrefix); ok {
		server, name, found := strings.Cut(rest, mcpToolSeparator)
		if !found {
			return rest
		}
		return server + mcpTargetJoin + name
	}
	var fields map[string]any
	if json.Unmarshal(input, &fields) != nil {
		return ""
	}
	field := func(key string) string {
		value, _ := fields[key].(string)
		return value
	}
	switch tool {
	case toolBash:
		line, _, _ := strings.Cut(strings.TrimSpace(field("command")), "\n")
		return line
	case toolGrep, toolGlob:
		return field("pattern")
	case toolWebFetch:
		return field("url")
	case toolWebSearch:
		return field("query")
	case toolAgent, toolTaskLegacy:
		return field("description")
	}
	for _, key := range fileInputKeys {
		if path := field(key); path != "" {
			return path
		}
	}
	return ""
}
