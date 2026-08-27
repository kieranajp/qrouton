package session

import (
	"os"
	"path/filepath"
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
	return replaceFile(sessionpaths.Opened(root), []byte(at.UTC().Format(time.RFC3339Nano)))
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

// Preferred is the session most recently shown, or the newest session when no
// candidate has been shown.
func Preferred(root string, sessions []Manifest) (Manifest, bool) {
	var best Manifest
	var stamp time.Time
	for _, m := range sessions {
		at, ok := LastOpened(filepath.Join(root, m.Slug))
		if ok && (best.Slug == "" || at.After(stamp)) {
			best, stamp = m, at
		}
	}
	if best.Slug != "" {
		return best, true
	}
	for _, m := range sessions {
		if best.Slug == "" || m.CreatedAt.After(best.CreatedAt) {
			best = m
		}
	}
	return best, best.Slug != ""
}
