package desktop

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
)

// fakeGitHub records which owners a refresh asked for and answers each with one
// repository, so the rules are exercised without a token or a network.
type fakeGitHub struct {
	mu     sync.Mutex
	all    []string
	one    []string
	cached [][]github.Repo
	fails  map[string]error
	token  error
}

func newFakeGitHub() *fakeGitHub { return &fakeGitHub{fails: map[string]error{}} }

func (f *fakeGitHub) calls() gh {
	return gh{
		token: func() (string, error) { return "t", f.token },
		all: func(_ context.Context, _ string, orgs []string, _ []github.Repo) <-chan github.RefreshMsg {
			f.mu.Lock()
			f.all = append(f.all, orgs...)
			f.mu.Unlock()
			ch := make(chan github.RefreshMsg, 2*len(orgs)+1)
			var merged []github.Repo
			for _, owner := range orgs {
				ch <- github.RefreshMsg{Owner: owner, State: github.RefreshStarted}
				if err := f.fails[owner]; err != nil {
					ch <- github.RefreshMsg{Owner: owner, State: github.RefreshFailed, Err: err}
					continue
				}
				merged = append(merged, github.Repo{Org: owner, Name: "repo"})
				ch <- github.RefreshMsg{Owner: owner, State: github.RefreshSucceeded, Repos: merged}
			}
			ch <- github.RefreshMsg{State: github.RefreshComplete, Repos: merged}
			close(ch)
			return ch
		},
		one: func(_ context.Context, _, owner string) ([]github.Repo, error) {
			f.mu.Lock()
			f.one = append(f.one, owner)
			f.mu.Unlock()
			if err := f.fails[owner]; err != nil {
				return nil, err
			}
			return []github.Repo{{Org: owner, Name: "repo"}}, nil
		},
		cache: func(_ []string, repos []github.Repo) {
			f.mu.Lock()
			f.cached = append(f.cached, repos)
			f.mu.Unlock()
		},
	}
}

func (f *fakeGitHub) asked() ([]string, []string, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.all...), append([]string(nil), f.one...), len(f.cached)
}

// testRepositories is a repository list that never reads the user's own cache.
func testRepositories(t *testing.T, orgs []string, fake *fakeGitHub) (*Repositories, chan int, *[]refreshEvent) {
	t.Helper()
	var mu sync.Mutex
	var events []refreshEvent
	finished := make(chan int, 8)
	r := &Repositories{
		cfg: &config.Config{Orgs: orgs},
		emit: func(event string, payload any) {
			if event != reposRefreshEvent {
				return
			}
			mu.Lock()
			events = append(events, payload.(refreshEvent))
			mu.Unlock()
		},
		gh:   fake.calls(),
		errs: map[string]error{},
		done: func(generation int) { finished <- generation },
	}
	return r, finished, &events
}

func TestRefreshFetchesEveryOwnerAndStampsItsGenerationOnEveryEvent(t *testing.T) {
	fake := newFakeGitHub()
	repos, finished, events := testRepositories(t, []string{"acme", "other"}, fake)

	gen := repos.Refresh()
	if gen != 1 {
		t.Fatalf("first refresh is generation %d", gen)
	}
	<-finished

	all, one, _ := fake.asked()
	if len(all) != 2 || len(one) != 0 {
		t.Fatalf("a clean refresh asked all=%v one=%v", all, one)
	}
	if len(*events) == 0 {
		t.Fatal("a refresh emitted nothing")
	}
	for _, event := range *events {
		if event.Generation != gen {
			t.Fatalf("event %+v does not carry generation %d", event, gen)
		}
	}
	if got := repos.Cached(); len(got) != 2 {
		t.Fatalf("the refreshed list is %+v", got)
	}
}

// One button serves refresh and retry both: with an owner still failing, only
// that owner is refetched.
func TestARefreshAfterAFailureRefetchesOnlyTheFailedOwner(t *testing.T) {
	fake := newFakeGitHub()
	fake.fails["other"] = errors.New("unavailable")
	repos, finished, _ := testRepositories(t, []string{"acme", "other"}, fake)

	repos.Refresh()
	<-finished
	if repos.clean() {
		t.Fatal("a failed owner left no error behind")
	}

	fake.fails = map[string]error{}
	repos.Refresh()
	<-finished

	all, one, cached := fake.asked()
	if len(all) != 2 {
		t.Fatalf("the retry fanned out over every owner again: %v", all)
	}
	if len(one) != 1 || one[0] != "other" {
		t.Fatalf("the retry refetched %v, want the failed owner alone", one)
	}
	if cached != 1 {
		t.Fatalf("the cache was written %d times, want once — after every owner came back clean", cached)
	}
	if !repos.clean() {
		t.Fatal("a clean retry left the owner's error behind")
	}
}

