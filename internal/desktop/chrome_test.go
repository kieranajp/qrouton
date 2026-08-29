package desktop

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

func TestChromeKeepsAttentionActiveAgentsAndUnseenIndependent(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	backgroundRoot := sessionDir(t, root, "background")
	shownRoot := sessionDir(t, root, "onscreen")
	sessionDir(t, root, "cold")
	reg := newSessionsWithActivity(clock.now, time.Minute)
	background := reg.add(backgroundRoot, []string{"/bin/cat"}, os.Environ())
	shown := reg.add(shownRoot, []string{"/bin/cat"}, os.Environ())
	reg.reveal(shown)
	background.agents.begin(agentProviderClaude, 1)
	background.agents.lifecycle(workbench.DelegatedLifecycleRequest{
		Provider: agentProviderClaude, Generation: 1, Kind: workbench.LifecycleStart,
		ID: "lead-1", Type: "qrouton-research-lead",
	})
	background.agents.attention(1, status.ActivityWaiting)
	shown.agents.begin(agentProviderCodex, 2)

	renderer := newFakeRenderer()
	pushChrome(reg, root, nil, map[string][]status.RepoStat{}, map[string]int{"background": 3}, renderer.Emit)
	fields := pushedChrome(t, renderer)
	rows := map[string]status.SessionRow{}
	for _, row := range fields.Sessions {
		rows[row.Slug] = row
	}
	backgroundSummary := rows["background"].Summary
	if backgroundSummary.Attention != status.AgentAttentionNeedsYou || backgroundSummary.Active != 2 ||
		backgroundSummary.Coverage != status.AgentCoverageFull || !backgroundSummary.Running || rows["background"].Unseen != 3 {
		t.Fatalf("background row = %+v", rows["background"])
	}
	shownSummary := rows["onscreen"].Summary
	if shownSummary.Attention != status.AgentAttentionUnknown || shownSummary.Active != 1 ||
		shownSummary.Coverage != status.AgentCoverageFull || !shownSummary.Running {
		t.Fatalf("onscreen row = %+v", rows["onscreen"])
	}
	if cold := rows["cold"].Summary; cold.Running || cold.Coverage != status.AgentCoverageNone || cold.Attention != status.AgentAttentionNone {
		t.Fatalf("cold row summary = %+v", cold)
	}
	if fields.Agents.Provider != agentProviderCodex || len(fields.Agents.Agents) != 1 || !fields.Agents.ChildrenKnown || fields.Agents.AttentionKnown {
		t.Fatalf("selected panel = %+v", fields.Agents)
	}
}

func TestSelectedAgentDetailFollowsTheSelectedSlugAndKeepsReferenceStats(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	alphaRoot := sessionDir(t, root, "alpha")
	betaRoot := sessionDir(t, root, "beta")
	reg := newSessionsWithActivity(clock.now, time.Minute)
	alpha := reg.add(alphaRoot, []string{"/bin/cat"}, os.Environ())
	beta := reg.add(betaRoot, []string{"/bin/cat"}, os.Environ())
	alpha.agents.begin(agentProviderClaude, 10)
	beta.agents.begin(agentProviderOpenCode, 20)
	renderer := newFakeRenderer()
	reference := status.RepoStat{Name: "lifesum/contracts", Role: "reference"}

	reg.reveal(alpha)
	pushChrome(reg, root, nil, map[string][]status.RepoStat{alphaRoot: {reference}}, nil, renderer.Emit)
	fields := pushedChrome(t, renderer)
	if fields.Agents.Provider != agentProviderClaude || len(fields.Agents.Agents) != 1 {
		t.Fatalf("alpha detail = %+v", fields.Agents)
	}
	if len(fields.Repos) != 1 || fields.Repos[0] != reference {
		t.Fatalf("selected reference stats = %+v", fields.Repos)
	}

	reg.reveal(beta)
	pushChrome(reg, root, nil, map[string][]status.RepoStat{}, nil, renderer.Emit)
	fields = pushedChrome(t, renderer)
	if fields.Agents.Provider != agentProviderOpenCode || len(fields.Agents.Agents) != 1 {
		t.Fatalf("beta detail = %+v", fields.Agents)
	}
}

