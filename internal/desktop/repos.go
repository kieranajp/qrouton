package desktop

import (
	"context"
	"net/http"
	"sync"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
)

// gh is the GitHub work a refresh does, as fields so a test drives the rules
// without a token or a network.
type gh struct {
	token func() (string, error)
	all   func(ctx context.Context, token string, orgs []string, cached []github.Repo) <-chan github.RefreshMsg
	one   func(ctx context.Context, token, owner string) ([]github.Repo, error)
	cache func(orgs []string, repos []github.Repo)
}

func liveGitHub() gh {
	return gh{
		token: github.Token,
		all: func(ctx context.Context, token string, orgs []string, cached []github.Repo) <-chan github.RefreshMsg {
			return github.RefreshRepos(ctx, http.DefaultClient, token, orgs, cached)
		},
		one: func(ctx context.Context, token, owner string) ([]github.Repo, error) {
			return github.RefreshOwnerRepos(ctx, http.DefaultClient, token, owner)
		},
		cache: github.WriteRepoCache,
	}
}

// Repositories is the list the second step chooses from: the cache it opens on,
// and the refresh its button drives.
type Repositories struct {
	cfg  *config.Config
	emit emitter
	gh   gh

	mu    sync.Mutex
	repos []github.Repo
	// errs is the last attempt's outcome per owner, which is what makes one
	// button serve refresh and retry both.
	errs   map[string]error
	gen    int
	cancel context.CancelFunc
	// done reports a run's end, so a test can wait on one.
	done func(generation int)
}

func newRepositories(cfg *config.Config, emit emitter) *Repositories {
	repos, _, _ := github.CachedRepos(cfg.Orgs)
	return &Repositories{cfg: cfg, emit: emit, gh: liveGitHub(), repos: repos, errs: map[string]error{}}
}

// Cached answers from the cache without touching the network, so the step draws
// a list before any owner has been fetched.
func (r *Repositories) Cached() []github.Repo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]github.Repo(nil), r.repos...)
}

// Select resolves picked rows against the list the step was drawn from, in the
// order they were picked. A repository a refresh has dropped simply is not there
// any more, which is what leaves a draft short of an editing repo for Check to
// refuse.
func (r *Repositories) Select(picks []repoPick) []session.RepoSelection {
	byID := make(map[string]github.Repo)
	for _, repo := range r.Cached() {
		byID[repo.ID()] = repo
	}
	out := make([]session.RepoSelection, 0, len(picks))
	for _, pick := range picks {
		repo, ok := byID[pick.ID]
		if !ok {
			continue
		}
		out = append(out, session.RepoSelection{Repo: repo, Role: session.RepoRole(pick.Role)})
	}
	return out
}

// Refresh refetches the owners whose last attempt failed if any did, and every
// owner otherwise, so the user never has to know which of two kinds of refresh
// he wants. It answers with the generation its events will carry.
func (r *Repositories) Refresh() int {
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.gen++
	gen := r.gen
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	var failed []string
	for _, owner := range r.cfg.Orgs {
		if r.errs[owner] != nil {
			failed = append(failed, owner)
		}
	}
	cached := append([]github.Repo(nil), r.repos...)
	r.mu.Unlock()

	go r.run(ctx, gen, failed, cached)
	return gen
}

// run fans out over every owner, or walks the failed ones alone. The retry half
// writes the cache itself, and only once every owner is clean — a cache holding
// one owner's stale list beside another's fresh one is worse than no write.
func (r *Repositories) run(ctx context.Context, gen int, failed []string, cached []github.Repo) {
	if r.done != nil {
		defer r.done(gen)
	}
	token, err := r.gh.token()
	if err != nil {
		owners := failed
		if len(owners) == 0 {
			owners = r.cfg.Orgs
		}
		for _, owner := range owners {
			r.push(gen, github.RefreshMsg{Owner: owner, State: github.RefreshFailed, Err: err})
		}
		r.push(gen, github.RefreshMsg{State: github.RefreshComplete})
		return
	}
	if len(failed) == 0 {
		for msg := range r.gh.all(ctx, token, r.cfg.Orgs, cached) {
			if !r.push(gen, msg) {
				return
			}
		}
		return
	}
	merged := cached
	for _, owner := range failed {
		if !r.push(gen, github.RefreshMsg{Owner: owner, State: github.RefreshStarted}) {
			return
		}
		repos, err := r.gh.one(ctx, token, owner)
		if err != nil {
			if !r.push(gen, github.RefreshMsg{Owner: owner, State: github.RefreshFailed, Err: err}) {
				return
			}
			continue
		}
		merged = github.ReplaceOwnerRepos(merged, owner, repos)
		if !r.push(gen, github.RefreshMsg{Owner: owner, State: github.RefreshSucceeded}) {
			return
		}
	}
	github.SortReposByActivity(merged)
	if r.clean() && ctx.Err() == nil {
		r.gh.cache(r.cfg.Orgs, merged)
	}
	r.push(gen, github.RefreshMsg{State: github.RefreshComplete, Repos: merged})
}

// push folds one message into the list and the per-owner errors, then emits it.
// A message from a superseded generation is dropped rather than applied, and the
// false answer stops the run it came from.
func (r *Repositories) push(gen int, msg github.RefreshMsg) bool {
	r.mu.Lock()
	if gen != r.gen {
		r.mu.Unlock()
		return false
	}
	switch msg.State {
	case github.RefreshSucceeded:
		delete(r.errs, msg.Owner)
	case github.RefreshFailed:
		r.errs[msg.Owner] = msg.Err
	}
	if msg.Repos != nil {
		r.repos = msg.Repos
	}
	r.mu.Unlock()
	r.emit(reposRefreshEvent, newRefreshEvent(gen, msg))
	return true
}

func (r *Repositories) clean() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.errs) == 0
}

// Orgs is the owner filter's vocabulary. An empty configuration answers with an
// empty list: nothing here asks for owners, and the step cannot be passed
// without an editing repository anyway.
type Orgs struct{ cfg *config.Config }

func (o *Orgs) List() []string {
	return append([]string(nil), o.cfg.Orgs...)
}
