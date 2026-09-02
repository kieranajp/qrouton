package desktop

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/gittest"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
	"github.com/kieranajp/qrouton/internal/workbench"
)

// pickerWorkbench is a registry holding two booted sessions, one on screen.
func pickerWorkbench(t *testing.T) (*Sessions, string, string) {
	t.Helper()
	root := t.TempDir()
	shown, background := sessionDir(t, root, "shown"), sessionDir(t, root, "background")
	reg := newSessions()
	t.Cleanup(reg.stopAll)
	reg.reveal(reg.add(shown, []string{"/bin/cat"}, nil))
	reg.add(background, []string{"/bin/cat"}, nil)
	return reg, shown, background
}

// An escalation from a background session changes nothing on screen: the request
// waits on the session it names until the user arrives there.
func TestAQueuedPickerOnABackgroundSessionOpensNothing(t *testing.T) {
	reg, shown, background := pickerWorkbench(t)
	deadline := time.Now().Add(time.Minute)

	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: background, Deadline: deadline}); err != nil {
		t.Fatal(err)
	}
	if reg.current().root() != shown {
		t.Fatalf("queueing a picker moved the screen to %q", reg.current().root())
	}
	if reg.current().pendingPicker() != nil {
		t.Fatal("a background session's escalation reached the session on screen")
	}
	if reg.bySlug("background").pendingPicker() == nil {
		t.Fatal("the escalation did not wait on the session it names")
	}
}

// Arrival is what draws it, and the chrome poll is how arrival reaches the page.
func TestAPendingPickerReachesTheChromeOfTheSessionItNames(t *testing.T) {
	reg, _, background := pickerWorkbench(t)
	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: background, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}

	if fields := chromeOf(t, reg); fields.Picker {
		t.Fatal("another session's escalation drew a picker over the one on screen")
	}
	reg.reveal(reg.bySlug("background"))
	if fields := chromeOf(t, reg); !fields.Picker {
		t.Fatal("arriving at the session did not draw its picker")
	}
}

// The workbench never learns that awaitEscalation gave up, so an expired request
// is ignored rather than drawn for an answer nobody is polling for.
func TestAnExpiredPickerIsIgnoredOnArrival(t *testing.T) {
	reg, _, background := pickerWorkbench(t)
	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: background, Deadline: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	reg.reveal(reg.bySlug("background"))
	if fields := chromeOf(t, reg); fields.Picker {
		t.Fatal("an escalation whose poll has timed out was still drawn")
	}
}

// Two escalations for one session is the second replacing the first; both
// pollers then read the one stanza the confirm writes.
func TestASecondPickerRequestReplacesTheFirst(t *testing.T) {
	reg, shown, _ := pickerWorkbench(t)
	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(-time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if reg.current().pendingPicker() == nil {
		t.Fatal("a live request did not replace the expired one before it")
	}
}

func TestAPickerForASessionThisWorkbenchIsNotRunningIsRefused(t *testing.T) {
	reg, _, _ := pickerWorkbench(t)
	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: "/sessions/kraken", Deadline: time.Now().Add(time.Minute)}); err == nil {
		t.Fatal("a picker for an unknown session was queued")
	}
}