func TestSelectedColdSessionKeepsKnownManifestProvider(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, "cold")
	if err := session.WriteManifest(dir, session.Manifest{Slug: "cold", Runner: agentProviderCodex}); err != nil {
		t.Fatal(err)
	}
	reg := testRegistry(t, dir)
	renderer := newFakeRenderer()
	pushChrome(reg, root, nil, nil, nil, renderer.Emit)
	panel := pushedChrome(t, renderer).Agents
	if panel.Provider != agentProviderCodex || len(panel.Agents) != 0 || panel.AttentionKnown || panel.ChildrenKnown {
		t.Fatalf("cold known-provider panel = %+v", panel)
	}
}

func TestAgentPanelKeepsProviderIdentityPerRetainedRun(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	tracker := newAgentActivity(clock.now, time.Minute)
	tracker.begin(agentProviderClaude, 1)
	tracker.exit(0)
	clock.at = clock.at.Add(time.Second)
	tracker.begin(agentProviderCodex, 2)
	panel := agentPanel(tracker.snapshot())
	providers := map[string]string{}
	for _, record := range panel.Agents {
		providers[record.RunID] = record.Provider
	}
	if providers["1"] != agentProviderClaude || providers["2"] != agentProviderCodex {
		t.Fatalf("record providers = %+v", providers)
	}
}

type fakeChromeExpiryTimer struct {
	ch     chan time.Time
	resets chan time.Duration
}

func newFakeChromeExpiryTimer() *fakeChromeExpiryTimer {
	return &fakeChromeExpiryTimer{ch: make(chan time.Time, 1), resets: make(chan time.Duration, 8)}
}

func (t *fakeChromeExpiryTimer) channel() <-chan time.Time { return t.ch }
func (t *fakeChromeExpiryTimer) reset(after time.Duration) { t.resets <- after }
func (t *fakeChromeExpiryTimer) stop()                     {}

type chromeClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *chromeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *chromeClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(by)
}

func TestAgentExpiryTimerPushesPrunedChromeAtTheExactBoundary(t *testing.T) {
	clock := &chromeClock{at: time.Date(2026, 8, 28, 15, 0, 0, 0, time.UTC)}
	retention := 10 * time.Second
	root := t.TempDir()
	dir := sessionDir(t, root, "octopus")
	reg := newSessionsWithActivity(clock.now, retention)
	state := reg.add(dir, []string{"/bin/cat"}, os.Environ())
	reg.reveal(state)
	state.agents.begin(agentProviderClaude, 1)
	state.agents.exit(0)
	select {
	case <-reg.touched:
	default:
	}

	timer := newFakeChromeExpiryTimer()
	updates := make(chan status.Fields, 16)
	emit := func(event string, payload any) {
		if event == chromeEvent {
			updates <- payload.(status.Fields)
		}
	}
	counts := unseenCounts{
		all: func(string) map[string]int { return map[string]int{} },
		in:  func(string) (int, bool) { return 0, false },
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watchWithExpiryTimer(ctx, reg, root, nil, emit, time.Hour, time.Hour, counts, timer)

	initial := <-updates
	if len(initial.Agents.Agents) != 1 {
		t.Fatalf("initial recent agents = %+v", initial.Agents.Agents)
	}
	if after := <-timer.resets; after != retention {
		t.Fatalf("expiry timer reset to %s, want %s", after, retention)
	}
	clock.advance(retention - time.Millisecond)
	pushChrome(reg, root, nil, nil, nil, emit)
	if before := <-updates; len(before.Agents.Agents) != 1 {
		t.Fatalf("record pruned before boundary: %+v", before.Agents.Agents)
	}
	clock.advance(time.Millisecond)
	timer.ch <- clock.now()
	for {
		select {
		case fields := <-updates:
			if len(fields.Agents.Agents) == 0 {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("expiry timer did not publish the pruned activity panel")
		}
	}
}

func TestAgentPanelJSONAlwaysUsesAnArrayForRecords(t *testing.T) {
	reg := testRegistry(t, filepath.Join(t.TempDir(), "octopus"))
	renderer := newFakeRenderer()
	pushChrome(reg, "", nil, nil, nil, renderer.Emit)
	if fields := pushedChrome(t, renderer); fields.Agents.Agents == nil {
		t.Fatal("chrome emitted a nil agent record list")
	}
}
