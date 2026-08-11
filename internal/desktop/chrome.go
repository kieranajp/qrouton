package desktop

import (
	"context"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
)

// chromeFields is the session identity the main window draws around its
// terminal.
type chromeFields struct {
	Mode     string `json:"mode"`
	Phase    string `json:"phase"`
	Identity string `json:"identity"`
}

// watchChrome pushes the manifest's mode, phase and name at the window until
// the context is cancelled. Escalation rewrites the manifest, so re-reading it
// on a poll is what keeps the window agreeing with the session.
func watchChrome(ctx context.Context, root func() string, emit emitter) {
	ticker := time.NewTicker(chromeInterval)
	defer ticker.Stop()
	for {
		pushChrome(root(), emit)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func pushChrome(root string, emit emitter) {
	fields, ok := status.Read(root)
	if !ok {
		return
	}
	emit(chromeEvent, chromeFields{Mode: fields.Mode, Phase: fields.Phase, Identity: fields.Identity})
}
