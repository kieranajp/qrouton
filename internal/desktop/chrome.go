package desktop

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/status"
)

// Chrome is the current workbench state exposed to a page that subscribes
// after the initial update.
type Chrome struct {
	mu          sync.RWMutex
	fields      status.Fields
	initialized bool
	emit        emitter
	tree        func(slug string) status.AgentTree
}

func newChrome(emit emitter) *Chrome {
	return &Chrome{fields: status.EmptyFields(), emit: emit}
}

func (c *Chrome) Snapshot() status.Fields {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fields
}

// publish adapts Chrome to the emitter seam. A payload that is not chrome state
// belongs to some other producer sharing the seam, so it goes out untouched.
func (c *Chrome) publish(event string, payload any) {
	fields, ok := payload.(status.Fields)
	if !ok {
		c.emit(event, payload)
		return
	}
	c.publishFields(fields)
}

func (c *Chrome) publishFields(fields status.Fields) {
	c.mu.Lock()
	if c.initialized && reflect.DeepEqual(c.fields, fields) {
		c.mu.Unlock()
		return
	}
	c.fields = fields
	c.initialized = true
	c.mu.Unlock()
	c.emit(chromeEvent, fields)
}

// watchChrome pushes what the window can observe about the session on screen
// until the context is cancelled. Escalation rewrites the manifest, so
// re-reading it on a poll is what keeps the window agreeing with the session.
func watchChrome(ctx context.Context, reg *Sessions, root string, cfg *config.Config, emit emitter) {
	watch(ctx, reg, root, cfg, emit, chromeInterval, repoStatInterval,
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
func watch(ctx context.Context, reg *Sessions, root string, cfg *config.Config, emit emitter, field, slow time.Duration, count unseenCounts) {
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
	pushChrome(reg, root, cfg, measured, unseen, emit)
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
			for _, root := range reg.takeRepositoryChanges() {
				delete(measured, root)
			}
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
		pushChrome(reg, root, cfg, measured, unseen, emit)
		if reg.trees != nil {
			reg.trees.schedule(reg.current())
		}
	}
}

// pushChrome emits even with the session-level fields empty: the page cannot
// attach to a conversation whose terminal id it has not been told.
func pushChrome(reg *Sessions, root string, cfg *config.Config, measured map[string][]status.RepoStat, unseen map[string]int, emit emitter) {
	for _, state := range reg.all() {
		state.mu.Lock()
		state.pruneBugReports(reg.now())
		state.mu.Unlock()
	}
	shown := reg.current()
	shownRoot := shown.root()
	fields := status.Read(shownRoot)
	configSnapshot := cfg.Snapshot()
	if configSnapshot != nil {
		labels := configSnapshot.EffectiveStickerLabels()
		fields.StickerLabels = status.StickerLabels{
			Star: labels.Star, Bookmark: labels.Bookmark,
			Question: labels.Question, Exclamation: labels.Exclamation,
		}
	} else {
		labels := config.DefaultStickerLabels
		fields.StickerLabels = status.StickerLabels{
			Star: labels.Star, Bookmark: labels.Bookmark,
			Question: labels.Question, Exclamation: labels.Exclamation,
		}
	}
	if shown != nil {
		shown.mu.Lock()
		shown.pruneBugReports(reg.now())
		if report := shown.bugReports[shown.latestBugReport]; report != nil {
			fields.BugReportID, fields.BugReportStatus, fields.BugReportLabel = report.outcome.ID, report.outcome.Status, bugReportReviewLabel
		}
		shown.mu.Unlock()
		fields.Activity = shown.agents.state()
		fields.Picker = shown.pendingPicker() != nil
		if shown.alive() {
			fields.Terminal = shown.terminal
		}
	}
	if repos, ok := measured[shownRoot]; ok {
		fields.Repos = repos
	}
	agentSnapshots := reg.agentActivitySnapshots()
	// Dereferenced on every tick: a value captured at wiring time re-raises the
	// overlay two seconds after it closes. A window holding a session never asks,
	// so the questions can never land over a live conversation — and an install
	// that always opens on one stays unasked until it opens on none.
	fields.Welcoming = configSnapshot != nil && !configSnapshot.Welcomed && fields.Slug == ""
	if snapshot, ok := agentSnapshots[fields.Slug]; ok {
		fields.StagesLive = status.StagesFor(snapshot.LiveTypes)
	}
	// The rail populates with nothing on screen: a window opening on no session at
	// all still has to offer the ones on disk.
	fields.Sessions = reg.railOrder(status.Sessions(root))
	for i, row := range fields.Sessions {
		// A terminal id and an activity are this workbench's own knowledge, so only
		// the sessions it has booted carry them. The id goes when the supervisor
		// does: a pane drawn against a dead one takes keystrokes nothing reads.
		if state := reg.bySlug(row.Slug); state != nil {
			if state.alive() {
				fields.Sessions[i].Terminal = state.terminal
			}
			fields.Sessions[i].Activity = state.agents.state()
		}
		if snapshot, ok := agentSnapshots[row.Slug]; ok {
			fields.Sessions[i].Summary = agentSummary(snapshot)
		}
		fields.Sessions[i].Unseen = unseen[row.Slug]
	}
	emit(chromeEvent, fields)
}

func agentSummary(snapshot agentActivitySnapshot) status.AgentSummary {
	summary := status.AgentSummary{
		Attention: status.AgentAttentionNone,
		Active:    snapshot.Active,
		Coverage:  status.AgentCoverageNone,
		Running:   snapshot.Running,
	}
	if !snapshot.Running {
		return summary
	}
	for _, record := range snapshot.Records {
		if record.Root && record.Generation == snapshot.Generation {
			summary.State, summary.Reason = record.State, record.Reason
			summary.Tool, summary.Target = record.Tool, record.Target
			break
		}
	}
	if snapshot.Capabilities.Children {
		summary.Coverage = status.AgentCoverageFull
	} else {
		summary.Coverage = status.AgentCoverageRoot
	}
	if snapshot.Capabilities.Attention {
		if snapshot.Unread {
			summary.Attention = status.AgentAttentionUnread
		}
	} else {
		summary.Attention = status.AgentAttentionUnknown
	}
	return summary
}
