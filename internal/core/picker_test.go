package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/gittest"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// pickerCore is a running core holding two booted sessions, one on screen.
func pickerCore(t *testing.T) (*Core, string, string) {
	t.Helper()
	opts, _ := testOptions(t)
	root := opts.Root
	opts.SessionRoot = ""
	shown, background := sessionDir(t, root, "shown"), sessionDir(t, root, "background")
	kernel := testWorkbench(t, newFakeClient(), opts)
	startWorkbench(t, kernel, opts)
	for _, slug := range []string{"background", "shown"} {
		if err := kernel.Sessions.Show(slug); err != nil {
			t.Fatal(err)
		}
	}
	return kernel, shown, background
}

// publishedChrome is the chrome a started core publishes once want holds of it.
func publishedChrome(t *testing.T, kernel *Core, want func(status.Fields) bool) status.Fields {
	t.Helper()
	var fields status.Fields
	waitFor(t, "the chrome to be published", func() bool {
		fields = kernel.Chrome.Snapshot()
		return want(fields)
	})
	return fields
}

// The picker names the branch anything added joins and locks what the session
// already holds, which is what keeps a repo the agent is working in from being
// cloned a second time.
func TestPickerLoadReportsTheSessionsBranchAndLocksWhatItHolds(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	reg := kernel.Sessions
	m := session.Manifest{Slug: "shown", Name: "Webhook retry", Repos: []session.ManifestRepo{
		{Name: "svc", Org: "org", Role: session.RepoRoleEditing, Branch: "fix/webhook-retry",
			DefaultBranch: "main", BaseBranch: "release/24.4"},
		{Name: "docs", Org: "org", Role: session.RepoRoleReference},
	}}
	if err := session.WriteManifest(shown, m); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Root: filepath.Dir(shown)}
	p := newPicker(context.Background(), cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil, nil)

	fields, err := p.Load("shown")
	if err != nil {
		t.Fatal(err)
	}
	if fields.Branch != "fix/webhook-retry" {
		t.Fatalf("picker branch = %q", fields.Branch)
	}
	if len(fields.Repos) != 2 {
		t.Fatalf("held repos = %+v", fields.Repos)
	}
	for _, held := range fields.Repos {
		if !held.Locked {
			t.Fatalf("%s is not locked; a repo the session holds is not composed again", held.ID)
		}
	}
	if fields.Repos[0].ID != "org/svc" || fields.Repos[0].Role != "editing" {
		t.Fatalf("held repo = %+v", fields.Repos[0])
	}
	if fields.Repos[0].Base != "release/24.4" {
		t.Fatalf("held row does not say what it was cut from: %+v", fields.Repos[0])
	}
	if fields.Repos[1].Base != "" {
		t.Fatalf("a row cut from the default branch names a base: %+v", fields.Repos[1])
	}
	if _, err := p.Load("kraken"); err == nil {
		t.Fatal("a picker loaded for a session this workbench is not running")
	}
}

func TestHeaderEscalationQueuesAPersistentPickerOnlyForAssistant(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	reg := kernel.Sessions
	p := newPicker(context.Background(), &config.Config{Root: filepath.Dir(shown)}, reg,
		&Repositories{errs: map[string]error{}}, nil, nil)

	if err := p.Escalate("shown"); err != nil {
		t.Fatal(err)
	}
	request := reg.Current().PendingPicker()
	if request == nil || !request.Deadline.IsZero() {
		t.Fatalf("header escalation request = %+v, want a persistent picker", request)
	}
	if fields := publishedChrome(t, kernel, func(f status.Fields) bool { return f.Picker }); !fields.Picker {
		t.Fatal("header escalation did not reach the shown session's chrome")
	}

	reg.Current().ClearPickerIf(request.ID)
	if err := session.SetMode(shown, session.ModeRPI); err != nil {
		t.Fatal(err)
	}
	if err := p.Escalate("shown"); err != nil {
		t.Fatal(err)
	}
	if request := reg.Current().PendingPicker(); request != nil {
		t.Fatalf("RPI session queued another escalation: %+v", request)
	}
	if err := p.Escalate("kraken"); err == nil {
		t.Fatal("header escalation accepted a session this workbench is not running")
	}
}

