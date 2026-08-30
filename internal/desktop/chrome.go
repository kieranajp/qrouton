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
}

func newChrome(emit emitter) *Chrome {
	return &Chrome{fields: status.EmptyFields(), emit: emit}
}

// Snapshot returns the most recently published chrome state.
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

// publishFields stores the newest chrome state and pushes it, unless it is what
// the page was last sent.
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

type chromeExpiryTimer interface {
	channel() <-chan time.Time
	reset(time.Duration)
	stop()
}

type realChromeExpiryTimer struct{ timer *time.Timer }

func newChromeExpiryTimer() chromeExpiryTimer {
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	return &realChromeExpiryTimer{timer: timer}
}

func (t *realChromeExpiryTimer) channel() <-chan time.Time { return t.timer.C }
func (t *realChromeExpiryTimer) reset(after time.Duration) { t.timer.Reset(after) }
func (t *realChromeExpiryTimer) stop() {
	if !t.timer.Stop() {
		select {
		case <-t.timer.C:
		default:
		}
	}
}

// watch takes its intervals and its unseen count so a test can drive the two
// tickers apart. Everything a background session costs rides the slow one.
func watch(ctx context.Context, reg *Sessions, root string, cfg *config.Config, emit emitter, field, slow time.Duration, count unseenCounts) {
	watchWithExpiryTimer(ctx, reg, root, cfg, emit, field, slow, count, newChromeExpiryTimer())
}

func watchWithExpiryTimer(ctx context.Context, reg *Sessions, root string, cfg *config.Config, emit emitter, field, slow time.Duration, count unseenCounts, expiry chromeExpiryTimer) {
	fields := time.NewTicker(field)
	defer fields.Stop()
	stats := time.NewTicker(slow)
	defer stats.Stop()
	defer expiry.stop()

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
	resetAgentExpiry(expiry, reg)
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
		case <-expiry.channel():
		}
		pushChrome(reg, root, cfg, measured, unseen, emit)
		resetAgentExpiry(expiry, reg)
	}
}

func resetAgentExpiry(timer chromeExpiryTimer, reg *Sessions) {
	timer.stop()
	expires := reg.earliestAgentExpiry()
	if expires.IsZero() {
		return
	}
	after := expires.Sub(reg.now())
	if after < 0 {
		after = 0
	}
	timer.reset(after)
}

// pushChrome emits even with the session-level fields empty: the page cannot
// attach to a conversation whose terminal id it has not been told.
func pushChrome(reg *Sessions, root string, cfg *config.Config, measured map[string][]status.RepoStat, unseen map[string]int, emit emitter) {
	shown := reg.current()
	fields := status.Read(shown.root())
	agentSnapshots := reg.agentActivitySnapshots()
	// Dereferenced on every tick: a value captured at wiring time re-raises the
	// overlay two seconds after it closes. A window holding a session never asks,
	// so the questions can never land over a live conversation — and an install
	// that always opens on one stays unasked until it opens on none.
	fields.Welcoming = cfg != nil && !cfg.Welcomed && fields.Slug == ""
	if repos, ok := measured[shown.root()]; ok {
		fields.Repos = repos
	}
	if shown != nil {
		fields.Terminal, fields.Activity = shown.terminal, shown.agents.state()
		fields.Picker = shown.pendingPicker() != nil
	}
	if snapshot, ok := agentSnapshots[fields.Slug]; ok {
		panel := agentPanel(snapshot)
		if panel.Provider == "" {
			panel.Provider = fields.Agents.Provider
		}
		fields.Agents = panel
	}
	// The rail populates with nothing on screen: a window opening on no session at
	// all still has to offer the ones on disk.
	fields.Sessions = reg.railOrder(status.Sessions(root))
	for i, row := range fields.Sessions {
		// A terminal id and an activity are this workbench's own knowledge, so only
		// the sessions it has booted carry them.
		if state := reg.bySlug(row.Slug); state != nil {
			fields.Sessions[i].Terminal = state.terminal
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
	if snapshot.Capabilities.Children {
		summary.Coverage = status.AgentCoverageFull
	} else {
		summary.Coverage = status.AgentCoverageRoot
	}
	if snapshot.Capabilities.Attention {
		if snapshot.Attention {
			summary.Attention = status.AgentAttentionNeedsYou
		}
	} else {
		summary.Attention = status.AgentAttentionUnknown
	}
	return summary
}

func agentPanel(snapshot agentActivitySnapshot) status.AgentPanel {
	records := make([]status.AgentRecord, 0, len(snapshot.Records))
	for _, record := range snapshot.Records {
		records = append(records, status.AgentRecord{
			ID: record.ID, RunID: record.RunID, Provider: record.Provider, ParentID: record.ParentID,
			Type: record.Type, Role: record.Role, State: record.State,
			ParentKnown: record.ParentKnown, StartedAt: record.StartedAt, FinishedAt: record.FinishedAt,
		})
	}
	return status.AgentPanel{
		Provider:       snapshot.Provider,
		AttentionKnown: snapshot.Capabilities.Attention,
		ChildrenKnown:  snapshot.Capabilities.Children,
		ParentsKnown:   snapshot.Capabilities.Parents,
		OutcomesKnown:  snapshot.Capabilities.Outcomes,
		Agents:         records,
	}
}
