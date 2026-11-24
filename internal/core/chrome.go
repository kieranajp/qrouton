package core

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/launch"
	"github.com/kieranajp/qrouton/internal/status"
)

// Chrome is the current workbench state exposed to a page that subscribes
// after the initial update.
type Chrome struct {
	mu          sync.RWMutex
	fields      status.Fields
	initialized bool
	emit        Emitter
	tree        func(slug string) status.AgentTree
}

func newChrome(emit Emitter) *Chrome {
	return &Chrome{fields: status.EmptyFields(), emit: emit}
}

func (c *Chrome) Snapshot() status.Fields {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fields
}

// publish adapts Chrome to the Emitter seam. A payload that is not chrome state
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
func watchChrome(ctx context.Context, reg *Sessions, root string, cfg *config.Config, emit Emitter, pulls pullRequestSource) {
	watch(ctx, reg, root, cfg, emit, chromeInterval, repoStatInterval,
		railCounts{all: status.Unseen, in: status.UnseenIn, todos: status.Todos}, pulls)
}

// railCounts is what the rail counts per session. Every session's unseen documents
// and open todos ride the slow ticker; one session's unseen count is cheap enough
// to run the moment the user arrives at it.
type railCounts struct {
	all   func(root string) map[string]int
	in    func(sessionRoot string) (int, bool)
	todos func(root string) map[string]status.TodoRollup
}

// railFacts are the per-session counts a push copies onto the rail's rows.
type railFacts struct {
	unseen map[string]int
	todos  map[string]status.TodoRollup
}

// pullRequestSource is the Dashboard as the chrome loop sees it.
type pullRequestSource interface {
	rows(root string) status.PullRequestList
	Request()
}

// measurement is one session's repository stats, named by the session they were
// measured for: a switch must not inherit the last session's numbers.
type measurement struct {
	root  string
	stats []status.RepoStat
}

// watch takes its intervals, its unseen count and its pull requests so a test
// can drive the tickers apart. Everything a background session costs rides the
// slow one.
func watch(ctx context.Context, reg *Sessions, root string, cfg *config.Config, emit Emitter, field, slow time.Duration, count railCounts, pulls pullRequestSource) {
	fields := time.NewTicker(field)
	defer fields.Stop()
	stats := time.NewTicker(slow)
	defer stats.Stop()

	measured := map[string][]status.RepoStat{}
	rail := railFacts{unseen: count.all(root), todos: count.todos(root)}
	result := make(chan measurement, 1)
	// A refresh runs off this loop so a wedged git cannot stall the field ticker;
	// pending guards against a second one starting before it answers.
	pending := false
	refresh := func() {
		shown := reg.Current().Root()
		if pending || shown == "" {
			return
		}
		if _, done := measured[shown]; done {
			return
		}
		pending = true
		go func() { result <- measurement{root: shown, stats: status.Repos(ctx, shown)} }()
	}

	// requestedFor is the shown root pull requests were last asked for; clearing
	// it asks again.
	requestedFor := ""
	request := func() {
		shown := reg.Current().Root()
		if shown == requestedFor {
			return
		}
		requestedFor = shown
		if shown != "" {
			pulls.Request()
		}
	}

	refresh()
	request()
	pushChrome(reg, root, cfg, measured, pulls, rail, emit)
	for {
		select {
		case <-ctx.Done():
			return
		case done := <-result:
			pending = false
			measured[done.root] = done.stats
			refresh()
		case <-stats.C:
			delete(measured, reg.Current().Root())
			refresh()
			rail.unseen = count.all(root)
			rail.todos = count.todos(root)
		case <-reg.touched:
			for _, root := range reg.takeRepositoryChanges() {
				delete(measured, root)
				if root == requestedFor {
					requestedFor = ""
				}
			}
			if len(reg.takePullRequestChanges()) > 0 {
				pulls.Request()
			}
			refresh()
			request()
			// Arriving at a session is looking at what it wrote, and a marker still
			// lit after that is a marker nobody reads.
			if shown := reg.Current().Root(); shown != "" {
				if at, ok := count.in(shown); ok {
					rail.unseen[slugFor(shown)] = at
				}
			}
		case <-fields.C:
		}
		pushChrome(reg, root, cfg, measured, pulls, rail, emit)
		if reg.trees != nil {
			reg.trees.schedule(reg.Current())
		}
	}
}

