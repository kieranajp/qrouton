package mcp

import "github.com/kieranajp/qrouton/internal/launch"

const (
	commandName  = launch.MCPSubcommand
	commandUsage = "Serve the qrouton MCP server (window and editor tools) over stdio"

	sessionRootFlag  = launch.SessionRootFlag
	sessionRootUsage = "qrouton session root"

	editorJSONFlag  = launch.EditorJSONFlag
	editorJSONUsage = "resolved editor configuration"

	workbenchJSONFlag  = launch.WorkbenchJSONFlag
	workbenchJSONUsage = "workbench handle stamped by the launcher"
)
