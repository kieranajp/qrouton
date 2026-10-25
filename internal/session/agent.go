package session

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// ReadAgentPID reads the pid a session's agent.pid records.
func ReadAgentPID(root string) (int, bool) {
	b, err := os.ReadFile(sessionpaths.AgentPID(root))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

// AgentAlive reports the pid in a session's agent.pid and whether a process still
// answers to it. A killed workbench leaves the file behind, so dead means free.
func AgentAlive(root string) (pid int, alive bool) {
	pid, ok := ReadAgentPID(root)
	if !ok {
		return 0, false
	}
	// Signal 0 probes for the process without delivering anything; EPERM is a
	// live process owned by another user.
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return pid, false
	}
	return pid, true
}
