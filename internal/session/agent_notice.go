package session

import (
	"os"
	"strings"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// QueueAgentNotice leaves one complete notice for the next resumed runner.
func QueueAgentNotice(dir, notice string) error {
	if strings.TrimSpace(notice) == "" {
		return nil
	}
	if err := os.MkdirAll(sessionpaths.Dir(dir), dirMode); err != nil {
		return err
	}
	return atomicfile.Replace(sessionpaths.AgentNotice(dir), []byte(notice), privateFileMode)
}
