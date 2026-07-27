// Package sessionpaths owns the on-disk layout of a qrouton session: the
// manifest, the session-private directory, and everything qrouton generates
// inside it. Five packages write into that directory — the launcher, the
// multiplexer adapter, the MCP server, the prompt stamper, and the subagent
// watcher — and before this package each of them spelled ".qrouton" out for
// itself. A path convention with five authors is a path convention that drifts.
package sessionpaths

import "path/filepath"

const (
	// DirName is the session-private directory: everything qrouton generates
	// lives under it, and nothing a user authors does.
	DirName = ".qrouton"

	// ManifestName identifies a session directory. It is written last during
	// assembly, so a directory without it must not appear resumable.
	ManifestName = "qrouton.json"

	// SrcDirName holds the repository worktrees.
	SrcDirName = "src"

	// ThoughtsDirName holds durable workflow artifacts, shared with the user.
	ThoughtsDirName = "thoughts"
	SharedDirName   = "shared"

	// canonicalPromptsDirName holds the stamped prompt assets that the runner
	// discovery files link to.
	canonicalPromptsDirName = "qrspi"

	notifyScriptName   = "notify.sh"
	helpScriptName     = "help.sh"
	claudeAgentLogName = "claude-agents.jsonl"
	handoffName        = "handoff.md"
	agentPIDName       = "agent.pid"
)

// Dir is the session-private directory inside a session root.
func Dir(root string) string { return filepath.Join(root, DirName) }

// Manifest is the session manifest's path.
func Manifest(root string) string { return filepath.Join(root, ManifestName) }

// Src is the directory holding a session's repository worktrees.
func Src(root string) string { return filepath.Join(root, SrcDirName) }

// Thoughts is the shared artifact directory the workflow writes into.
func Thoughts(root string) string {
	return filepath.Join(root, ThoughtsDirName, SharedDirName)
}

// CanonicalPrompts is where stamped prompt assets live; the discovery files at
// the session root are symlinks into it.
func CanonicalPrompts(root string) string {
	return filepath.Join(Dir(root), canonicalPromptsDirName)
}

// NotifyScript plays the attention sound, for both the notify MCP tool and the
// runner's notification hook.
func NotifyScript(root string) string {
	return filepath.Join(Dir(root), notifyScriptName)
}

// HelpScript renders the quick-start panel.
func HelpScript(root string) string {
	return filepath.Join(Dir(root), helpScriptName)
}

// ClaudeAgentLog records Claude subagent lifecycle hook events.
func ClaudeAgentLog(root string) string {
	return filepath.Join(Dir(root), claudeAgentLogName)
}

// Handoff is the assistant's escalation brief. When it exists, the prompt
// stamper appends it to the primary mode prompt, so a fresh orchestrator
// starts with the brief in its system prompt.
func Handoff(root string) string {
	return filepath.Join(Dir(root), handoffName)
}

// AgentPID records the agent supervisor's pid, so the picker and
// `qrouton mode` can signal it to relaunch the runner.
func AgentPID(root string) string {
	return filepath.Join(Dir(root), agentPIDName)
}
