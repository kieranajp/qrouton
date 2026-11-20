package core

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kieranajp/qrouton/internal/atomicfile"
	"github.com/kieranajp/qrouton/internal/github"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/status"
)

// DashboardStatus is what the dashboard draws: every session's pull requests and
// git stats, and how fresh they are. A zero time means never.
type DashboardStatus struct {
	Viewer      string                      `json:"viewer"`
	CheckedAt   time.Time                   `json:"checkedAt"`
	Refreshing  bool                        `json:"refreshing"`
	Failed      bool                        `json:"failed"`
	SignedOut   bool                        `json:"signedOut"`
	PausedUntil time.Time                   `json:"pausedUntil"`
	Sessions    map[string]DashboardSession `json:"sessions"`
}

type DashboardSession struct {
	PullRequests []status.PullRequest `json:"pullRequests"`
	PullState    string               `json:"pullState"`
	Repos        []status.RepoStat    `json:"repos"`
}

// dashboardAPI is the outside work a refresh does, as fields so a test drives
// the rules without a token, a network or git.
type dashboardAPI struct {
	credential func(ctx context.Context) (github.Credential, string)
	pulls      func(ctx context.Context, token string, targets []github.BranchTarget) (string, []github.BranchResult, error)
	measure    func(ctx context.Context, root string) []status.RepoStat
}

func liveDashboardAPI() dashboardAPI {
	return dashboardAPI{
		credential: func(ctx context.Context) (github.Credential, string) {
			ctx, cancel := context.WithTimeout(ctx, dashboardTimeout)
			defer cancel()
			return github.ResolveCredential(ctx)
		},
		pulls: func(ctx context.Context, token string, targets []github.BranchTarget) (string, []github.BranchResult, error) {
			ctx, cancel := context.WithTimeout(ctx, dashboardTimeout)
			defer cancel()
			return github.BranchPullRequests(ctx, http.DefaultClient, token, targets)
		},
		measure: status.Repos,
	}
}

// pullsCache is pulls.json: rows keyed by session slug, then repository, then branch.
type pullsCache struct {
	SchemaVersion int                                                   `json:"schemaVersion"`
	Viewer        string                                                `json:"viewer"`
	CheckedAt     time.Time                                             `json:"checkedAt"`
	Sessions      map[string]map[string]map[string][]status.PullRequest `json:"sessions"`
}

// Dashboard owns every session's pull requests, refreshed in one GitHub request,
// and the git stats the cards show. Triggers never block: they leave a wake-up
// the loop takes once the refresh in flight is done.
type Dashboard struct {
	root      string
	showing   func() bool
	emit      Emitter
	api       dashboardAPI
	now       func() time.Time
	cachePath string

	every, shownEvery time.Duration

	wake        chan struct{}
	measureNext atomic.Bool

	mu          sync.Mutex
	viewer      string
	checkedAt   time.Time
	refreshing  bool
	failed      bool
	signedOut   bool
	pausedUntil time.Time
	pulls       map[string]map[string]map[string][]status.PullRequest
	stats       map[string][]status.RepoStat
	// unanswered holds each session's branches whose alias failed in the last per-alias answer, by repo key.
	unanswered map[string]map[string]bool
	// logged is the last failure logged per alias, with the whole refresh under "".
	logged map[string]string
	cancel context.CancelFunc

	// publishMu orders emits, so an older status never lands after a newer one.
	publishMu sync.Mutex
	last      DashboardStatus
	published bool

	runs sync.WaitGroup
}

func newDashboard(root string, showing func() bool, emit Emitter, api dashboardAPI, now func() time.Time, cachePath string) *Dashboard {
	d := &Dashboard{
		root: root, showing: showing, emit: emit, api: api, now: now, cachePath: cachePath,
		every: dashboardInterval, shownEvery: dashboardShownInterval,
		wake:  make(chan struct{}, 1),
		pulls: map[string]map[string]map[string][]status.PullRequest{},
		stats: map[string][]status.RepoStat{}, logged: map[string]string{},
	}
	if cached, ok := readPullsCache(cachePath); ok {
		d.viewer, d.checkedAt = cached.Viewer, cached.CheckedAt
		if cached.Sessions != nil {
			d.pulls = cached.Sessions
		}
	}
	return d
}

