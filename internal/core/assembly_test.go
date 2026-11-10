package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/gittest"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/ticket"
)

// Neither domain type is emittable as written: an error marshals to {} and an
// untagged field reaches the page under its Go name.
func TestTheRefreshAndProgressEventsRenderEveryFieldAndTheirErrors(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[RefreshEvent](), reflect.TypeFor[ProgressEvent]()} {
		for i := range typ.NumField() {
			if typ.Field(i).Tag.Get("json") == "" {
				t.Errorf("%s.%s reaches the page under its Go name", typ.Name(), typ.Field(i).Name)
			}
		}
	}

	refresh, err := json.Marshal(newRefreshEvent(4, github.RefreshMsg{
		Owner: "acme", State: github.RefreshFailed, Err: errors.New("unavailable")}))
	if err != nil {
		t.Fatal(err)
	}
	var decodedRefresh map[string]any
	if err := json.Unmarshal(refresh, &decodedRefresh); err != nil {
		t.Fatal(err)
	}
	if decodedRefresh["error"] != "unavailable" || decodedRefresh["generation"] != 4.0 {
		t.Fatalf("refresh event = %s", refresh)
	}

	repo := github.Repo{Org: "acme", Name: "api"}
	progress, err := json.Marshal(newProgressEvent("cleanup", session.Progress{
		Step: session.ProgressMirror, Status: session.ProgressFailed, Repo: &repo,
		Role: session.RepoRoleEditing, Err: errors.New("clone failed")}))
	if err != nil {
		t.Fatal(err)
	}
	var decodedProgress map[string]any
	if err := json.Unmarshal(progress, &decodedProgress); err != nil {
		t.Fatal(err)
	}
	if decodedProgress["error"] != "clone failed" || decodedProgress["repo"] != "acme/api" ||
		decodedProgress["session"] != "cleanup" || decodedProgress["role"] != "editing" {
		t.Fatalf("progress event = %s", progress)
	}
}

func TestRunnersOffersOnlyTheAgentsWithAResolvedPath(t *testing.T) {
	a := &Assembly{runners: func() ([]assembly.Runner, error) {
		return []assembly.Runner{
			{ID: "claude", Label: "Claude Code", Installed: true},
			{ID: "codex", Label: "Codex CLI"},
		}, nil
	}}
	got, err := a.Runners()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "claude" {
		t.Fatalf("runners = %+v", got)
	}
	if _, err := (&Assembly{}).Runners(); !errors.Is(err, ErrNoAgentCommand) {
		t.Fatalf("a workbench with no runners answered %v", err)
	}
}

