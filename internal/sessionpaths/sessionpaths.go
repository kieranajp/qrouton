// Package sessionpaths owns the on-disk layout of a qrouton session: the
// manifest, the session-private directory, and everything qrouton generates
// inside it. Several packages write into that directory, and a path convention
// with several authors is a path convention that drifts.
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

	manifestLockName   = "manifest.lock"
	notifyScriptName   = "notify.sh"
	workbenchLogName   = "workbench.log"
	handoffName        = "handoff.md"
	handoffPendingName = "handoff.pending"
	initialPromptName  = "initial-prompt"
	agentNoticeName    = "agent-notice"
	agentPIDName       = "agent.pid"
	openedName         = "opened"

	// sharePagesDirName holds rendered pages staged for the agent to publish.
	// Distinct from thoughts/shared, which holds the documents themselves.
	sharePagesDirName = "share"
)

// Dir is the session-private directory inside a session root.
func Dir(root string) string { return filepath.Join(root, DirName) }

// Manifest is the session manifest's path.
func Manifest(root string) string { return filepath.Join(root, ManifestName) }

// ManifestLock is the advisory lock every manifest read-modify-write holds.
func ManifestLock(root string) string {
	return filepath.Join(Dir(root), manifestLockName)
}

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

// WorkbenchLog collects the detached workbench process's stdio. It has no
// terminal to fail into, so a crash after it started answering is only ever
// read here.
func WorkbenchLog(root string) string {
	return filepath.Join(Dir(root), workbenchLogName)
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

// Opened records when the workbench last showed a session, so opening the app
// comes back to the one you were in rather than the newest on disk.
func Opened(root string) string {
	return filepath.Join(Dir(root), openedName)
}

// HandoffPending marks an escalation that still owes the next runner a fresh
// conversation. On disk rather than in the supervisor's memory, so a restart
// between the escalation and the next launch still honours the handoff.
func HandoffPending(root string) string {
	return filepath.Join(Dir(root), handoffPendingName)
}

// InitialPrompt carries an external tool's opening request until the first
// runner launch consumes it. It is session-private and intentionally absent
// from the durable manifest.
func InitialPrompt(root string) string {
	return filepath.Join(Dir(root), initialPromptName)
}

// AgentNotice carries a workbench event until the next resumed runner launch.
func AgentNotice(root string) string {
	return filepath.Join(Dir(root), agentNoticeName)
}

// SharePages stages the self-contained pages rendered from session documents,
// each one waiting for the agent to hand it to somebody.
func SharePages(root string) string {
	return filepath.Join(Dir(root), sharePagesDirName)
}
