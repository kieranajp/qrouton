package desktop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/assembly"
	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/gittest"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
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
	if got.Escalation == nil || got.Escalation.Status != session.EscalationCancelled {
		t.Fatalf("cancelling an escalation wrote %+v", got.Escalation)
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

// offeredGitHub answers every refresh with one fixed list, which is the
// vocabulary an agent's add resolves its names against.
func offeredGitHub(offered []github.Repo) gh {
	return gh{
		token: func() (string, error) { return "t", nil },
		all: func(context.Context, string, []string, []github.Repo) <-chan github.RefreshMsg {
			ch := make(chan github.RefreshMsg, 1)
			ch <- github.RefreshMsg{State: github.RefreshComplete, Repos: offered}
			close(ch)
			return ch
		},
		one: func(context.Context, string, string) ([]github.Repo, error) { return offered, nil },
	}
}

// agentPicker is a picker over a live session, wired to compose for real against
// gittest origins. The signal it is given records rather than fires: an add that
// signals would SIGTERM the very process waiting for its reply, so every test
// here can assert nothing did.
func agentPicker(t *testing.T, req session.CreateRequest, offered []github.Repo) (*Picker, string, *[]string) {
	t.Helper()
	cfg := &config.Config{Root: t.TempDir(), Orgs: []string{"org"}}
	dir, err := session.Create(cfg, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	reg := newSessions()
	t.Cleanup(reg.stopAll)
	reg.reveal(reg.add(dir, []string{"/bin/cat"}, nil))
	repos := &Repositories{
		cfg: cfg, errs: map[string]error{}, gh: offeredGitHub(offered),
		emit: func(string, any) {},
	}
	var signalled []string
	return newPicker(cfg, reg, repos, func(root string) {
		signalled = append(signalled, root)
	}), dir, &signalled
}

// assertUndisturbed is the whole of the suppression, checked from the caller an
// agent actually reaches: no supervisor signal, and no notice queued for a
// relaunch to hand over.
func assertUndisturbed(t *testing.T, dir string, signalled *[]string) {
	t.Helper()
	if len(*signalled) != 0 {
		t.Fatalf("the add signalled %v; that would kill the agent waiting for its reply", *signalled)
	}
	if _, err := os.Stat(sessionpaths.AgentNotice(dir)); !os.IsNotExist(err) {
		t.Fatalf("the add queued an agent notice (stat err = %v)", err)
	}
}

func orEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// wantAdd spells the result an add returns, whose lists are empty rather than
// nil — so a partial literal would not compare equal.
func wantAdd(added, promoted, held []string) addReposResult {
	return addReposResult{Added: orEmpty(added), Promoted: orEmpty(promoted), Held: orEmpty(held)}
}

func repoNamed(t *testing.T, m session.Manifest, name string) session.ManifestRepo {
	t.Helper()
	for _, r := range m.Repos {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("manifest holds no repository named %q: %+v", name, m.Repos)
	return session.ManifestRepo{}
}

// headBranch is the branch a worktree is on, empty when detached.
func headBranch(t *testing.T, wt string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", wt, "symbolic-ref", "--short", "-q", "HEAD").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestAddComposesAFreshEditingRepoOntoTheSessionBranch(t *testing.T) {
	svc := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	extra := github.Repo{Org: "org", Name: "extra", SSHURL: gittest.Origin(t, "extra"), DefaultBranch: "main"}
	p, dir, signalled := agentPicker(t, session.CreateRequest{
		Name: "adding", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{Role: session.RepoRoleEditing, Repo: svc}},
	}, []github.Repo{svc, extra})

	got, err := p.add("adding", []repoAddition{{Name: "org/extra", Role: "editing"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantAdd([]string{"org/extra"}, nil, nil)) {
		t.Fatalf("add = %+v, want org/extra added and nothing else", got)
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repos) != 2 {
		t.Fatalf("manifest gained %d entries, want exactly one: %+v", len(m.Repos)-1, m.Repos)
	}
	added := repoNamed(t, m, "extra")
	if added.Role != session.RepoRoleEditing || added.Branch != "feat/adding" || added.Revision != "" {
		t.Fatalf("composed editing repo = %+v", added)
	}
	wt := filepath.Join(dir, added.WorktreePath)
	if got := headBranch(t, wt); got != "feat/adding" {
		t.Fatalf("composed worktree is on %q, want the session branch", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "extra")); err != nil {
		t.Fatalf("composed worktree is not under the session's src/: %v", err)
	}
	assertUndisturbed(t, dir, signalled)
}

// The default role reads rather than writes, so an unqualified add must not cut a
// branch or move the session's own.
func TestAddComposesAFreshReferenceRepoDetachedAndLeavesTheBranchAlone(t *testing.T) {
	svc := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	extra := github.Repo{Org: "org", Name: "extra", SSHURL: gittest.Origin(t, "extra"), DefaultBranch: "main"}
	p, dir, signalled := agentPicker(t, session.CreateRequest{
		Name: "adding", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{Role: session.RepoRoleEditing, Repo: svc}},
	}, []github.Repo{svc, extra})

	got, err := p.add("adding", []repoAddition{{Name: "org/extra"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Added, []string{"org/extra"}) {
		t.Fatalf("add = %+v, want org/extra added", got)
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	added := repoNamed(t, m, "extra")
	if added.Role != session.RepoRoleReference || added.Branch != "" || added.Revision == "" {
		t.Fatalf("composed reference repo = %+v, want detached at a pinned revision", added)
	}
	if got := headBranch(t, filepath.Join(dir, added.WorktreePath)); got != "" {
		t.Fatalf("reference worktree is on branch %q, want a detached HEAD", got)
	}
	if held := repoNamed(t, m, "svc"); held.Branch != "feat/adding" || held.Role != session.RepoRoleEditing {
		t.Fatalf("adding a reference repo disturbed the session's own repo: %+v", held)
	}
	assertUndisturbed(t, dir, signalled)
}

// Idempotence, and the no-demotion rule. Both directions land in Held, so the
// reply can say "already there" rather than the tool erroring or, worse,
// detaching a checkout out from under uncommitted work.
func TestAddReportsARepoItAlreadyHoldsWithoutRecomposingIt(t *testing.T) {
	svc := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	for _, tc := range []struct {
		name string
		role string
	}{
		{"the role it already holds", "editing"},
		{"reference, which would be a demotion", "reference"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, dir, signalled := agentPicker(t, session.CreateRequest{
				Name: "adding", Prefix: "feat", Mode: session.ModeAssistant,
				Repos: []session.RepoSelection{{Role: session.RepoRoleEditing, Repo: svc}},
			}, []github.Repo{svc})
			before, err := session.Load(dir)
			if err != nil {
				t.Fatal(err)
			}

			got, err := p.add("adding", []repoAddition{{Name: "org/svc", Role: tc.role}})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, wantAdd(nil, nil, []string{"org/svc"})) {
				t.Fatalf("add = %+v, want org/svc held and nothing composed", got)
			}
			m, err := session.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(m.Repos, before.Repos) {
				t.Fatalf("repos changed from %+v to %+v", before.Repos, m.Repos)
			}
			assertUndisturbed(t, dir, signalled)
		})
	}
}

// A session that edits nothing and takes nothing up has no business acquiring
// only more reading; that is what escalation is for.
func TestAddRefusesAnAllReferenceAddToASessionThatEditsNothing(t *testing.T) {
	docs := github.Repo{Org: "org", Name: "docs", SSHURL: gittest.Origin(t, "docs"), DefaultBranch: "main"}
	extra := github.Repo{Org: "org", Name: "extra", SSHURL: gittest.Origin(t, "extra"), DefaultBranch: "main"}
	p, dir, signalled := agentPicker(t, session.CreateRequest{
		Name: "reading", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{Role: session.RepoRoleReference, Repo: docs}},
	}, []github.Repo{docs, extra})

	_, err := p.add("reading", []repoAddition{{Name: "org/extra"}})
	if err == nil {
		t.Fatal("an all-reference add to a session with no editing repo was allowed")
	}
	assertUndisturbed(t, dir, signalled)
	// "repos" alone is a substring of "repository", so it would accept a resolve
	// or compose failure too; the field prefix is what names the reason.
	if !strings.HasPrefix(err.Error(), string(assembly.FieldRepos)+": ") {
		t.Fatalf("refusal = %v, want the repos field's problem", err)
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("a refused add composed something: %+v", m.Repos)
	}
}

// Promotion is the move an agent that started out reading actually needs, and it
// satisfies the editing-repo rule on its own — this session edits nothing else.
func TestAddPromotesAHeldReferenceRepoOntoTheSessionBranch(t *testing.T) {
	docs := github.Repo{Org: "org", Name: "docs", SSHURL: gittest.Origin(t, "docs"), DefaultBranch: "main"}
	p, dir, signalled := agentPicker(t, session.CreateRequest{
		Name: "reading", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{Role: session.RepoRoleReference, Repo: docs}},
	}, []github.Repo{docs})
	before, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wtBefore := filepath.Join(dir, before.Repos[0].WorktreePath)
	// Untracked, so only an in-place switch keeps it.
	gittest.WriteFile(t, wtBefore, "scratch.txt", "kept")

	got, err := p.add("reading", []repoAddition{{Name: "org/docs", Role: "editing"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantAdd(nil, []string{"org/docs"}, nil)) {
		t.Fatalf("add = %+v, want org/docs promoted and nothing composed", got)
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Repos) != 1 {
		t.Fatalf("promotion composed a second checkout: %+v", m.Repos)
	}
	promoted := repoNamed(t, m, "docs")
	if promoted.Role != session.RepoRoleEditing || promoted.Branch != "feat/reading" || promoted.Revision != "" {
		t.Fatalf("promoted repo = %+v", promoted)
	}
	// The manifest is not evidence the checkout moved; the checkout is.
	wt := filepath.Join(dir, promoted.WorktreePath)
	if got := headBranch(t, wt); got != "feat/reading" {
		t.Fatalf("promoted worktree is on %q, want feat/reading", got)
	}
	if wt != wtBefore {
		t.Fatalf("promoted worktree moved from %q to %q", wtBefore, wt)
	}
	if _, err := os.Stat(filepath.Join(wt, "scratch.txt")); err != nil {
		t.Fatalf("the in-place switch lost an untracked file: %v", err)
	}
	assertUndisturbed(t, dir, signalled)
}

func TestAddPutsAFreshRepoAndAPromotionOnTheSameBranch(t *testing.T) {
	docs := github.Repo{Org: "org", Name: "docs", SSHURL: gittest.Origin(t, "docs"), DefaultBranch: "main"}
	extra := github.Repo{Org: "org", Name: "extra", SSHURL: gittest.Origin(t, "extra"), DefaultBranch: "main"}
	p, dir, signalled := agentPicker(t, session.CreateRequest{
		Name: "reading", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{Role: session.RepoRoleReference, Repo: docs}},
	}, []github.Repo{docs, extra})

	got, err := p.add("reading", []repoAddition{
		{Name: "org/docs", Role: "editing"},
		{Name: "org/extra", Role: "editing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantAdd([]string{"org/extra"}, []string{"org/docs"}, nil)) {
		t.Fatalf("add = %+v, want extra added and docs promoted", got)
	}
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"docs", "extra"} {
		r := repoNamed(t, m, name)
		if r.Role != session.RepoRoleEditing || r.Branch != "feat/reading" {
			t.Fatalf("%s = %+v, want editing on feat/reading", name, r)
		}
		if got := headBranch(t, filepath.Join(dir, r.WorktreePath)); got != "feat/reading" {
			t.Fatalf("%s worktree is on %q", name, got)
		}
	}
	assertUndisturbed(t, dir, signalled)
}

// Take-up runs before any clone precisely so this happens: the refusal leaves the
// session exactly as it was, rather than half-composed around a repo that would
// not move.
func TestAddRefusesAPromotionOverUncommittedWorkAndComposesNothing(t *testing.T) {
	origin := gittest.Origin(t, "edited", gittest.WithFile("version", "orig"))
	edited := github.Repo{Org: "org", Name: "edited", SSHURL: origin, DefaultBranch: "main"}
	extra := github.Repo{Org: "org", Name: "extra", SSHURL: gittest.Origin(t, "extra"), DefaultBranch: "main"}
	p, dir, signalled := agentPicker(t, session.CreateRequest{
		Name: "reading", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{Role: session.RepoRoleReference, Repo: edited}},
	}, []github.Repo{edited, extra})

	// The same tracked file changed on both sides, so the switch cannot keep both.
	gittest.WriteFile(t, origin, "version", "upstream")
	gittest.Run(t, origin, "add", ".")
	gittest.Run(t, origin, "commit", "-m", "advance")
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(dir, m.Repos[0].WorktreePath)
	gittest.WriteFile(t, wt, "version", "mine")

	_, err = p.add("reading", []repoAddition{
		{Name: "org/edited", Role: "editing"},
		{Name: "org/extra", Role: "editing"},
	})
	if !errors.Is(err, session.ErrCheckoutHasWork) {
		t.Fatalf("promotion over uncommitted work = %v, want ErrCheckoutHasWork", err)
	}
	after, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Repos) != 1 {
		t.Fatalf("a refused add composed the rest of the batch anyway: %+v", after.Repos)
	}
	if r := after.Repos[0]; r.Role != session.RepoRoleReference || r.Branch != "" {
		t.Fatalf("the refused repo was recorded as taken up: %+v", r)
	}
	if body, err := os.ReadFile(filepath.Join(wt, "version")); err != nil || string(body) != "mine\n" {
		t.Fatalf("the edit was overwritten: %q (%v)", body, err)
	}
	assertUndisturbed(t, dir, signalled)
}

// The socket is the only path add_repos takes, so the request's shape and the
// three lists coming back are pinned against a real listener.
func TestOpAddReposCarriesTheAdditionsAndTheOutcomeBack(t *testing.T) {
	windows, _ := testWindows(t)
	owner := windows.shown()
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	var got []repoAddition
	var gotSlug string
	server, err := serveControl(socket, windows, owner, controlHooks{
		addRepos: func(slug string, additions []repoAddition) (addReposResult, error) {
			gotSlug, got = slug, additions
			return addReposResult{
				Added:    []string{"org/extra"},
				Promoted: []string{"org/docs"},
				Held:     []string{"org/svc"},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	host := (workbench.Handle{Socket: socket, SessionRoot: "/sessions/x"}).WindowHost()
	result, err := host.AddRepos(t.Context(), workbench.AddReposRequest{
		Repos: []workbench.RepoAddition{{Name: "org/extra"}, {Name: "org/docs", Role: "editing"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []repoAddition{{Name: "org/extra"}, {Name: "org/docs", Role: "editing"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("hook received %+v, want %+v", got, want)
	}
	// The listener's own session names the add; an agent cannot address another.
	if gotSlug != owner.slug() {
		t.Fatalf("hook was given slug %q, want the listener's own %q", gotSlug, owner.slug())
	}
	if !reflect.DeepEqual(result, workbench.AddReposResult{
		Added:    []string{"org/extra"},
		Promoted: []string{"org/docs"},
		Held:     []string{"org/svc"},
	}) {
		t.Fatalf("result = %+v", result)
	}
}

// A refusal from the picker has to reach the agent as an error, not as an add
// that quietly did nothing.
func TestOpAddReposSurfacesTheHooksRefusal(t *testing.T) {
	windows, _ := testWindows(t)
	socket, err := workbench.NewSocketPath()
	if err != nil {
		t.Fatal(err)
	}
	server, err := serveControl(socket, windows, windows.shown(), controlHooks{
		addRepos: func(string, []repoAddition) (addReposResult, error) {
			return addReposResult{}, ErrRepoNotFound
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	host := (workbench.Handle{Socket: socket, SessionRoot: "/sessions/x"}).WindowHost()
	_, err = host.AddRepos(t.Context(), workbench.AddReposRequest{
		Repos: []workbench.RepoAddition{{Name: "org/kraken"}},
	})
	if err == nil {
		t.Fatal("a refused add returned no error")
	}
	if !strings.Contains(err.Error(), ErrRepoNotFound.Error()) {
		t.Fatalf("refusal = %v", err)
	}
}

// partitionAdditions is the promotion rule itself, so it is pinned directly
// rather than only through a compose: it is pure over a manifest and a set of
// selections, and needs no origins.
func TestPartitionAdditionsSortsFreshPromotionAndHeld(t *testing.T) {
	m := session.Manifest{Repos: []session.ManifestRepo{
		{Org: "org", Name: "svc", Role: session.RepoRoleEditing},
		{Org: "Org", Name: "Docs", Role: session.RepoRoleReference},
	}}
	sel := func(org, name string, role session.RepoRole) session.RepoSelection {
		return session.RepoSelection{Repo: github.Repo{Org: org, Name: name}, Role: role}
	}
	for _, tc := range []struct {
		name     string
		in       []session.RepoSelection
		fresh    []string
		promoted []string
		held     []string
	}{
		{
			name:  "a repo the session does not hold is composed",
			in:    []session.RepoSelection{sel("org", "extra", session.RepoRoleReference)},
			fresh: []string{"org/extra"},
		},
		{
			name:     "a held reference asked for editing is taken up",
			in:       []session.RepoSelection{sel("org", "docs", session.RepoRoleEditing)},
			promoted: []string{"Org/Docs"},
		},
		{
			name: "a held editing asked for reference is never demoted",
			in:   []session.RepoSelection{sel("org", "svc", session.RepoRoleReference)},
			held: []string{"org/svc"},
		},
		{
			name: "a held repo in the role asked for is left alone",
			in:   []session.RepoSelection{sel("org", "svc", session.RepoRoleEditing)},
			held: []string{"org/svc"},
		},
		{
			// The manifest spells this repo "Org/Docs"; matching has to ignore case
			// or a held repo reaches Draft.Repos and the reply double-counts it.
			name: "casing differing from the manifest still counts as held",
			in:   []session.RepoSelection{sel("ORG", "DOCS", session.RepoRoleReference)},
			held: []string{"ORG/DOCS"},
		},
		{
			name: "the same repo named twice is acted on once",
			in: []session.RepoSelection{
				sel("org", "docs", session.RepoRoleEditing),
				sel("Org", "Docs", session.RepoRoleEditing),
			},
			promoted: []string{"Org/Docs"},
		},
		{
			name: "one fresh repo named twice is composed once",
			in: []session.RepoSelection{
				sel("org", "extra", session.RepoRoleReference),
				sel("org", "extra", session.RepoRoleReference),
			},
			fresh: []string{"org/extra"},
		},
		{
			// org/x and a bare x normalise to one key, so this is the same
			// repository twice and not two — asked for in one role, so it dedupes.
			name: "org-qualified and bare spellings of one repo dedupe",
			in: []session.RepoSelection{
				sel("org", "extra", session.RepoRoleEditing),
				sel("ORG", "Extra", session.RepoRoleEditing),
			},
			fresh: []string{"org/extra"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fresh, promote, held, err := partitionAdditions(m, tc.in)
			if err != nil {
				t.Fatalf("partitionAdditions() error = %v, want nil", err)
			}
			if got := selectionIDs(fresh); !reflect.DeepEqual(got, orEmpty(tc.fresh)) {
				t.Errorf("fresh = %v, want %v", got, orEmpty(tc.fresh))
			}
			if got := refIDs(promote); !reflect.DeepEqual(got, orEmpty(tc.promoted)) {
				t.Errorf("promoted = %v, want %v", got, orEmpty(tc.promoted))
			}
			if !reflect.DeepEqual(held, orEmpty(tc.held)) {
				t.Errorf("held = %v, want %v", held, orEmpty(tc.held))
			}
		})
	}
}

// Refusing rather than resolving is the whole reason the roles asked for and the
// names already handled share one map, so it is pinned on the function directly
// and not only through a compose.
func TestPartitionAdditionsRefusesOneRepoNamedInTwoRoles(t *testing.T) {
	m := session.Manifest{Repos: []session.ManifestRepo{
		{Org: "org", Name: "docs", Role: session.RepoRoleReference},
	}}
	sel := func(org, name string, role session.RepoRole) session.RepoSelection {
		return session.RepoSelection{Repo: github.Repo{Org: org, Name: name}, Role: role}
	}
	for _, tc := range []struct {
		name string
		in   []session.RepoSelection
	}{
		{
			name: "a fresh repo in two roles",
			in: []session.RepoSelection{
				sel("org", "extra", session.RepoRoleReference),
				sel("org", "extra", session.RepoRoleEditing),
			},
		},
		{
			name: "the same conflict in the other order",
			in: []session.RepoSelection{
				sel("org", "extra", session.RepoRoleEditing),
				sel("org", "extra", session.RepoRoleReference),
			},
		},
		{
			name: "a held repo in two roles",
			in: []session.RepoSelection{
				sel("org", "docs", session.RepoRoleEditing),
				sel("org", "docs", session.RepoRoleReference),
			},
		},
		{
			// Differing spellings of one repository still collide, so a conflict
			// cannot be smuggled past the refusal by re-spelling the name.
			name: "a conflict across org-qualified and bare spellings",
			in: []session.RepoSelection{
				sel("org", "extra", session.RepoRoleEditing),
				sel("ORG", "Extra", session.RepoRoleReference),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fresh, promote, held, err := partitionAdditions(m, tc.in)
			if !errors.Is(err, ErrRepoRoleConflict) {
				t.Fatalf("partitionAdditions() error = %v, want ErrRepoRoleConflict", err)
			}
			if fresh != nil || promote != nil || held != nil {
				t.Fatalf("a refusal still partitioned: fresh=%v promote=%v held=%v", fresh, promote, held)
			}
		})
	}
}

// Keeping either role would hand the agent one it did not ask for; an unwanted
// detached checkout it then commits into strands the work.
func TestAddRefusesOneRepoNamedTwiceInTwoRoles(t *testing.T) {
	extra := github.Repo{Org: "org", Name: "extra", SSHURL: gittest.Origin(t, "extra"), DefaultBranch: "main"}
	svc := github.Repo{Org: "org", Name: "svc", SSHURL: gittest.Origin(t, "svc"), DefaultBranch: "main"}
	for _, order := range [][]repoAddition{
		{{Name: "org/extra"}, {Name: "extra", Role: "editing"}},
		{{Name: "extra", Role: "editing"}, {Name: "org/extra"}},
	} {
		p, dir, signalled := agentPicker(t, session.CreateRequest{
			Name: "adding", Prefix: "feat", Mode: session.ModeAssistant,
			Repos: []session.RepoSelection{{Role: session.RepoRoleEditing, Repo: svc}},
		}, []github.Repo{svc, extra})

		_, err := p.add("adding", order)
		if !errors.Is(err, ErrRepoRoleConflict) {
			t.Fatalf("add(%+v) = %v, want ErrRepoRoleConflict", order, err)
		}
		m, err := session.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Repos) != 1 {
			t.Fatalf("a refused add composed something: %+v", m.Repos)
		}
		assertUndisturbed(t, dir, signalled)
	}
}

// Re-adding what the session already holds is the natural retry after a partial
// failure, so it answers rather than erroring — and writes nothing.
func TestAddOfOnlyHeldReposIsANoOpEvenWithNoEditingRepo(t *testing.T) {
	docs := github.Repo{Org: "org", Name: "docs", SSHURL: gittest.Origin(t, "docs"), DefaultBranch: "main"}
	p, dir, signalled := agentPicker(t, session.CreateRequest{
		Name: "reading", Prefix: "feat", Mode: session.ModeAssistant,
		Repos: []session.RepoSelection{{Role: session.RepoRoleReference, Repo: docs}},
	}, []github.Repo{docs})
	before, err := os.ReadFile(filepath.Join(dir, "qrouton.json"))
	if err != nil {
		t.Fatal(err)
	}

	got, err := p.add("reading", []repoAddition{{Name: "org/docs"}})
	if err != nil {
		t.Fatalf("re-adding a held repo = %v, want the held report", err)
	}
	if !reflect.DeepEqual(got, wantAdd(nil, nil, []string{"org/docs"})) {
		t.Fatalf("add = %+v, want org/docs held", got)
	}
	after, err := os.ReadFile(filepath.Join(dir, "qrouton.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("a no-op add rewrote the manifest")
	}
	assertUndisturbed(t, dir, signalled)
}