func readPullsCache(path string) (pullsCache, bool) {
	var c pullsCache
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &c) != nil || c.SchemaVersion != pullsCacheSchemaVersion {
		return pullsCache{}, false
	}
	return c, true
}

func (d *Dashboard) Snapshot() DashboardStatus { return d.status(d.scan()) }

// Request refreshes pull requests without measuring git.
func (d *Dashboard) Request() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *Dashboard) Refresh() {
	d.measureNext.Store(true)
	d.Request()
}

func (d *Dashboard) start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	d.mu.Lock()
	d.cancel = cancel
	d.mu.Unlock()
	d.runs.Go(func() { d.loop(ctx) })
}

// stop returns only once no refresh or measurement can still write.
func (d *Dashboard) stop() {
	d.mu.Lock()
	if d.cancel != nil {
		d.cancel()
	}
	d.mu.Unlock()
	d.runs.Wait()
}

func (d *Dashboard) loop(ctx context.Context) {
	full := time.NewTicker(d.every)
	defer full.Stop()
	shown := time.NewTicker(d.shownEvery)
	defer shown.Stop()
	refreshed := make(chan struct{}, 1)
	measured := make(chan struct{}, 1)
	refreshing, measuring, measureAgain := false, false, false
	run := func(stats bool) {
		if stats && measuring {
			measureAgain = true
		} else if stats {
			measuring = true
			d.runs.Go(func() { d.measure(ctx); measured <- struct{}{} })
		}
		refreshing = true
		d.runs.Go(func() { d.refresh(ctx); refreshed <- struct{}{} })
	}

	run(true)
	for {
		// Unread while a refresh runs, a trigger waits in its channel and runs once after it.
		wake, tick, poll := d.wake, full.C, shown.C
		if refreshing {
			wake, tick, poll = nil, nil, nil
		}
		select {
		case <-ctx.Done():
			return
		case <-refreshed:
			refreshing = false
		case <-measured:
			measuring = false
			if measureAgain {
				measureAgain = false
				measuring = true
				d.runs.Go(func() { d.measure(ctx); measured <- struct{}{} })
			}
		case <-tick:
			shown.Reset(d.shownEvery)
			d.measureNext.Store(false)
			run(true)
		case <-poll:
			if d.showing() {
				run(false)
			}
		case <-wake:
			run(d.measureNext.Swap(false))
		}
	}
}

type dashboardSession struct {
	slug, root string
	repos      []dashboardRepo
}

// dashboardRepo is an editing repository with a branch, the only kind with pull requests of its own.
type dashboardRepo struct {
	id     string
	target github.BranchTarget
}

func (r dashboardRepo) key() string {
	return strings.ToLower(r.target.Owner) + "/" + strings.ToLower(r.target.Name) + "\x00" + r.target.Branch
}

func dashboardRepos(m session.Manifest) []dashboardRepo {
	var out []dashboardRepo
	for _, repo := range m.Repos {
		if !repo.Role.IsEditing() || repo.Branch == "" {
			continue
		}
		out = append(out, dashboardRepo{
			id:     pullRequestRepoID(repo),
			target: github.BranchTarget{Owner: repo.Org, Name: repo.Name, Branch: repo.Branch},
		})
	}
	return out
}

func (d *Dashboard) scan() []dashboardSession {
	manifests, _ := session.Scan(d.root)
	out := make([]dashboardSession, 0, len(manifests))
	for _, m := range manifests {
		out = append(out, dashboardSession{slug: m.Slug, root: filepath.Join(d.root, m.Slug), repos: dashboardRepos(m)})
	}
	return out
}

