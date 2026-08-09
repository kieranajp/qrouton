package desktop

import (
	"context"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
)

// watchChrome pushes what the window can observe until the context is
// cancelled. Escalation rewrites the manifest, so re-reading it on a poll is
// what keeps the window agreeing with the session.
func watchChrome(ctx context.Context, root func() string, activity func() string, emit emitter) {
	fields := time.NewTicker(chromeInterval)
	defer fields.Stop()
	stats := time.NewTicker(repoStatInterval)
	defer stats.Stop()

	repos := status.Repos(root())
	pushChrome(root(), activity(), repos, emit)
	for {
		select {
		case <-ctx.Done():
			return
		case <-stats.C:
			repos = status.Repos(root())
		case <-fields.C:
		}
		pushChrome(root(), activity(), repos, emit)
	}
}

func pushChrome(root, activity string, repos []status.RepoStat, emit emitter) {
	fields, ok := status.Read(root)
	if !ok {
		return
	}
	fields.Repos, fields.Activity = repos, activity
	emit(chromeEvent, fields)
}
