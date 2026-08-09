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

	repos := []status.RepoStat{}
	result := make(chan []status.RepoStat, 1)
	// A refresh runs off this loop so a wedged git cannot stall the 2s field
	// ticker; pending guards against a second one starting before it answers.
	pending := false
	refresh := func() {
		if !pending {
			pending = true
			go func(root string) { result <- status.Repos(ctx, root) }(root())
		}
	}

	refresh()
	pushChrome(root(), activity(), repos, emit)
	for {
		select {
		case <-ctx.Done():
			return
		case repos = <-result:
			pending = false
		case <-stats.C:
			refresh()
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