// rows is the session at root's pull requests, in its manifest's repository
// order, so a push that changes nothing compares equal.
func (d *Dashboard) rows(root string) status.PullRequestList {
	m, err := session.Load(root)
	d.mu.Lock()
	defer d.mu.Unlock()
	out := status.PullRequestList{Items: []status.PullRequest{}}
	if err != nil {
		return out
	}
	repos := dashboardRepos(m)
	out.SignedOut = d.signedOut && len(repos) > 0
	out.Items = d.rowsLocked(m.Slug, repos)
	return out
}

func (d *Dashboard) rowsLocked(slug string, repos []dashboardRepo) []status.PullRequest {
	out := []status.PullRequest{}
	for _, repo := range repos {
		out = append(out, d.pulls[slug][repo.id][repo.target.Branch]...)
	}
	return out
}

func (d *Dashboard) status(sessions []dashboardSession) DashboardStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := DashboardStatus{
		Viewer: d.viewer, CheckedAt: d.checkedAt, Refreshing: d.refreshing, Failed: d.failed,
		SignedOut: d.signedOut, PausedUntil: d.pausedUntil, Sessions: make(map[string]DashboardSession, len(sessions)),
	}
	for _, s := range sessions {
		repos := d.stats[s.slug]
		if repos == nil {
			repos = []status.RepoStat{}
		}
		out.Sessions[s.slug] = DashboardSession{PullRequests: d.rowsLocked(s.slug, s.repos), PullState: d.pullStateLocked(s), Repos: repos}
	}
	return out
}

func (d *Dashboard) pullStateLocked(s dashboardSession) string {
	if len(s.repos) == 0 {
		return dashboardPullsNone
	}
	state := dashboardPullsAnswered
	for _, repo := range s.repos {
		if d.unanswered[s.slug][repo.key()] {
			return dashboardPullsUnavailable
		}
		if _, ok := d.pulls[s.slug][repo.id][repo.target.Branch]; !ok {
			state = dashboardPullsPending
		}
	}
	return state
}

func (d *Dashboard) publish() {
	d.publishMu.Lock()
	defer d.publishMu.Unlock()
	next := d.status(d.scan())
	if d.published && reflect.DeepEqual(next, d.last) {
		return
	}
	d.last, d.published = next, true
	if d.emit != nil {
		d.emit(dashboardEvent, next)
	}
}

// refresh asks GitHub for every session's pull requests at once. A branch
// that cannot be answered keeps its rows unless GitHub names another viewer.
func (d *Dashboard) refresh(ctx context.Context) {
	sessions := d.scan()
	var targets []github.BranchTarget
	index := map[string]int{}
	for _, s := range sessions {
		for _, repo := range s.repos {
			if _, seen := index[repo.key()]; !seen {
				index[repo.key()] = len(targets)
				targets = append(targets, repo.target)
			}
		}
	}
	if len(targets) == 0 {
		d.publish()
		return
	}
	d.mu.Lock()
	if d.now().Before(d.pausedUntil) {
		d.mu.Unlock()
		return
	}
	d.refreshing = true
	d.mu.Unlock()
	d.publish()

	cache, ok := d.fetch(ctx, sessions, targets, index)
	if ctx.Err() != nil {
		return
	}
	if ok {
		if err := writePullsCache(d.cachePath, cache); err != nil {
			d.logOnce(pullsCacheLogKey, err)
		} else {
			d.mu.Lock()
			delete(d.logged, pullsCacheLogKey)
			d.mu.Unlock()
		}
	}
	d.mu.Lock()
	d.refreshing = false
	d.mu.Unlock()
	d.publish()
}