func TestOpPickerRefusesAnEmptyRootAsAnAnswer(t *testing.T) {
	r := newFakeRenderer()
	reg, _, windows := testWorkbench(t, r, r.Emit)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, reg.current(),
		controlHooks{picker: func(workbench.PickerRequest) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	host := (workbench.Handle{Socket: socket, SessionRoot: "/sessions/x"}).WindowHost()
	if err := host.Picker(t.Context(), workbench.PickerRequest{Deadline: time.Now()}); err == nil {
		t.Fatal("a picker with no session root succeeded")
	} else if !strings.Contains(err.Error(), ErrNoSessionRoot.Error()) {
		t.Fatalf("picker refusal = %v", err)
	}
}

// The picker names the branch anything added joins and locks what the session
// already holds, which is what keeps a repo the agent is working in from being
// cloned a second time.
func TestPickerLoadReportsTheSessionsBranchAndLocksWhatItHolds(t *testing.T) {
	reg, shown, _ := pickerWorkbench(t)
	m := session.Manifest{Slug: "shown", Name: "Webhook retry", Repos: []session.ManifestRepo{
		{Name: "svc", Org: "org", Role: session.RepoRoleEditing, Branch: "fix/webhook-retry"},
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
	if _, err := p.Load("kraken"); err == nil {
		t.Fatal("a picker loaded for a session this workbench is not running")
	}
}

func TestHeaderEscalationQueuesAPersistentPickerOnlyForAssistant(t *testing.T) {
	reg, shown, _ := pickerWorkbench(t)
	p := newPicker(&config.Config{Root: filepath.Dir(shown)}, reg,
		&Repositories{errs: map[string]error{}}, nil)

	if err := p.Escalate("shown"); err != nil {
		t.Fatal(err)
	}
	request := reg.current().pendingPicker()
	if request == nil || !request.Deadline.IsZero() {
		t.Fatalf("header escalation request = %+v, want a persistent picker", request)
	}
	if fields := chromeOf(t, reg); !fields.Picker {
		t.Fatal("header escalation did not reach the shown session's chrome")
	}

	reg.current().clearPicker()
	if err := session.SetMode(shown, session.ModeRPI); err != nil {
		t.Fatal(err)
	}
	if err := p.Escalate("shown"); err != nil {
		t.Fatal(err)
	}
	if request := reg.current().pendingPicker(); request != nil {
		t.Fatalf("RPI session queued another escalation: %+v", request)
	}
	if err := p.Escalate("kraken"); err == nil {
		t.Fatal("header escalation accepted a session this workbench is not running")
	}
}

// Answering clears the request, so arriving at the session again draws nothing.
func TestConfirmAndCancelClearThePendingPicker(t *testing.T) {
	reg, shown, _ := pickerWorkbench(t)
	cfg := &config.Config{Root: filepath.Dir(shown)}
	repo := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	repos := &Repositories{cfg: cfg, errs: map[string]error{}, repos: []github.Repo{repo}}
	p := newPicker(cfg, reg, repos, nil)

	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := p.Cancel("shown"); err != nil {
		t.Fatal(err)
	}
	if reg.current().pendingPicker() != nil {
		t.Fatal("cancelling left the escalation waiting to be drawn again")
	}
	got, err := session.Load(shown)
	if err != nil {
		t.Fatal(err)
	}
	if got.Picker == nil || got.Picker.Status != session.PickerCancelled {
		t.Fatalf("cancelling an escalation wrote %+v", got.Picker)
	}

	if err := reg.queuePicker(workbench.PickerRequest{SessionRoot: shown, Deadline: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	in := pickerInput{Repos: []repoPick{{ID: "org/svc", Role: "editing"}}}
	if err := p.Confirm("shown", in); err != nil {
		t.Fatal(err)
	}
	if reg.current().pendingPicker() != nil {
		t.Fatal("confirming left the escalation waiting to be drawn again")
	}
	got, err = session.Load(shown)
	if err != nil {
		t.Fatal(err)
	}
	if got.Picker == nil || got.Picker.Status != session.PickerConfirmed {
		t.Fatalf("confirming an escalation wrote %+v", got.Picker)
	}
	if len(got.Repos) != 1 || got.Repos[0].Name != "svc" {
		t.Fatalf("confirm did not add the repository: %+v", got.Repos)
	}
}

// Every answer is for a session this workbench is running, so a slug it does not
// hold is refused rather than answered against a session that is not there.
func TestConfirmAndCancelRefuseASessionThisWorkbenchIsNotRunning(t *testing.T) {
	reg, shown, _ := pickerWorkbench(t)
	cfg := &config.Config{Root: filepath.Dir(shown)}
	p := newPicker(cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil)

	if err := p.Confirm("kraken", pickerInput{}); err == nil {
		t.Fatal("a picker confirmed for a session this workbench is not running")
	}
	if err := p.Cancel("kraken"); err == nil {
		t.Fatal("a picker cancelled for a session this workbench is not running")
	}
}

func chromeOf(t *testing.T, reg *Sessions) status.Fields {
	t.Helper()
	var fields status.Fields
	pushChrome(reg, "", nil, map[string][]status.RepoStat{}, map[string]int{},
		func(event string, payload any) {
			if event == chromeEvent {
				fields = payload.(status.Fields)
			}
		})
	return fields
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
	reg := newSessions()
	t.Cleanup(reg.stopAll)
	reg.reveal(reg.add(dir, []string{"/bin/cat"}, nil))
	p := newPicker(cfg, reg, &Repositories{cfg: cfg, errs: map[string]error{}}, nil)

	in := pickerInput{Upgrades: []string{"org/docs", "org/docs", "org/kraken"}}
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
