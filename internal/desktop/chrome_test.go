package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/agentevent"
	"github.com/kieranajp/qrouton/internal/config"
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
	reg := newSessionsWithActivity(clock.now)
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
	if backgroundSummary.Attention != status.AgentAttentionUnread || backgroundSummary.Active != 2 ||
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
	if tree := agentTree(shown.agents.snapshot(), "", ""); tree.Provider != agentProviderCodex || len(tree.Nodes) != 1 || tree.Coverage == "" {
		t.Fatalf("selected tree = %+v", tree)
	}
}

func TestSelectedRepoStatsFollowTheSelectedSlug(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	alphaRoot := sessionDir(t, root, "alpha")
	betaRoot := sessionDir(t, root, "beta")
	reg := newSessionsWithActivity(clock.now)
	alpha := reg.add(alphaRoot, []string{"/bin/cat"}, os.Environ())
	beta := reg.add(betaRoot, []string{"/bin/cat"}, os.Environ())
	alpha.agents.begin(agentProviderClaude, 10)
	beta.agents.begin(agentProviderOpenCode, 20)
	renderer := newFakeRenderer()
	reference := status.RepoStat{Name: "lifesum/contracts", Role: "reference"}

	reg.reveal(alpha)
	pushChrome(reg, root, nil, map[string][]status.RepoStat{alphaRoot: {reference}}, nil, renderer.Emit)
	fields := pushedChrome(t, renderer)
	if len(fields.Repos) != 1 || fields.Repos[0] != reference {
		t.Fatalf("selected reference stats = %+v", fields.Repos)
	}

	reg.reveal(beta)
	pushChrome(reg, root, nil, map[string][]status.RepoStat{}, nil, renderer.Emit)
	if fields = pushedChrome(t, renderer); len(fields.Repos) != 0 {
		t.Fatalf("beta inherited alpha's stats: %+v", fields.Repos)
	}
}

func TestAColdSessionsTreeNamesItsManifestRunner(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, "cold")
	if err := session.WriteManifest(dir, session.Manifest{Slug: "cold", Runner: agentProviderCodex}); err != nil {
		t.Fatal(err)
	}
	tree := agentTree(agentActivitySnapshot{}, manifestRunner(dir), "")
	if tree.Provider != agentProviderCodex || len(tree.Nodes) != 0 || tree.Coverage != fmt.Sprintf(agentCoverageNoTools, tree.ProviderLabel) {
		t.Fatalf("cold tree = %+v", tree)
	}
}

func TestChromePublishesDefaultAndCustomStickersAndRefreshedRows(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, "octopus")
	reg := testRegistry(t, dir)
	renderer := newFakeRenderer()

	pushChrome(reg, root, nil, nil, nil, renderer.Emit)
	got := pushedChrome(t, renderer).Stickers
	if len(got) != 4 || got[0] != (status.Sticker{ID: "star", Colour: "blue", Symbol: "star", Label: "Important"}) ||
		got[3] != (status.Sticker{ID: "exclamation", Colour: "red", Symbol: "exclamation", Label: "Has bugs"}) {
		t.Fatalf("default stickers = %#v", got)
	}
	if _, err := session.CycleSticker(dir, []session.Sticker{"a1b2c3d4"}); err != nil {
		t.Fatal(err)
	}
	custom := []config.Sticker{{ID: "a1b2c3d4", Colour: "pink", Symbol: "heart", Label: "Ship it"}}
	pushChrome(reg, root, &config.Config{Stickers: &custom}, nil, nil, renderer.Emit)
	fields := pushedChrome(t, renderer)
	if want := []status.Sticker{{ID: "a1b2c3d4", Colour: "pink", Symbol: "heart", Label: "Ship it"}}; !reflect.DeepEqual(fields.Stickers, want) {
		t.Fatalf("custom stickers = %#v", fields.Stickers)
	}
	if len(fields.Sessions) != 1 || fields.Sessions[0].Sticker != "a1b2c3d4" {
		t.Fatalf("refreshed row = %#v", fields.Sessions)
	}
}