func TestARetryThatStillFailsWritesNoCache(t *testing.T) {
	fake := newFakeGitHub()
	fake.fails["other"] = errors.New("unavailable")
	repos, finished, _ := testRepositories(t, []string{"acme", "other"}, fake)

	repos.Refresh()
	<-finished
	repos.Refresh()
	<-finished

	if _, _, cached := fake.asked(); cached != 0 {
		t.Fatalf("the cache was written %d times while an owner was still failing", cached)
	}
}

// Emit broadcasts process-wide, so a superseded refresh's events are still in
// flight. They are dropped rather than applied.
func TestASupersededRefreshsEventsAreDropped(t *testing.T) {
	fake := newFakeGitHub()
	repos, _, events := testRepositories(t, []string{"acme"}, fake)
	repos.gen = 7

	if repos.push(6, github.RefreshMsg{Owner: "acme", State: github.RefreshSucceeded,
		Repos: []github.Repo{{Org: "stale", Name: "repo"}}}) {
		t.Fatal("a stale generation was applied")
	}
	if got := repos.Cached(); len(got) != 0 {
		t.Fatalf("a stale refresh replaced the list: %+v", got)
	}
	if len(*events) != 0 {
		t.Fatalf("a stale refresh reached the page: %+v", *events)
	}
	if !repos.push(7, github.RefreshMsg{Owner: "acme", State: github.RefreshSucceeded,
		Repos: []github.Repo{{Org: "acme", Name: "repo"}}}) {
		t.Fatal("the live generation was dropped")
	}
	if got := repos.Cached(); len(got) != 1 {
		t.Fatalf("the live refresh did not land: %+v", got)
	}
}

// A refresh in flight is abandoned rather than raced: the second one cancels the
// first's context, and the first's remaining events are dropped by generation.
func TestARefreshCancelsThePriorOne(t *testing.T) {
	fake := newFakeGitHub()
	repos, _, _ := testRepositories(t, []string{"acme"}, fake)
	contexts := make(chan context.Context, 2)
	live := repos.gh
	repos.gh.all = func(ctx context.Context, _ string, _ []string, _ []github.Repo) <-chan github.RefreshMsg {
		contexts <- ctx
		return live.all(ctx, "t", []string{"acme"}, nil)
	}

	first := repos.Refresh()
	firstCtx := <-contexts
	second := repos.Refresh()
	<-contexts
	if second <= first {
		t.Fatalf("a second refresh reused generation %d", second)
	}
	select {
	case <-firstCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("the superseded refresh's context is still live")
	}
}

func TestATokenFailureFailsEveryOwnerAndCompletes(t *testing.T) {
	fake := newFakeGitHub()
	fake.token = errors.New("no token")
	repos, finished, events := testRepositories(t, []string{"acme", "other"}, fake)

	repos.Refresh()
	<-finished

	failed := map[string]bool{}
	complete := false
	for _, event := range *events {
		switch event.State {
		case string(github.RefreshFailed):
			failed[event.Owner] = true
			if event.Error != "no token" {
				t.Fatalf("failure event carries %q, not the error's text", event.Error)
			}
		case string(github.RefreshComplete):
			complete = true
		}
	}
	if len(failed) != 2 || !complete {
		t.Fatalf("a token failure reported failed=%v complete=%v", failed, complete)
	}
}

func TestOrgsListAnswersAnEmptyConfigurationWithAnEmptyList(t *testing.T) {
	if got := (&Orgs{cfg: &config.Config{}}).List(); len(got) != 0 {
		t.Fatalf("an empty configuration listed %v", got)
	}
	if got := (&Orgs{cfg: &config.Config{Orgs: []string{"acme"}}}).List(); len(got) != 1 || got[0] != "acme" {
		t.Fatalf("orgs = %v", got)
	}
}

func TestARetryDropsReposOfAnOwnerThatLeftTheConfiguration(t *testing.T) {
	fake := newFakeGitHub()
	fake.fails["acme"] = errors.New("unavailable")
	repos, finished, _ := testRepositories(t, []string{"acme"}, fake)

	repos.Refresh()
	<-finished
	repos.mu.Lock()
	repos.repos = append(repos.repos, github.Repo{Org: "gone", Name: "orphan"})
	repos.mu.Unlock()

	fake.fails = map[string]error{}
	repos.Refresh()
	<-finished

	for _, repo := range repos.Cached() {
		if repo.Org == "gone" {
			t.Fatalf("the list kept %s, whose owner is no longer configured", repo.ID())
		}
	}
}