// A repository the page names that the live list no longer holds is dropped, so
// a draft whose only editing repo has gone is refused rather than assembled.
func TestADraftDropsRepositoriesTheListNoLongerHolds(t *testing.T) {
	repos := &Repositories{cfg: &config.Config{}, errs: map[string]error{},
		repos: []github.Repo{{Org: "acme", Name: "api", DefaultBranch: "main"}}}
	a := &Assembly{cfg: &config.Config{}, repos: repos}

	in := DraftInput{Name: "Cleanup", Prefix: "feat", Repos: []RepoPick{
		{ID: "acme/api", Role: "editing"},
		{ID: "acme/gone", Role: "editing"},
	}}
	if got := a.draft(in); len(got.Repos) != 1 || got.Repos[0].Repo.Name != "api" {
		t.Fatalf("draft repos = %+v", got.Repos)
	}
	if problems := a.Check(in); len(problems) != 0 {
		t.Fatalf("a draft keeping one editing repo was refused: %+v", problems)
	}

	in.Repos = []RepoPick{{ID: "acme/gone", Role: "editing"}}
	problems := a.Check(in)
	if len(problems) != 1 || problems[0].Field != assembly.FieldRepos {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestPreviewAndPrefixesReachThePageThroughTheBinding(t *testing.T) {
	a := &Assembly{cfg: &config.Config{}, repos: &Repositories{cfg: &config.Config{}, errs: map[string]error{}}}
	if got := a.Preview(DraftInput{Name: "API Cleanup!", Entropy: "4f3a", Prefix: "fix"}); got != "fix/api-cleanup-4f3a" {
		t.Fatalf("preview = %q", got)
	}
	if got := a.Preview(DraftInput{Name: "Full ticket title", BranchDescription: "short title", Entropy: "4f3a",
		Ticket: "https://linear.app/issue/LIF-2841", Prefix: "fix"}); got != "fix/lif-2841-short-title" {
		t.Fatalf("Linear preview = %q", got)
	}
	if got := a.Prefixes(); len(got) == 0 || got[0] != "feat" {
		t.Fatalf("prefixes = %v", got)
	}
}

func TestAssemblyOfferQueuesAndClaimsOneCanonicalSeed(t *testing.T) {
	root := t.TempDir()
	a := newAssembly(&config.Config{Root: root}, nil, newSessions(), nil, nil, nil)
	got, err := a.offer("lif-2841", "Fix the regression")
	if err != nil || got != assembly.OutcomeQueued {
		t.Fatalf("offer = %q, %v", got, err)
	}
	if pending := a.Pending(); pending != "https://linear.app/issue/LIF-2841" {
		t.Fatalf("pending = %q", pending)
	}
	if ticket, prompt := a.pendingTicket(); ticket != "https://linear.app/issue/LIF-2841" || prompt != "Fix the regression" {
		t.Fatalf("pending Linear request = %q, %q", ticket, prompt)
	}
	seed := a.Begin()
	if seed.Ticket != "https://linear.app/issue/LIF-2841" || !regexp.MustCompile(`^[0-9a-f]{4}$`).MatchString(seed.Entropy) || seed.Generation == 0 || a.Pending() != "" {
		t.Fatalf("Begin = %+v, pending %q", seed, a.Pending())
	}
	if got, err := a.offer("https://linear.app/lifesum/issue/LIF-2841/title", "ignored duplicate"); err != nil || got != assembly.OutcomeDraft {
		t.Fatalf("same issue against open draft = %q, %v", got, err)
	}
	if _, err := a.offer("LIF-2842", "another task"); !errors.Is(err, assembly.ErrDraftConflict) {
		t.Fatalf("different issue against open draft = %v", err)
	}
}

func TestAssemblyManualDraftIsNeverTakenOver(t *testing.T) {
	a := newAssembly(&config.Config{Root: t.TempDir()}, nil, newSessions(), nil, nil, nil)
	manual := a.Begin()
	if manual.Ticket != "" {
		t.Fatalf("manual Begin = %+v", manual)
	}
	if _, err := a.offer("LIF-2841", ""); !errors.Is(err, assembly.ErrDraftConflict) {
		t.Fatalf("external issue took over a manual draft: %v", err)
	}
	a.End(manual.Generation)
	if got, err := a.offer("LIF-2841", ""); err != nil || got != assembly.OutcomeQueued {
		t.Fatalf("offer after cancel = %q, %v", got, err)
	}
}

func TestAssemblyEndCannotCloseAReplacementGeneration(t *testing.T) {
	a := newAssembly(&config.Config{Root: t.TempDir()}, nil, newSessions(), nil, nil, nil)
	if _, err := a.offer("LIF-2841", ""); err != nil {
		t.Fatal(err)
	}
	first := a.Begin()
	replacement := a.Begin()
	if replacement.Generation <= first.Generation || replacement.Ticket != first.Ticket {
		t.Fatalf("replacement = %+v after %+v", replacement, first)
	}
	a.End(first.Generation)
	if got, err := a.offer("LIF-2842", ""); !errors.Is(err, assembly.ErrDraftConflict) || got != "" {
		t.Fatalf("late End closed the replacement: %q, %v", got, err)
	}
	a.End(replacement.Generation)
	if got, err := a.offer("LIF-2842", ""); err != nil || got != assembly.OutcomeQueued {
		t.Fatalf("current End did not release the draft: %q, %v", got, err)
	}
}

func TestAssemblyOfferRevealsThePreferredMatchingSessionOnce(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	created := time.Now()
	for i, slug := range []string{"older", "shown", "newest"} {
		dir := sessionDir(t, root, slug)
		manifest := session.Manifest{Slug: slug, Name: slug, TicketURL: "https://linear.app/acme/issue/LIF-2841/title",
			Mode: session.ModeAssistant, CreatedAt: created.Add(time.Duration(i) * time.Hour)}
		if err := session.WriteManifest(dir, manifest); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.MarkOpened(filepath.Join(root, "shown"), created.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	a := newAssembly(&config.Config{Root: root}, nil, reg, nil, nil, nil)
	for range 2 {
		if got, err := a.offer("LIF-2841", ""); err != nil || got != assembly.OutcomeExisting {
			t.Fatalf("matching offer = %q, %v", got, err)
		}
	}
	if current := reg.Current(); current == nil || current.Slug() != "shown" {
		t.Fatalf("shown session = %v, want preferred match", current)
	}
	if agents, serves := boot.counts(); agents != 1 || serves != 1 {
		t.Fatalf("repeated match started %d agents and %d listeners", agents, serves)
	}
}

func TestAssemblyOfferSerializesDifferentSimultaneousIssues(t *testing.T) {
	a := newAssembly(&config.Config{Root: t.TempDir()}, nil, newSessions(), nil, nil, nil)
	start := make(chan struct{})
	type answer struct {
		issue string
		err   error
	}
	results := make(chan answer, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, issue := range []string{"LIF-2841", "LIF-2842"} {
		go func() {
			ready.Done()
			<-start
			_, err := a.offer(issue, "")
			results <- answer{issue: issue, err: err}
		}()
	}
	ready.Wait()
	close(start)
	var accepted, refused int
	var queued string
	for range 2 {
		got := <-results
		switch {
		case got.err == nil:
			accepted++
			queued = got.issue
		case errors.Is(got.err, assembly.ErrDraftConflict):
			refused++
		default:
			t.Fatalf("simultaneous offer = %v", got.err)
		}
	}
	if accepted != 1 || refused != 1 {
		t.Fatalf("simultaneous offers: %d accepted, %d refused", accepted, refused)
	}
	if pending := a.Pending(); pending != "https://linear.app/issue/"+queued {
		t.Fatalf("pending = %q after %q was the accepted offer", pending, queued)
	}
}

// A ticket already carried by a session boots it: a supervisor process and a
// control socket. An offer of another issue arriving during that boot is
// answered rather than stalled behind it.
func TestAssemblyOfferLeavesTheDraftFreeWhileAMatchingSessionBoots(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	dir := sessionDir(t, root, "shown")
	if err := session.WriteManifest(dir, session.Manifest{Slug: "shown", Name: "shown",
		TicketURL: "https://linear.app/acme/issue/LIF-2841/title", Mode: session.ModeAssistant,
		CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	a := newAssembly(&config.Config{Root: root}, nil, reg, nil, nil, nil)

	during := make(chan error, 1)
	boot.booting = func(req AgentRequest) {
		offered := make(chan error, 1)
		go func() {
			outcome, err := a.offer("LIF-2842", "")
			if err == nil && outcome != assembly.OutcomeQueued {
				err = fmt.Errorf("offer during a boot = %q", outcome)
			}
			offered <- err
		}()
		select {
		case err := <-offered:
			during <- err
		case <-time.After(2 * time.Second):
			during <- errors.New("an offer could not proceed while a matching session booted")
		}
	}

	if got, err := a.offer("LIF-2841", ""); err != nil || got != assembly.OutcomeExisting {
		t.Fatalf("matching offer = %q, %v", got, err)
	}
	if err := <-during; err != nil {
		t.Fatal(err)
	}
	if pending := a.Pending(); pending != "https://linear.app/issue/LIF-2842" {
		t.Fatalf("pending = %q, want the issue offered during the boot", pending)
	}
}

// Create is the whole of assembly from the overlay: the manifest, the worktrees,
// and a session that boots itself. A webview has no PTY to hand over, so the
// adoption is in process and the supervisor lands on a socket of its own.
func TestCreateAssemblesTheSessionAndBootsItOnItsOwnSocket(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	cfg := &config.Config{Root: root, Orgs: []string{"acme"}}
	repo := github.Repo{Org: "acme", Name: "api", SSHURL: gittest.Origin(t, "api"), DefaultBranch: "main"}
	repos := &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}

	var steps []ProgressEvent
	a := newAssembly(cfg, repos, reg, emitFunc(func(event string, payload any) {
		if event == assemblyProgressName {
			steps = append(steps, payload.(ProgressEvent))
		}
	}), nil, nil)

	in := DraftInput{Name: "API Cleanup", Entropy: "4f3a", Description: "tidy", Prefix: "fix", Mode: "rpi",
		Runner: "codex", Repos: []RepoPick{{ID: "acme/api", Role: "editing"}}}
	if err := a.Create(in); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(root, "api-cleanup-4f3a")
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "API Cleanup" || m.Description != "tidy" || m.EffectiveMode() != session.ModeRPI {
		t.Fatalf("manifest = %+v", m)
	}
	if m.Name != "API Cleanup" || m.Slug != "api-cleanup-4f3a" {
		t.Fatalf("session identity = %+v", m)
	}
	if len(m.Repos) != 1 || m.Repos[0].Role != session.RepoRoleEditing || m.Repos[0].Branch != "fix/api-cleanup-4f3a" {
		t.Fatalf("manifest repos = %+v", m.Repos)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "api", ".git")); err != nil {
		t.Fatal("the worktree was not checked out:", err)
	}
	if steps == nil {
		t.Fatal("assembly reported no progress")
	}
	for _, step := range steps {
		if step.Session != "api-cleanup-4f3a" {
			t.Fatalf("progress event names %q, not the session being assembled", step.Session)
		}
	}

	// Adopted with boot: the session came up here rather than being handed a PTY.
	if reg.Current().Root() != dir {
		t.Fatalf("the created session is not on screen: %q", reg.Current().Root())
	}
	if boot.sockets[dir] == "" {
		t.Fatal("the adopted session did not boot its own supervisor")
	}
	if boot.resumed(t, dir) {
		t.Fatal("a session assembled from nothing was told to resume a conversation")
	}
	if boot.runners[dir] != "codex" {
		t.Fatalf("the session booted the %q agent, not the one the overlay chose", boot.runners[dir])
	}
}

// The page names the mode it offers; a misspelt one must not quietly assemble
// the guided workflow in its place.
func TestCreateRefusesAModeTheSessionDoesNotHave(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Root: root}
	repos := &Repositories{cfg: cfg, errs: map[string]error{},
		repos: []github.Repo{{Org: "acme", Name: "api", DefaultBranch: "main"}}}
	a := newAssembly(cfg, repos, nil, nil, nil, nil)
	err := a.Create(DraftInput{Name: "Cleanup", Entropy: "4f3a", Prefix: "feat", Mode: "assistent",
		Repos: []RepoPick{{ID: "acme/api", Role: "editing"}}})
	if !errors.Is(err, session.ErrInvalidMode) {
		t.Fatalf("Create with an unknown mode answered %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "cleanup-4f3a")); !os.IsNotExist(err) {
		t.Fatalf("a refused mode still left a session directory: %v", err)
	}
}

func TestCreateRefusesADraftItsOwnRulesReject(t *testing.T) {
	cfg := &config.Config{Root: t.TempDir()}
	a := newAssembly(cfg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil, nil, nil, nil)
	err := a.Create(DraftInput{Name: "Cleanup", Prefix: "feat"})
	if err == nil || !strings.Contains(err.Error(), "editing") {
		t.Fatalf("Create with no editing repo answered %v", err)
	}
}

// A control whose effect expires after the first boot is worse than no control,
// so the session records the agent it was assembled with and every later boot
// reads it back.
func TestABootAfterTheFirstStartsTheAgentTheSessionRecords(t *testing.T) {
	root := t.TempDir()
	boot := newStubBoot("/bin/cat")
	reg, _, _ := testSessions(t, root, boot)
	cfg := &config.Config{Root: root, Orgs: []string{"acme"}}
	repo := github.Repo{Org: "acme", Name: "api", SSHURL: gittest.Origin(t, "api"), DefaultBranch: "main"}
	repos := &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}
	a := newAssembly(cfg, repos, reg, emitFunc(func(event string, payload any) {}), nil, nil)

	in := DraftInput{Name: "API Cleanup", Entropy: "4f3a", Prefix: "fix", Mode: "rpi", Runner: "codex",
		Repos: []RepoPick{{ID: "acme/api", Role: "editing"}}}
	if err := a.Create(in); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "api-cleanup-4f3a")

	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Runner != "codex" {
		t.Fatalf("manifest records runner %q, want the one the overlay chose", m.Runner)
	}

	// The rail booting it later names no runner, so the manifest's is what reaches
	// the agent command.
	if err := reg.Unload("api-cleanup-4f3a"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Show("api-cleanup-4f3a"); err != nil {
		t.Fatal(err)
	}
	if got := boot.runners[dir]; got != "codex" {
		t.Fatalf("the rail booted the %q agent, not the one the session records", got)
	}
}

func TestAssemblyOfferRevealsTheSessionHoldingAGitHubIssue(t *testing.T) {
	root := t.TempDir()
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	dir := sessionDir(t, root, "shown")
	if err := session.WriteManifest(dir, session.Manifest{Slug: "shown", Name: "shown",
		TicketURL: "https://github.com/Acme/API/issues/42", Mode: session.ModeAssistant,
		CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	a := newAssembly(&config.Config{Root: root}, nil, reg, nil, nil, nil)
	if got, err := a.offer("https://github.com/acme/api/issues/42?utm=x", ""); err != nil ||
		got != assembly.OutcomeExisting {
		t.Fatalf("matching offer = %q, %v", got, err)
	}
	if current := reg.Current(); current == nil || current.Slug() != "shown" {
		t.Fatalf("shown session = %v, want the matching session", current)
	}
}

func TestAssemblyOfferQueuesAnUnmatchedGitHubIssue(t *testing.T) {
	a := newAssembly(&config.Config{Root: t.TempDir()}, nil, newSessions(), nil, nil, nil)
	if got, err := a.offer("https://github.com/Acme/API/issues/42", ""); err != nil ||
		got != assembly.OutcomeQueued {
		t.Fatalf("offer = %q, %v", got, err)
	}
	if pending := a.Pending(); pending != "https://github.com/acme/api/issues/42" {
		t.Fatalf("pending = %q, want the canonical GitHub issue URL", pending)
	}
}

// A base names the branch a repository's work is cut from. One naming the
// repository's own default branch is no choice at all, so it is cleared and the
// manifest records nothing.
func TestABaseReachesTheSelectionUnlessItNamesTheDefaultBranch(t *testing.T) {
	repos := &Repositories{cfg: &config.Config{}, errs: map[string]error{},
		repos: []github.Repo{
			{Org: "acme", Name: "api", DefaultBranch: "main"},
			{Org: "acme", Name: "web", DefaultBranch: "main"},
		}}
	a := &Assembly{cfg: &config.Config{}, repos: repos}

	got := a.draft(DraftInput{Name: "Cleanup", Prefix: "feat", Repos: []RepoPick{
		{ID: "acme/api", Role: "editing", Base: "release/24.4"},
		{ID: "acme/web", Role: "reference", Base: "main"},
	}})
	if len(got.Repos) != 2 {
		t.Fatalf("draft repos = %+v", got.Repos)
	}
	if got.Repos[0].Base != "release/24.4" {
		t.Fatalf("chosen base = %q", got.Repos[0].Base)
	}
	if got.Repos[1].Base != "" {
		t.Fatalf("a base naming the default branch survived as %q", got.Repos[1].Base)
	}
}

// Catches CheckSlug answering with anything but the assembler's slug check: a
// taken slug is refused under the field the slug was seeded from, and a draft
// with no repositories raises no repository problem here.
func TestCheckSlugReturnsTheAssemblersProblemsForTheDraftsSlug(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Root: root}
	a := newAssembly(cfg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil, nil, nil, nil)
	named := DraftInput{Name: "Cleanup", Entropy: "4f3a", Prefix: "feat"}
	ticketed := DraftInput{Name: "Full title", BranchDescription: "short title", Entropy: "4f3a",
		Ticket: "https://linear.app/issue/LIF-2841", Prefix: "fix"}

	if problems := a.CheckSlug(named); len(problems) != 0 {
		t.Fatalf("a free slug was refused: %+v", problems)
	}
	for _, slug := range []string{"cleanup-4f3a", "lif-2841-short-title"} {
		if err := os.MkdirAll(filepath.Join(root, slug), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if problems := a.CheckSlug(named); len(problems) != 1 || problems[0].Field != assembly.FieldName {
		t.Fatalf("a taken name slug = %+v", problems)
	}
	if problems := a.CheckSlug(ticketed); len(problems) != 1 || problems[0].Field != assembly.FieldBranchDescription {
		t.Fatalf("a taken ticket slug = %+v", problems)
	}
}

// Catches Fetch dropping a field, deriving the branch description from anything
// but the title, or swallowing the fetch's error.
func TestFetchTurnsATicketIntoItsFieldsAndPassesAFetchErrorThrough(t *testing.T) {
	a := newAssembly(&config.Config{}, nil, nil, nil, nil, nil)
	var asked string
	var bounded bool
	a.fetch = func(ctx context.Context, url string) (ticket.Ticket, error) {
		asked = url
		_, bounded = ctx.Deadline()
		return ticket.Ticket{Title: "Fix the webhook retry storm", Body: "It retries forever."}, nil
	}
	got, err := a.Fetch("https://linear.app/issue/LIF-2841")
	if err != nil {
		t.Fatal(err)
	}
	want := TicketFields{Title: "Fix the webhook retry storm", Body: "It retries forever.",
		BranchDescription: assembly.SuggestBranchDescription("Fix the webhook retry storm")}
	if got != want || want.BranchDescription == "" {
		t.Fatalf("Fetch = %+v, want %+v", got, want)
	}
	if asked != "https://linear.app/issue/LIF-2841" || !bounded {
		t.Fatalf("fetch asked for %q, bounded %v", asked, bounded)
	}

	a.fetch = func(context.Context, string) (ticket.Ticket, error) { return ticket.Ticket{}, ticket.ErrTicketNotFound }
	if got, err := a.Fetch("https://linear.app/issue/LIF-2841"); !errors.Is(err, ticket.ErrTicketNotFound) || got != (TicketFields{}) {
		t.Fatalf("a failed fetch = %+v, %v", got, err)
	}
}
