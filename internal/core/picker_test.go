package core

import (
	"path/filepath"
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
	p := newPicker(cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil)

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
	p := newPicker(&config.Config{Root: filepath.Dir(shown)}, reg,
		&Repositories{errs: map[string]error{}}, nil)

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

	reg.Current().ClearPicker()
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
	p := newPicker(cfg, reg, repos, nil)

	if err := reg.QueuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := p.Cancel("shown"); err != nil {
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
	if err := p.Confirm("shown", in); err != nil {
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
	p := newPicker(cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil)

	if err := p.Confirm("kraken", PickerInput{}); err == nil {
		t.Fatal("a picker confirmed for a session this workbench is not running")
	}
	if err := p.Cancel("kraken"); err == nil {
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
	p := newPicker(cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil)

	in := PickerInput{Upgrades: []string{"org/docs", "org/docs", "org/kraken"}}
	if err := p.Confirm("reading", in); err != nil {
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
	p := newPicker(cfg, nil, repos, nil)

	got := p.draft(session.Manifest{Slug: "shown", Name: "Webhook retry"}, nil,
		PickerInput{Repos: []RepoPick{{ID: "org/svc", Role: "editing", Base: "release/24.4"}}})
	if len(got.Repos) != 1 || got.Repos[0].Base != "release/24.4" {
		t.Fatalf("draft repos = %+v", got.Repos)
	}
}