// fetch updates the rows in memory and answers what pulls.json should hold, or
// false when pulls.json keeps what it has. Rows outlive a failed alias only for
// the viewer who fetched them.
func (d *Dashboard) fetch(ctx context.Context, sessions []dashboardSession, targets []github.BranchTarget, index map[string]int) (pullsCache, bool) {
	_, token := d.api.credential(ctx)
	if ctx.Err() != nil {
		return pullsCache{}, false
	}
	if token == "" {
		d.mu.Lock()
		d.signedOut = true
		d.mu.Unlock()
		return pullsCache{}, false
	}
	viewer, results, err := d.api.pulls(ctx, token, targets)
	if ctx.Err() != nil {
		return pullsCache{}, false
	}

	var limited *github.RateLimitError
	if errors.As(err, &limited) {
		d.mu.Lock()
		d.pausedUntil = limited.Until
		d.mu.Unlock()
	}
	switch {
	case errors.Is(err, github.ErrTokenRejected):
		d.mu.Lock()
		d.signedOut = true
		d.mu.Unlock()
		return pullsCache{}, false
	case limited != nil && results == nil:
		d.logOnce("", err)
		return pullsCache{}, false
	case err != nil && limited == nil:
		d.mu.Lock()
		d.failed = true
		d.mu.Unlock()
		d.logOnce("", err)
		return pullsCache{}, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	reuse := viewer == "" || d.viewer == "" || viewer == d.viewer
	next := map[string]map[string]map[string][]status.PullRequest{}
	unanswered := map[string]map[string]bool{}
	answered := false
	for _, s := range sessions {
		for _, repo := range s.repos {
			result := results[index[repo.key()]]
			var rows []status.PullRequest
			switch {
			case result.Err == nil:
				rows, answered = pullRequestRows(repo.id, result.PullRequests), true
				delete(d.logged, repo.key())
			case errors.Is(result.Err, github.ErrNotFound):
				rows, answered = []status.PullRequest{}, true
				delete(d.logged, repo.key())
			default:
				d.logOnceLocked(repo.key(), result.Err)
				if unanswered[s.slug] == nil {
					unanswered[s.slug] = map[string]bool{}
				}
				unanswered[s.slug][repo.key()] = true
				previous, kept := d.pulls[s.slug][repo.id][repo.target.Branch]
				if !reuse || !kept {
					continue
				}
				rows = previous
			}
			if next[s.slug] == nil {
				next[s.slug] = map[string]map[string][]status.PullRequest{}
			}
			next[s.slug][repo.id] = map[string][]status.PullRequest{repo.target.Branch: rows}
		}
	}
	d.unanswered = unanswered
	if !answered {
		d.failed = true
		if reuse {
			return pullsCache{}, false
		}
		d.pulls, d.viewer, d.checkedAt, d.signedOut = next, viewer, time.Time{}, false
		return pullsCache{SchemaVersion: pullsCacheSchemaVersion, Viewer: viewer, Sessions: next}, true
	}
	d.pulls = next
	d.viewer, d.checkedAt = viewer, d.now()
	d.failed, d.signedOut, d.pausedUntil = false, false, time.Time{}
	if limited != nil {
		d.pausedUntil = limited.Until
	}
	delete(d.logged, "")
	return pullsCache{SchemaVersion: pullsCacheSchemaVersion, Viewer: d.viewer, CheckedAt: d.checkedAt, Sessions: next}, true
}

func writePullsCache(path string, cache pullsCache) error {
	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), pullsCacheDirMode); err != nil {
		return err
	}
	return atomicfile.Replace(path, b, pullsCacheFileMode)
}

func (d *Dashboard) logOnce(key string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logOnceLocked(key, err)
}

func (d *Dashboard) logOnceLocked(key string, err error) {
	if text := github.WithoutRequestID(err.Error()); d.logged[key] != text {
		log.Printf(dashboardLogFormat, err)
		d.logged[key] = text
	}
}

// measure takes every session's git stats, which only the cards show.
func (d *Dashboard) measure(ctx context.Context) {
	stats := map[string][]status.RepoStat{}
	for _, s := range d.scan() {
		if ctx.Err() != nil {
			return
		}
		stats[s.slug] = d.api.measure(ctx, s.root)
	}
	if ctx.Err() != nil {
		return
	}
	d.mu.Lock()
	d.stats = stats
	d.mu.Unlock()
	d.publish()
}