// Answering clears the request, so arriving at the session again draws nothing.
func TestConfirmAndCancelClearThePendingPicker(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	reg := kernel.Sessions
	cfg := &config.Config{Root: filepath.Dir(shown)}
	repo := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	repos := &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}
	p := newPicker(context.Background(), cfg, reg, repos, func(string, any) {}, nil)

	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := p.Cancel("shown", reg.Current().PendingPicker().ID); err != nil {
		t.Fatal(err)
	}
	if reg.Current().PendingPicker() != nil {
		t.Fatal("cancelling left the escalation waiting to be drawn again")
	}
	got, err := session.Load(shown)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation == nil || got.Escalation.Status != session.EscalationCancelled {
		t.Fatalf("cancelling an escalation wrote %+v", got.Escalation)
	}

	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	in := PickerInput{Repos: []RepoPick{{ID: "org/svc", Role: "editing"}}}
	if err := p.Confirm(t.Context(), "shown", reg.Current().PendingPicker().ID, in); err != nil {
		t.Fatal(err)
	}
	if reg.Current().PendingPicker() != nil {
		t.Fatal("confirming left the escalation waiting to be drawn again")
	}
	got, err = session.Load(shown)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation == nil || got.Escalation.Status != session.EscalationConfirmed {
		t.Fatalf("confirming an escalation wrote %+v", got.Escalation)
	}
	if len(got.Repos) != 1 || got.Repos[0].Name != "svc" {
		t.Fatalf("confirm did not add the repository: %+v", got.Repos)
	}
}

// Every answer is for a session this workbench is running, so a slug it does not
// hold is refused rather than answered against a session that is not there.
func TestConfirmAndCancelRefuseASessionThisWorkbenchIsNotRunning(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	reg := kernel.Sessions
	cfg := &config.Config{Root: filepath.Dir(shown)}
	p := newPicker(context.Background(), cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil, nil)

	if err := p.Confirm(t.Context(), "kraken", "", PickerInput{}); err == nil {
		t.Fatal("a picker confirmed for a session this workbench is not running")
	}
	if err := p.Cancel("kraken", ""); err == nil {
		t.Fatal("a picker cancelled for a session this workbench is not running")
	}
}

// Upgrades name rows against the manifest, not the cached repository list: the
// picker's held rows are the session's, and the page may be a refresh behind. An
// id the session does not read is dropped, and a repeated one counts once.
func TestConfirmTakesUpAHeldReferenceRepoAndIgnoresTheRest(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Root: root}
	dir, err := session.Create(cfg, session.CreateRequest{
		Name: "reading", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{
			Role: session.RepoRoleReference,
			Repo: github.Repo{Org: "org", Name: "docs", SSHURL: gittest.Origin(t, "docs"), DefaultBranch: "main"},
		}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	reg, _, _ := testSessions(t, root, newStubBoot("/bin/cat"))
	if err := reg.Show(filepath.Base(dir)); err != nil {
		t.Fatal(err)
	}
	p := newPicker(context.Background(), cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, func(string, any) {}, nil)

	in := PickerInput{Upgrades: []string{"org/docs", "org/docs", "org/kraken"}}
	if err := p.Confirm(t.Context(), "reading", "", in); err != nil {
		t.Fatal(err)
	}
	got, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 {
		t.Fatalf("repos after taking one up = %+v", got.Repos)
	}
	if r := got.Repos[0]; r.Role != session.RepoRoleEditing || r.Branch != "feat/reading" || r.Revision != "" {
		t.Fatalf("taken-up repo = %+v", r)
	}
}

// The editing-repo rule is short-circuited by the presence of an upgrade, so an
// id naming a repository the session is already editing must not reach the draft.
func TestHeldRefsKeepOnlyTheReferenceRowsNamedOnce(t *testing.T) {
	m := session.Manifest{Repos: []session.ManifestRepo{
		{Org: "org", Name: "svc", Role: session.RepoRoleEditing},
		{Org: "org", Name: "docs", Role: session.RepoRoleReference},
	}}
	got := heldRefs(m, []string{"org/svc", "org/docs", "org/docs", "org/kraken"})
	if len(got) != 1 || got[0] != (session.RepoRef{Org: "org", Name: "docs"}) {
		t.Fatalf("resolved refs = %+v", got)
	}
}

// The escalation picker speaks the same per-row shape as the first assembly, so
// a repository added later can be cut from a branch of its own too.
func TestAPickerRowCarriesItsChosenBaseIntoTheDraft(t *testing.T) {
	cfg := &config.Config{}
	repos := &Repositories{cfg: cfg, errs: map[string]error{},
		repos: []github.Repo{{Org: "org", Name: "svc", DefaultBranch: "main"}}}
	p := newPicker(context.Background(), cfg, nil, repos, nil, nil)

	got := p.draft(session.Manifest{Slug: "shown", Name: "Webhook retry"}, nil,
		PickerInput{Repos: []RepoPick{{ID: "org/svc", Role: "editing", Base: "release/24.4"}}})
	if len(got.Repos) != 1 || got.Repos[0].Base != "release/24.4" {
		t.Fatalf("draft repos = %+v", got.Repos)
	}
}

// holdClones puts a git on PATH whose clone makes its target, as git does, then
// never finishes. It reports once a clone has started; anything else fails at once.
func holdClones(t *testing.T) func() bool {
	t.Helper()
	dir := t.TempDir()
	started := filepath.Join(dir, "cloning")
	script := "#!/bin/sh\nif [ \"$1\" = clone ]; then for a; do target=$a; done; mkdir -p \"$target\"; : >" +
		strconv.Quote(started) + "; exec sleep 60; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() bool {
		_, err := os.Stat(started)
		return err == nil
	}
}