// pushChrome emits even with the session-level fields empty: the page cannot
// attach to a conversation whose terminal id it has not been told.
func pushChrome(reg *Sessions, root string, cfg *config.Config, measured map[string][]status.RepoStat, pulls pullRequestSource, rail railFacts, emit Emitter) {
	for _, state := range reg.all() {
		state.mu.Lock()
		state.pruneBugReports(reg.now())
		state.mu.Unlock()
	}
	shown := reg.Current()
	shownRoot := shown.Root()
	fields := status.Read(shownRoot)
	configSnapshot := cfg.Snapshot()
	fields.Stickers = []status.Sticker{}
	for _, s := range configSnapshot.EffectiveStickers() {
		fields.Stickers = append(fields.Stickers, status.Sticker{ID: s.ID, Colour: s.Colour, Symbol: s.Symbol, Label: s.Label})
	}
	fields.Profiles = switchableProfiles(configSnapshot)
	if shown != nil {
		shown.mu.Lock()
		shown.pruneBugReports(reg.now())
		if report := shown.bugReports[shown.latestBugReport]; report != nil {
			fields.BugReportID, fields.BugReportStatus, fields.BugReportLabel = report.outcome.ID, report.outcome.Status, bugReportReviewLabel
		}
		shown.mu.Unlock()
		fields.Activity = shown.agents.state()
		if pending := shown.PendingPicker(); pending != nil {
			fields.Picker, fields.PickerID = true, pending.ID
		}
		if shown.alive() {
			fields.Terminal = shown.terminal
		}
	}
	if repos, ok := measured[shownRoot]; ok {
		fields.Repos = repos
	}
	if pulls != nil && shownRoot != "" {
		fields.PullRequests = pulls.rows(shownRoot)
	}
	agentSnapshots := reg.agentActivitySnapshots()
	if snapshot, ok := agentSnapshots[fields.Slug]; ok {
		fields.StagesLive = status.StagesFor(snapshot.LiveTypes)
	}
	// The rail populates with nothing on screen: a window opening on no session at
	// all still has to offer the ones on disk.
	fields.Sessions = reg.railOrder(status.Sessions(root))
	// Dereferenced on every tick: a value captured at wiring time re-raises the
	// overlay two seconds after it closes. An install with sessions has been set up
	// already, so only an empty one is asked.
	fields.Welcoming = configSnapshot != nil && !configSnapshot.Welcomed && fields.Slug == "" && len(fields.Sessions) == 0
	fields.Placing = reg.Placing() && !fields.Welcoming
	for i, row := range fields.Sessions {
		// A terminal id and an activity are this workbench's own knowledge, so only
		// the sessions it has booted carry them. The id goes when the supervisor
		// does: a pane drawn against a dead one takes keystrokes nothing reads.
		if state := reg.BySlug(row.Slug); state != nil {
			if state.alive() {
				fields.Sessions[i].Terminal = state.terminal
			}
			fields.Sessions[i].Activity = state.agents.state()
		}
		if snapshot, ok := agentSnapshots[row.Slug]; ok {
			fields.Sessions[i].Summary = agentSummary(snapshot)
		}
		fields.Sessions[i].Unseen = rail.unseen[row.Slug]
		if row.Slug == fields.Slug {
			// The shown row agrees with the Overview, and keeps that count once the user moves on.
			shownTodos := status.Rollup(fields.Todos)
			fields.Sessions[i].Todos = shownTodos
			if rail.todos != nil {
				rail.todos[row.Slug] = shownTodos
			}
		} else if rollup, ok := rail.todos[row.Slug]; ok {
			fields.Sessions[i].Todos = rollup
		}
	}
	emit(chromeEvent, fields)
}

func switchableProfiles(cfg *config.Config) []status.RunnerProfiles {
	out := []status.RunnerProfiles{}
	if cfg == nil {
		return out
	}
	for _, runner := range slices.Sorted(maps.Keys(cfg.Profiles)) {
		if !launch.SwitchesProfiles(runner) {
			continue
		}
		group := status.RunnerProfiles{Runner: runner, Profiles: []status.Profile{}}
		for _, p := range cfg.Profiles[runner] {
			group.Profiles = append(group.Profiles, status.Profile{ID: p.ID, Label: p.Label})
		}
		out = append(out, group)
	}
	return out
}

func agentSummary(snapshot agentActivitySnapshot) status.AgentSummary {
	summary := status.AgentSummary{
		Attention: status.AgentAttentionNone,
		Active:    snapshot.Active,
		Coverage:  status.AgentCoverageNone,
		Running:   snapshot.Running,
		Tools:     snapshot.Capabilities.Parents,
	}
	if !snapshot.Running {
		return summary
	}
	summary.Stage = status.StagesFor(snapshot.LiveTypes).Latest()
	for _, record := range snapshot.Records {
		if record.Root && record.Generation == snapshot.Generation {
			summary.State, summary.Reason = record.State, record.Reason
			summary.Tool = record.Tool
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