func TestChromeReadsOneLiveConfigGenerationDuringReplacement(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	first := config.StickerLabels{Star: "first", Bookmark: "first", Question: "first", Exclamation: "first"}
	second := config.StickerLabels{Star: "second", Bookmark: "second", Question: "second", Exclamation: "second"}
	cfg := &config.Config{Root: root, StickerLabels: &first}
	firstGeneration := &config.Config{Root: root, StickerLabels: &first}
	secondGeneration := &config.Config{Root: root, Welcomed: true, StickerLabels: &second}

	start := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-start
		for range 1000 {
			cfg.Replace(firstGeneration)
			cfg.Replace(secondGeneration)
		}
	}()
	close(start)
	for range 1000 {
		pushChrome(newSessions(), root, cfg, nil, nil, func(_ string, payload any) {
			fields := payload.(status.Fields)
			switch fields.Stickers[0].Label {
			case "first":
				if !fields.Welcoming {
					t.Error("chrome mixed first labels with the second welcome state")
				}
			case "second":
				if fields.Welcoming {
					t.Error("chrome mixed second labels with the first welcome state")
				}
			default:
				t.Errorf("chrome published unknown labels: %+v", fields.Stickers)
			}
		})
	}
	<-done
}

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

func TestAgentTreeJSONNeverCarriesANullList(t *testing.T) {
	tracker := newAgentActivity(time.Now)
	tracker.begin(agentProviderClaude, 1)
	for _, tree := range []status.AgentTree{agentTree(agentActivitySnapshot{}, "", ""), agentTree(tracker.snapshot(), "", "")} {
		body, err := json.Marshal(tree)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "null") {
			t.Fatalf("tree marshals a null: %s", body)
		}
	}
}