func addingPicker(ctx context.Context, kernel *Core, shown string) *Picker {
	cfg := &config.Config{Root: filepath.Dir(shown)}
	repo := github.Repo{Org: "org", Name: "svc", SSHURL: "git@example.invalid:org/svc.git", DefaultBranch: "main"}
	return newPicker(ctx, cfg, kernel.Sessions, &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}, func(string, any) {}, nil)
}

var pickSvc = PickerInput{Repos: []RepoPick{{ID: "org/svc", Role: "editing"}}}

func TestASecondAddForTheSameSessionIsRefused(t *testing.T) {
	cloning := holdClones(t)
	kernel, shown, _ := pickerCore(t)
	p := addingPicker(context.Background(), kernel, shown)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	first := make(chan error, 1)
	go func() { first <- p.Confirm(ctx, "shown", "", pickSvc) }()
	waitFor(t, "the first add to start cloning", cloning)

	// A deadline, so a broken refusal fails here rather than after the held clone.
	second, stop := context.WithTimeout(t.Context(), 2*time.Second)
	defer stop()
	if err := p.Confirm(second, "shown", "", pickSvc); !errors.Is(err, ErrAddRunning) {
		t.Fatalf("a second add while the first runs = %v, want %v", err, ErrAddRunning)
	}
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatalf("the cancelled add = %v", err)
	}
	ended, end := context.WithCancel(t.Context())
	end()
	if err := p.Confirm(ended, "shown", "", pickSvc); errors.Is(err, ErrAddRunning) {
		t.Fatal("an add that has returned still refuses the next one")
	}
}

func TestShutdownStopsARunningAdd(t *testing.T) {
	cloning := holdClones(t)
	kernel, shown, _ := pickerCore(t)
	kernel.Picker = addingPicker(kernel.ctx, kernel, shown)
	added := make(chan error, 1)
	go func() { added <- kernel.Picker.Confirm(context.Background(), "shown", "", pickSvc) }()
	waitFor(t, "the add to start cloning", cloning)

	kernel.Shutdown()
	partial := filepath.Join(filepath.Dir(shown), ".mirrors", "org", "svc.git.partial")
	if _, err := os.Lstat(partial); !os.IsNotExist(err) {
		t.Fatalf("Shutdown returned before the add cleaned up its partial clone: %v", err)
	}
	select {
	case err := <-added:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the add Shutdown stopped = %v", err)
		}
	case <-time.After(waitCeiling):
		t.Fatal("the add never returned")
	}
}

// The add modal follows the add through assembly progress keyed by the session's
// slug; a picker emitting nothing, or under another key, leaves its rows empty.
func TestPickerConfirmEmitsProgressForItsSession(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	cfg := &config.Config{Root: filepath.Dir(shown)}
	repo := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	var mu sync.Mutex
	var steps []progressEvent
	p := newPicker(context.Background(), cfg, kernel.Sessions, &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}},
		func(event string, payload any) {
			if event != assemblyProgressEvent {
				return
			}
			mu.Lock()
			steps = append(steps, payload.(progressEvent))
			mu.Unlock()
		}, nil)

	if err := p.Confirm(t.Context(), "shown", "", pickSvc); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	seen := map[string]bool{}
	for _, step := range steps {
		if step.Session != "shown" {
			t.Fatalf("progress event names %q, not the session being added to", step.Session)
		}
		if step.Repo == "org/svc" && step.Status == string(session.ProgressCompleted) {
			seen[step.Step] = true
		}
	}
	if !seen[string(session.ProgressMirror)] || !seen[string(session.ProgressWorktree)] {
		t.Fatalf("the add's mirror and worktree steps did not both complete: %+v", steps)
	}
}

