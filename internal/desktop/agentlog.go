package desktop

import (
	"fmt"
	"os"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// recordAgentExit appends one line per supervisor exit, and the tail of what
// the terminal printed when that exit was a failure. A failure to write is
// swallowed: a log nobody can keep must not take a session down with it.
func recordAgentExit(root, provider string, code int, tail string) {
	if root == "" {
		return
	}
	// Cleanup removes the session directory without waiting for the pump, so a
	// log written here must never be what puts it back.
	if _, err := os.Stat(sessionpaths.Dir(root)); err != nil {
		return
	}
	path := sessionpaths.AgentLog(root)
	rotateAgentLog(path)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	entry := fmt.Sprintf(agentExitLogFormat, time.Now().Format(time.RFC3339), provider, code)
	if code != 0 && tail != "" {
		entry += fmt.Sprintf(agentExitTailFormat, tail)
	}
	_, _ = file.WriteString(entry)
}

// rotateAgentLog keeps one previous log beside the current one, so a session
// whose supervisor dies over and over cannot grow a file without end.
func rotateAgentLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < agentLogLimit {
		return
	}
	_ = os.Rename(path, path+agentLogPreviousSuffix)
}