func TestRepositoryChangeImmediatelyReplacesCachedMeasurementsWithManifestRows(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, "octopus")
	initial := session.Manifest{Slug: "octopus", Repos: []session.ManifestRepo{
		{Org: "org", Name: "docs", Role: session.RepoRoleReference, WorktreePath: "src/docs"},
	}}
	if err := session.WriteManifest(dir, initial); err != nil {
		t.Fatal(err)
	}
	reg := testRegistry(t, dir)
	select {
	case <-reg.touched:
	default:
	}
	updates := make(chan status.Fields, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watch(ctx, reg, root, nil, func(event string, payload any) {
		if event == chromeEvent {
			updates <- payload.(status.Fields)
		}
	}, time.Hour, time.Hour, unseenCounts{
		all: func(string) map[string]int { return map[string]int{} },
		in:  func(string) (int, bool) { return 0, false },
	})

	for pushes := 0; pushes < 2; pushes++ {
		select {
		case <-updates:
		case <-time.After(5 * time.Second):
			t.Fatal("initial repository measurement did not settle")
		}
	}
	updated := initial
	updated.Repos = append([]session.ManifestRepo(nil), initial.Repos...)
	updated.Repos[0].Role = session.RepoRoleEditing
	updated.Repos = append(updated.Repos,
		session.ManifestRepo{Org: "org", Name: "svc", Role: session.RepoRoleEditing, WorktreePath: "src/svc"})
	if err := session.WriteManifest(dir, updated); err != nil {
		t.Fatal(err)
	}
	reg.repositoriesChanged(dir)

	select {
	case fields := <-updates:
		if len(fields.Repos) != 2 || fields.Repos[0] != (status.RepoStat{Name: "org/docs", Role: "editing"}) ||
			fields.Repos[1] != (status.RepoStat{Name: "org/svc", Role: "editing"}) {
			t.Fatalf("repository change pushed %+v", fields.Repos)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("repository change did not push chrome immediately")
	}
}

// Chrome sits on the emitter seam, so a payload it does not recognise has to
// reach the page rather than take the workbench down.
func TestChromeForwardsPayloadsThatAreNotChromeState(t *testing.T) {
	var events []string
	var payloads []any
	chrome := newChrome(func(event string, payload any) {
		events = append(events, event)
		payloads = append(payloads, payload)
	})

	foreign := windowDataEvent + "w1"
	chrome.publish(foreign, "cGluZw==")
	if len(events) != 1 || events[0] != foreign || payloads[0] != "cGluZw==" {
		t.Fatalf("forwarded events = %v, payloads = %v", events, payloads)
	}
	if snapshot := chrome.Snapshot(); snapshot.Activity != "" || len(snapshot.Sessions) != 0 {
		t.Fatalf("a foreign payload changed the stored chrome: %+v", snapshot)
	}

	fields := status.EmptyFields()
	fields.Activity = "working"
	chrome.publishFields(fields)
	chrome.publishFields(fields)
	if len(events) != 2 || events[1] != chromeEvent {
		t.Fatalf("typed publishes = %v, want one chrome event", events)
	}
	if chrome.Snapshot().Activity != "working" {
		t.Fatalf("snapshot = %+v", chrome.Snapshot())
	}
}

// The rail row and the Overview tab read one tracker, so a terminal-timed
// session's activity reaches the page as one answer in two vocabularies.
func TestRailRowAndAgentTreeAgreeOnOneSessionsActivity(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now)
	shown := reg.add(sessionDir(t, root, "octopus"), []string{"/bin/cat"}, os.Environ())
	reg.reveal(shown)
	shown.agents.begin(agentProviderOpenCode, 1)

	for _, step := range []struct {
		name   string
		poke   func()
		rail   string
		record string
	}{
		{"output", shown.agents.output, status.ActivityWorking, agentStateWorking},
		{"attention", func() { shown.agents.attention(1, status.ActivityWaiting) }, status.ActivityWaiting, agentStateYourTurn},
		{"typing", shown.agents.input, status.ActivityWorking, agentStateWorking},
		{"silence", func() { clock.at = clock.at.Add(activityQuiet) }, status.ActivityIdle, agentStateIdle},
	} {
		step.poke()
		renderer := newFakeRenderer()
		pushChrome(reg, root, nil, nil, nil, renderer.Emit)
		fields := pushedChrome(t, renderer)
		if fields.Activity != step.rail {
			t.Fatalf("%s: chrome activity = %q, want %q", step.name, fields.Activity, step.rail)
		}
		if len(fields.Sessions) != 1 || fields.Sessions[0].Activity != step.rail {
			t.Fatalf("%s: rail rows = %+v, want activity %q", step.name, fields.Sessions, step.rail)
		}
		if got := treeRoot(t, agentTree(shown.agents.snapshot(), "", "")).State; got != step.record {
			t.Fatalf("%s: tree root reads %q while the rail reads %q", step.name, got, step.rail)
		}
	}
}

func treeRoot(t *testing.T, tree status.AgentTree) status.AgentNode {
	t.Helper()
	for _, node := range tree.Nodes {
		if node.Root {
			return node
		}
	}
	t.Fatalf("no root node in %+v", tree.Nodes)
	return status.AgentNode{}
}

// A stage mark pulses while its lead runs in the current run, and stops when
// the lead stops or the run is relaunched.
func TestChromeStagesLiveFollowsTheResearchLead(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now)
	shown := reg.add(sessionDir(t, root, "onscreen"), []string{"/bin/cat"}, os.Environ())
	reg.reveal(shown)
	live := func() status.Stages {
		renderer := newFakeRenderer()
		pushChrome(reg, root, nil, map[string][]status.RepoStat{}, map[string]int{}, renderer.Emit)
		return pushedChrome(t, renderer).StagesLive
	}
	lead := func(hook string) workbench.AgentEventRequest {
		return workbench.AgentEventRequest{
			Provider: agentProviderClaude, Generation: 1, Hook: hook,
			AgentID: "lead-1", AgentType: "qrouton-research-lead", Timestamp: clock.now(),
		}
	}

	shown.agents.begin(agentProviderClaude, 1)
	shown.agents.apply(lead(agentevent.HookSubagentStart))
	if got := live(); got != (status.Stages{Research: true}) {
		t.Fatalf("live stages with the research lead running = %+v", got)
	}
	shown.agents.apply(lead(agentevent.HookSubagentStop))
	if got := live(); got != (status.Stages{}) {
		t.Fatalf("live stages after the lead stopped = %+v", got)
	}

	shown.agents.begin(agentProviderClaude, 2)
	start := lead(agentevent.HookSubagentStart)
	start.Generation, start.AgentID = 2, "lead-2"
	shown.agents.apply(start)
	shown.agents.begin(agentProviderClaude, 3)
	if got := live(); got != (status.Stages{}) {
		t.Fatalf("live stages after a relaunch = %+v", got)
	}
}

