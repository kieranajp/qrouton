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
	token    func() (string, error)
	all      func(ctx context.Context, token string, orgs []string, cached []github.Repo) <-chan github.RefreshMsg
	one      func(ctx context.Context, token, owner string) ([]github.Repo, error)
	branches func(ctx context.Context, token, owner, name string) ([]string, error)
	cache    func(orgs []string, repos []github.Repo) error
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
		branches: func(ctx context.Context, token, owner, name string) ([]string, error) {
			ctx, cancel := context.WithTimeout(ctx, branchListTimeout)
			defer cancel()
			return github.ListBranches(ctx, http.DefaultClient, token, owner, name)
		},
		cache: github.WriteRepoCache,
	}
}

type Repositories struct {
	cfg  *config.Config
	emit emitter
	gh   gh

	mu    sync.Mutex
	repos []github.Repo
	// errs is the last attempt's outcome per owner, which is what makes one
	// button serve refresh and retry both.
	errs map[string]error
	// branchLists is this workbench's memo of what a repository's branches are,
	// held only in memory: the on-disk repository cache is owner-keyed and
	// version-gated, so a branch list has no business in it.
	branchLists map[string]branchList
	gen         int
	cancel      context.CancelFunc
	// done reports a run's end, so a test can wait on one.
	done func(generation int)
}

func newRepositories(cfg *config.Config, emit emitter) *Repositories {
	repos, _, _ := github.CachedRepos(cfg.Snapshot().Orgs)
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
// any more. A base naming the repository's own default branch is cleared here,
// so a session nobody chose a branch for records none.
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
		base := pick.Base
		if base == repo.DefaultBranch {
			base = ""
		}
		out = append(out, session.RepoSelection{Repo: repo, Role: session.RepoRole(pick.Role), Base: base})
	}
	return out
}

// Branches names what a repository's work can be cut from, the default branch
// first. A failure arrives inside the payload rather than as a Go error, and is
// not memoised, so an unreachable GitHub leaves the menu the default branch to
// offer and the next open tries again.
func (r *Repositories) Branches(id string) branchList {
	repo, ok := r.byID(id)
	if !ok {
		return branchList{}
	}
	if memo, ok := r.memoisedBranches(id); ok {
		return memo
	}
	list, ok := r.fetchBranches(repo)
	if ok {
		r.memoiseBranches(id, list)
	}
	return list
}

func (r *Repositories) fetchBranches(repo github.Repo) (branchList, bool) {
	fallback := branchList{Default: repo.DefaultBranch, Branches: defaultFirst(nil, repo.DefaultBranch)}
	token, err := r.gh.token()
	if err != nil {
		fallback.Error = errorText(err)
		return fallback, false
	}
	names, err := r.gh.branches(context.Background(), token, repo.Org, repo.Name)
	if err != nil {
		fallback.Error = errorText(err)
		return fallback, false
	}
	return branchList{Default: repo.DefaultBranch, Branches: defaultFirst(names, repo.DefaultBranch)}, true
}

// defaultFirst leads with the branch a session would take anyway.
func defaultFirst(names []string, defaultBranch string) []string {
	out := make([]string, 0, len(names)+1)
	if defaultBranch != "" {
		out = append(out, defaultBranch)
	}
	for _, name := range names {
		if name != defaultBranch {
			out = append(out, name)
		}
	}
	return out
}

func (r *Repositories) byID(id string) (github.Repo, bool) {
	for _, repo := range r.Cached() {
		if repo.ID() == id {
			return repo, true
		}
	}
	return github.Repo{}, false
}

func (r *Repositories) memoisedBranches(id string) (branchList, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	list, ok := r.branchLists[id]
	return list, ok
}

func (r *Repositories) memoiseBranches(id string, list branchList) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.branchLists == nil {
		r.branchLists = map[string]branchList{}
	}
	r.branchLists[id] = list
}

// Refresh refetches the owners whose last attempt failed if any did, and every
// owner otherwise, so the user never has to know which of two kinds of refresh
// he wants. It answers with the generation its events will carry.
func (r *Repositories) Refresh() int {
	owners := r.cfg.Snapshot().Orgs
	r.mu.Lock()
	if r.cancel != nil {
		r.cancel()
	}
	r.gen++
	gen := r.gen
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	var failed []string
	for _, owner := range owners {
		if r.errs[owner] != nil {
			failed = append(failed, owner)
		}
	}
	cached := github.OwnedBy(r.repos, owners)
	r.mu.Unlock()

	go r.run(ctx, gen, owners, failed, cached)
	return gen
}

// run fans out over every owner, or walks the failed ones alone. The retry half
// writes the cache itself, and only once every owner is clean: the write stamps
// every configured owner as fetched now, so writing with one still failing would
// date its stale list to this moment.
func (r *Repositories) run(ctx context.Context, gen int, configured []string, failed []string, cached []github.Repo) {
	if r.done != nil {
		defer r.done(gen)
	}
	token, err := r.gh.token()
	if err != nil {
		owners := failed
		if len(owners) == 0 {
			owners = configured
		}
		for _, owner := range owners {
			r.push(gen, github.RefreshMsg{Owner: owner, State: github.RefreshFailed, Err: err})
		}
		r.push(gen, github.RefreshMsg{State: github.RefreshComplete})
		return
	}
	if len(failed) == 0 {
		for msg := range r.gh.all(ctx, token, configured, cached) {
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
		_ = r.gh.cache(configured, merged)
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
	return o.cfg.Snapshot().Orgs
}
