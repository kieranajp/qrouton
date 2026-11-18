package session

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// QueueAgentNotice appends one notice to the queue a runner takes from, so a
// second notice never replaces one nobody has read yet.
func QueueAgentNotice(dir, notice string) error {
	if strings.TrimSpace(notice) == "" {
		return nil
	}
	if err := os.MkdirAll(sessionpaths.Dir(dir), dirMode); err != nil {
		return err
	}
	line, _ := json.Marshal(notice) // marshalling a string cannot fail
	return atomicfile.WithLock(sessionpaths.AgentNoticeLock(dir), privateFileMode, func() error {
		f, err := os.OpenFile(sessionpaths.AgentNotice(dir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, privateFileMode)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
}

// TakeAgentNotices empties the queue and returns its notices in the order they
// were queued. Whichever process takes first gets them all.
func TakeAgentNotices(dir string) ([]string, error) {
	if _, err := os.Stat(sessionpaths.Dir(dir)); os.IsNotExist(err) {
		return nil, nil
	}
	var notices []string
	err := atomicfile.WithLock(sessionpaths.AgentNoticeLock(dir), privateFileMode, func() error {
		raw, err := os.ReadFile(sessionpaths.AgentNotice(dir))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := os.Remove(sessionpaths.AgentNotice(dir)); err != nil {
			return err
		}
		notices = parseAgentNotices(raw)
		return nil
	})
	return notices, err
}

// parseAgentNotices reads a file that is not JSON lines as one notice, which is
// how a notice queued by an older qrouton looks.
func parseAgentNotices(raw []byte) []string {
	var notices []string
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var notice string
		if err := json.Unmarshal(line, &notice); err != nil {
			return wholeNotice(raw)
		}
		if strings.TrimSpace(notice) != "" {
			notices = append(notices, notice)
		}
	}
	return notices
}

func wholeNotice(raw []byte) []string {
	if notice := strings.TrimSpace(string(raw)); notice != "" {
		return []string{notice}
	}
	return nil
}

// JoinAgentNotices is how several notices read as one message to a runner.
func JoinAgentNotices(notices []string) string {
	return strings.Join(notices, agentNoticeSeparator)
}