// A background session's row carries the stage of its live lead, and drops it
// when the lead stops.
func TestChromeRowStageFollowsABackgroundSessionsLead(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now)
	background := reg.add(sessionDir(t, root, "background"), []string{"/bin/cat"}, os.Environ())
	shown := reg.add(sessionDir(t, root, "onscreen"), []string{"/bin/cat"}, os.Environ())
	reg.reveal(shown)
	stage := func() string {
		renderer := newFakeRenderer()
		pushChrome(reg, root, nil, map[string][]status.RepoStat{}, map[string]int{}, renderer.Emit)
		for _, r := range pushedChrome(t, renderer).Sessions {
			if r.Slug == "background" {
				return r.Summary.Stage
			}
		}
		t.Fatal("no background row")
		return ""
	}
	lead := func(hook string) workbench.AgentEventRequest {
		return workbench.AgentEventRequest{
			Provider: agentProviderClaude, Generation: 1, Hook: hook,
			AgentID: "lead-1", AgentType: "qrouton-research-lead", Timestamp: clock.now(),
		}
	}

	background.agents.begin(agentProviderClaude, 1)
	background.agents.apply(lead(agentevent.HookSubagentStart))
	if got := stage(); got != "research" {
		t.Fatalf("stage with the research lead running = %q", got)
	}
	background.agents.apply(lead(agentevent.HookSubagentStop))
	if got := stage(); got != "" {
		t.Fatalf("stage after the lead stopped = %q", got)
	}
}

func TestAgentSummaryCarriesABackgroundSessionsHeadline(t *testing.T) {
	clock := &activityClock{at: time.Now()}
	root := t.TempDir()
	reg := newSessionsWithActivity(clock.now)
	background := reg.add(sessionDir(t, root, "background"), []string{"/bin/cat"}, os.Environ())
	shown := reg.add(sessionDir(t, root, "onscreen"), []string{"/bin/cat"}, os.Environ())
	reg.reveal(shown)
	row := func() status.AgentSummary {
		renderer := newFakeRenderer()
		pushChrome(reg, root, nil, map[string][]status.RepoStat{}, map[string]int{}, renderer.Emit)
		for _, r := range pushedChrome(t, renderer).Sessions {
			if r.Slug == "background" {
				return r.Summary
			}
		}
		t.Fatal("no background row")
		return status.AgentSummary{}
	}
	event := func(hook, tool, id, target string) workbench.AgentEventRequest {
		return workbench.AgentEventRequest{
			Provider: agentProviderClaude, Generation: 1, Hook: hook, Tool: tool, ToolUseID: id, Target: target, Timestamp: clock.now(),
		}
	}

	background.agents.begin(agentProviderClaude, 1)
	background.agents.apply(event(agentevent.HookUserPromptSubmit, "", "", ""))
	background.agents.apply(event(agentevent.HookPreToolUse, "Bash", "t1", "go test ./..."))
	if got := row(); got.State != agentStateWorking || got.Tool != "Bash" {
		t.Fatalf("working headline = %+v", got)
	}
	background.agents.apply(event(agentevent.HookPostToolUse, "Bash", "t1", "go test ./..."))
	background.agents.apply(event(agentevent.HookPreToolUse, agentevent.ToolAskUserQuestion, "ask-1", "Which branch?"))
	if got := row(); got.State != agentStateNeedsYou || got.Reason != "Which branch?" || got.Attention != status.AgentAttentionUnread {
		t.Fatalf("needs-you headline = %+v", got)
	}
}
