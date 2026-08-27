package mcpserver

import (
	"fmt"
	"os"

	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"github.com/kieranajp/qrouton/internal/share"
)

// sharePage stages a session document as a page somebody outside the session
// can read. qrouton renders it and stops there: who sees it is the agent's
// business, and the user's.
func sharePage(root string, input sharePageInput) (string, error) {
	path, err := launch.ResolveSessionFile(root, input.Path)
	if err != nil {
		return "", err
	}
	markdown, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	source := launch.SessionRelative(root, path, input.Path)
	page, err := share.Write(sessionpaths.SharePages(root), source, markdown)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(sharedPageFormat, source, page), nil
}
