package session

import (
	"os"
	"strings"
	"time"

	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// MarkOpened records that the workbench has just shown a session. The manifest
// has three unsynchronised load-modify-write authors, so the stamp is its own file.
func MarkOpened(root string, at time.Time) error {
	if err := os.MkdirAll(sessionpaths.Dir(root), dirMode); err != nil {
		return err
	}
	return os.WriteFile(sessionpaths.Opened(root), []byte(at.UTC().Format(time.RFC3339Nano)), fileMode)
}

// LastOpened is when a session was last shown, and false for one this workbench
// has never opened.
func LastOpened(root string) (time.Time, bool) {
	b, err := os.ReadFile(sessionpaths.Opened(root))
	if err != nil {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(b)))
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}