// Abort ends only the add for the slug it names; an add running for another
// session carries on, and aborting a session with nothing running is not an error.
func TestAbortEndsTheRunningAddOnly(t *testing.T) {
	cloning := holdClones(t)
	kernel, shown, _ := pickerCore(t)
	p := addingPicker(context.Background(), kernel, shown)
	if err := p.Abort("background"); err != nil {
		t.Fatalf("aborting with no add running = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	added := make(chan error, 1)
	go func() { added <- p.Confirm(ctx, "shown", "", pickSvc) }()
	waitFor(t, "the add to start cloning", cloning)

	if err := p.Abort("background"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-added:
		t.Fatalf("aborting another session ended this add: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := p.Abort("shown"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-added:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the aborted add = %v", err)
		}
	case <-time.After(waitCeiling):
		t.Fatal("the aborted add never returned")
	}
}

func queueEscalation(t *testing.T, reg *Sessions, root string) *workbench.PickerRequest {
	t.Helper()
	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: root, Name: "Escalated", Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	return reg.BySlug(slugFor(root)).PendingPicker()
}

// A plain add's confirm once cleared whatever was pending, so an escalation that
// arrived mid-add was dropped and the agent's escalate call polled until it timed out.
func TestAnEscalationArrivingDuringAnAddStaysPending(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	reg := kernel.Sessions
	cfg := &config.Config{Root: filepath.Dir(shown)}
	repo := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	var arrived *workbench.PickerRequest
	var once sync.Once
	arriveMidAdd := func(string, any) { once.Do(func() { arrived = queueEscalation(t, reg, shown) }) }
	p := newPicker(context.Background(), cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}, arriveMidAdd, nil)

	if err := p.Confirm(t.Context(), "shown", "", pickSvc); err != nil {
		t.Fatal(err)
	}
	if arrived == nil {
		t.Fatal("the add emitted no progress, so no escalation arrived during it")
	}
	if pending := reg.Current().PendingPicker(); pending == nil || pending.ID != arrived.ID {
		t.Fatalf("after the add the pending escalation is %+v, want %+v", pending, arrived)
	}
	got, err := session.Load(shown)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation != nil || got.Name == arrived.Name {
		t.Fatalf("a plain add answered the escalation that arrived during it: %+v, name %q", got.Escalation, got.Name)
	}
}

// Cancel once read whatever was pending at cancel time, so dismissing a plain add
// recorded a newer escalation as cancelled and cleared it unseen.
func TestCancellingAnAddLeavesAPendingEscalationAlone(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	p := addingPicker(context.Background(), kernel, shown)
	waiting := queueEscalation(t, kernel.Sessions, shown)

	if err := p.Cancel("shown", ""); err != nil {
		t.Fatal(err)
	}
	if pending := kernel.Sessions.Current().PendingPicker(); pending == nil || pending.ID != waiting.ID {
		t.Fatalf("cancelling a plain add left %+v pending, want %+v", pending, waiting)
	}
	got, err := session.Load(shown)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation != nil {
		t.Fatalf("cancelling a plain add recorded the escalation as %+v", got.Escalation)
	}
}

// An overlay left open on a replaced request must not answer the one that replaced it.
func TestAPickerConfirmForAGoneRequestIsRefused(t *testing.T) {
	kernel, shown, _ := pickerCore(t)
	p := addingPicker(context.Background(), kernel, shown)
	gone := queueEscalation(t, kernel.Sessions, shown)
	current := queueEscalation(t, kernel.Sessions, shown)

	if err := p.Confirm(t.Context(), "shown", gone.ID, pickSvc); !errors.Is(err, ErrPickerStale) {
		t.Fatalf("confirming a replaced escalation = %v, want %v", err, ErrPickerStale)
	}
	if err := p.Cancel("shown", gone.ID); err != nil {
		t.Fatal(err)
	}
	if pending := kernel.Sessions.Current().PendingPicker(); pending == nil || pending.ID != current.ID {
		t.Fatalf("answering a replaced escalation left %+v pending, want %+v", pending, current)
	}
	got, err := session.Load(shown)
	if err != nil {
		t.Fatal(err)
	}
	if got.Escalation != nil || len(got.Repos) != 0 {
		t.Fatalf("a stale answer reached the session: escalation %+v, repos %+v", got.Escalation, got.Repos)
	}
}
