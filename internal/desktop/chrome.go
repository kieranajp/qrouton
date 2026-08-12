package desktop

import (
	"context"
	"time"

	"github.com/kieranajp/qrouton/internal/status"
)

// watchChrome pushes what the window can observe about the session on screen
// until the context is cancelled. Escalation rewrites the manifest, so
// re-reading it on a poll is what keeps the window agreeing with the session.
func watchChrome(ctx context.Context, reg *Sessions, root string, emit emitter) {
	watch(ctx, reg, root, emit, chromeInterval, repoStatInterval,
		unseenCounts{all: status.Unseen, in: status.UnseenIn})
}

// unseenCounts is what the rail knows about documents nobody has looked at: every
// session's, which is what rides the slow ticker, and one session's, which is
// cheap enough to run the moment the user arrives at it.
type unseenCounts struct {
	all func(root string) map[string]int
	in  func(sessionRoot string) (int, bool)
}

// measurement is one session's repository stats, named by the session they were
// measured for: a switch must not inherit the last session's numbers.
type measurement struct {
	root  string
	stats []status.RepoStat
}

// watch takes its intervals and its unseen count so a test can drive the two
// tickers apart. Everything a background session costs rides the slow one.
func watch(ctx context.Context, reg *Sessions, root string, emit emitter, field, slow time.Duration, count unseenCounts) {
	fields := time.NewTicker(field)
	defer fields.Stop()
	stats := time.NewTicker(slow)
	defer stats.Stop()

	measured := map[string][]status.RepoStat{}
	unseen := count.all(root)
	result := make(chan measurement, 1)
	// A refresh runs off this loop so a wedged git cannot stall the field ticker;
	// pending guards against a second one starting before it answers.
	pending := false
	refresh := func() {
		shown := reg.current().root()
		if pending || shown == "" {
			return
		}
		if _, done := measured[shown]; done {
			return
		}
		pending = true
		go func() { result <- measurement{root: shown, stats: status.Repos(ctx, shown)} }()
	}

	refresh()
	pushChrome(reg, root, measured, unseen, emit)
	for {
		select {
		case <-ctx.Done():
			return
		case done := <-result:
			pending = false
			measured[done.root] = done.stats
			refresh()
		case <-stats.C:
			delete(measured, reg.current().root())
			refresh()
			unseen = count.all(root)
		case <-reg.touched:
			refresh()
			// Arriving at a session is looking at what it wrote, and a marker still
			// lit after that is a marker nobody reads.
			if shown := reg.current().root(); shown != "" {
				if at, ok := count.in(shown); ok {
					unseen[slugFor(shown)] = at
				}
			}
		case <-fields.C:
		}
		pushChrome(reg, root, measured, unseen, emit)
	}
}

// pushChrome emits even with the session-level fields empty: the page cannot
// attach to a conversation whose terminal id it has not been told.
func pushChrome(reg *Sessions, root string, measured map[string][]status.RepoStat, unseen map[string]int, emit emitter) {
	shown := reg.current()
	fields := status.Read(shown.root())
	if repos, ok := measured[shown.root()]; ok {
		fields.Repos = repos
	}
	if shown != nil {
		fields.Terminal, fields.Activity = shown.terminal, shown.activity.state()
	}
	// The rail stays empty while the conversation pane is choosing a session: a row
	// clicked from there would switch away from an onboarding nothing can return to.
	if shown.slug() != "" {
		// A terminal id and an activity are this workbench's own knowledge, so only
		// the sessions it has booted carry them.
		fields.Sessions = reg.railOrder(status.Sessions(root))
		for i, row := range fields.Sessions {
			if state := reg.bySlug(row.Slug); state != nil {
				fields.Sessions[i].Terminal = state.terminal
				fields.Sessions[i].Activity = state.activity.state()
			}
			fields.Sessions[i].Unseen = unseen[row.Slug]
		}
	}
	emit(chromeEvent, fields)
}
