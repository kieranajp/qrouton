// Package placement finds sessions whose thoughts folder sits in a different
// root from the one their repositories route to, and moves them.
package placement

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type State string

const (
	StatePlaced    State = "placed"
	StateMisplaced State = "misplaced"
	StateFinished  State = "finished"
	StatePending   State = "pending"
	StateSkipped   State = "skipped"
)

type Reason string

const (
	ReasonAgentRunning       Reason = "agent-running"
	ReasonDestinationMissing Reason = "destination-missing"
	ReasonCrossDevice        Reason = "cross-device"
	ReasonLinkBroken         Reason = "link-broken"
	ReasonConflict           Reason = "conflict"
	ReasonOutsideRoots       Reason = "outside-roots"
	ReasonUnreadable         Reason = "unreadable"
)

type Result string

const (
	ResultMoved    Result = "moved"
	ResultConflict Result = "conflict"
	ResultRefused  Result = "refused"
	ResultFailed   Result = "failed"
)

type Root struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Item struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	State  State  `json:"state"`
	Reason Reason `json:"reason,omitempty"`
	Detail string `json:"detail,omitempty"`
	From   Root   `json:"from"`
	To     Root   `json:"to"`
}

type Report struct {
	Items []Item `json:"items"`
}

// Misplaced lists the sessions a move would fix.
func (r Report) Misplaced() []Item { return r.with(StateMisplaced) }

// Skipped lists the sessions that cannot be moved, with their reasons.
func (r Report) Skipped() []Item { return r.with(StateSkipped) }

// Pending reports whether an interrupted move is still unfinished.
func (r Report) Pending() bool { return len(r.with(StatePending)) > 0 }

// PendingItems lists the sessions whose interrupted move could not be finished.
func (r Report) PendingItems() []Item { return r.with(StatePending) }

func (r Report) with(state State) []Item {
	var out []Item
	for _, item := range r.Items {
		if item.State == state {
			out = append(out, item)
		}
	}
	return out
}

type Outcome struct {
	Slug   string `json:"slug"`
	Result Result `json:"result"`
	Detail string `json:"detail,omitempty"`
}

type placer struct {
	cfg    *config.Config
	device func(string) uint64
}

func newPlacer(cfg *config.Config) placer { return placer{cfg: cfg, device: device} }

// Scan inspects every session under the sessions root. It finishes an
// interrupted move and records a root a session already sits in, and moves
// nothing else.
func Scan(cfg *config.Config) (Report, error) { return newPlacer(cfg).scan() }

// Move moves each named session on its own and reports one outcome apiece.
func Move(cfg *config.Config, slugs []string) []Outcome { return newPlacer(cfg).move(slugs) }

func (p placer) scan() (Report, error) {
	entries, err := os.ReadDir(p.cfg.Root)
	if err != nil {
		return Report{}, fmt.Errorf("%w: %w", ErrNoSessions, err)
	}
	report := Report{Items: []Item{}}
	for _, e := range entries {
		dir := filepath.Join(p.cfg.Root, e.Name())
		if !e.IsDir() {
			continue
		}
		m, err := session.Load(dir)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			report.Items = append(report.Items, Item{Slug: e.Name(), Name: e.Name(), State: StateSkipped,
				Reason: ReasonUnreadable, Detail: fmt.Sprintf(detailUnreadableFmt, err)})
		default:
			item, act := p.inspect(dir, m)
			if item.State == StateMisplaced {
				if err := p.clearStaleMarker(slugOf(dir), m, item); err != nil {
					item.State, item.Detail = StatePending, fmt.Sprintf(detailPendingFormat, err)
				}
			}
			if act != nil && item.State != StateMisplaced && item.State != StatePending {
				if err := act(); err != nil {
					item.State, item.Detail = StatePending, fmt.Sprintf(detailPendingFormat, err)
				}
			}
			if item.State != StatePlaced || item.Detail != "" {
				report.Items = append(report.Items, item)
			}
		}
	}
	return report, nil
}

// clearStaleMarker removes the marker a move wrote before its rename never
// happened, so no marker lingers in a folder a sync tool shares.
func (p placer) clearStaleMarker(slug string, m session.Manifest, item Item) error {
	source := resolve(filepath.Join(item.From.Path, slug))
	if !marked(source, marker(slug, m)) {
		return nil
	}
	return os.Remove(filepath.Join(source, moveMarker))
}

func slugOf(dir string) string { return filepath.Base(dir) }

func (p placer) move(slugs []string) []Outcome {
	out := make([]Outcome, 0, len(slugs))
	for _, slug := range slugs {
		out = append(out, p.moveOne(slug))
	}
	return out
}

func (p placer) moveOne(slug string) Outcome {
	o := Outcome{Slug: slug, Result: ResultRefused}
	if slug == "" || slug != filepath.Base(slug) || slug == "." || slug == ".." {
		o.Detail = detailNotASlug
		return o
	}
	dir := filepath.Join(p.cfg.Root, slug)
	m, err := session.Load(dir)
	if err != nil {
		o.Detail = fmt.Sprintf(detailUnreadableFmt, err)
		return o
	}
	item, act := p.inspect(dir, m)
	o.Detail = item.Detail
	switch {
	case item.State == StateMisplaced:
		if err := act(); err != nil {
			o.Result, o.Detail = ResultFailed, item.Detail+": "+err.Error()
			return o
		}
		o.Result = ResultMoved
	case item.Reason == ReasonConflict:
		o.Result = ResultConflict
	case item.State == StatePlaced || item.State == StateFinished:
		o.Detail = detailNotMisplaced
	}
	return o
}

// inspect classifies one session without changing anything. act, when
// non-nil, makes the classification true: the move for a misplaced session,
// and the finishing or recording step for an interrupted or unrecorded one.
func (p placer) inspect(dir string, m session.Manifest) (Item, func() error) {
	slug := filepath.Base(dir)
	item := Item{Slug: slug, Name: m.DisplayName(), State: StateSkipped}
	orgs := make([]string, len(m.Repos))
	for i, repo := range m.Repos {
		orgs[i] = repo.Org
	}
	routed := p.cfg.RouteThoughts(orgs)
	item.To = Root{ID: routed.ID, Path: routed.Path}
	home := filepath.Join(routed.Path, slug)
	to := resolve(home)

	link := filepath.Join(dir, sessionpaths.ThoughtsDirName)
	info, err := os.Lstat(link)
	if err != nil {
		item.State = StatePlaced
		return item, nil
	}
	target, err := os.Readlink(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		item.Reason, item.Detail = ReasonLinkBroken, fmt.Sprintf(detailBrokenFormat, link)
		return item, nil
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	target = resolve(target)

	if target == to && isDir(to) {
		item.State, item.From = StatePlaced, item.To
		if m.ThoughtsRoot == routed.ID && !marked(to, marker(slug, m)) {
			return item, nil
		}
		return item, func() error { return settle(dir, home, routed.ID) }
	}

	from, ok := p.sourceRoot(slug, target)
	if !ok {
		item.From = Root{Path: target}
		item.Reason, item.Detail = ReasonOutsideRoots, fmt.Sprintf(detailOutsideFormat, target)
		return item, nil
	}
	item.From = from
	source := resolve(filepath.Join(from.Path, slug))
	item.Detail = fmt.Sprintf(detailMoveFormat, source, to)
	mark := marker(slug, m)

	switch {
	case exists(source) && exists(to):
		item.Reason, item.Detail = ReasonConflict, fmt.Sprintf(detailExistsFormat, to)
	case !exists(source) && isDir(to) && !marked(to, mark):
		item.Reason, item.Detail = ReasonConflict, fmt.Sprintf(detailUnmarkedFormat, to)
	case !exists(source) && isDir(to):
		item.State = StateFinished
		return item, func() error { return p.relink(dir, home, routed.ID) }
	case !isDir(source):
		item.Reason, item.Detail = ReasonLinkBroken, fmt.Sprintf(detailBrokenFormat, target)
	case p.device(routed.Path) == 0:
		item.Reason, item.Detail = ReasonDestinationMissing, fmt.Sprintf(detailMissingFormat, routed.Path)
	case p.device(source) != p.device(routed.Path):
		item.Reason, item.Detail = ReasonCrossDevice, fmt.Sprintf(detailCrossFormat, item.Detail)
	default:
		if pid, alive := session.AgentAlive(dir); alive {
			item.Reason, item.Detail = ReasonAgentRunning, fmt.Sprintf(detailAgentFormat, pid)
			return item, nil
		}
		item.State = StateMisplaced
		return item, func() error {
			if err := os.WriteFile(filepath.Join(source, moveMarker), []byte(mark), markerMode); err != nil {
				return err
			}
			if err := os.Rename(source, to); err != nil {
				return err
			}
			return p.relink(dir, home, routed.ID)
		}
	}
	return item, nil
}

// sourceRoot is the configured root whose folder for this slug the link points at.
func (p placer) sourceRoot(slug, target string) (Root, bool) {
	for _, r := range p.cfg.ThoughtsRoots() {
		if resolve(filepath.Join(r.Path, slug)) == target {
			return Root{ID: r.ID, Path: r.Path}, true
		}
	}
	return Root{}, false
}

// relink swaps the link before recording, so a crash in between leaves a
// session the next scan finishes.
func (p placer) relink(dir, home, id string) error {
	link := filepath.Join(dir, sessionpaths.ThoughtsDirName)
	staged := link + linkStaging
	_ = os.Remove(staged)
	if err := os.Symlink(session.ThoughtsLink(p.cfg.Root, dir, home), staged); err != nil {
		return err
	}
	if err := os.Rename(staged, link); err != nil {
		return err
	}
	return settle(dir, home, id)
}

func settle(dir, home, id string) error {
	err := session.UpdateManifest(dir, func(m session.Manifest) (session.Manifest, error) {
		m.ThoughtsRoot = id
		return m, nil
	})
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(home, moveMarker)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// marker names the session a move is for, so a rescan adopts only its own destination.
func marker(slug string, m session.Manifest) string {
	return slug + " " + m.CreatedAt.UTC().Format(time.RFC3339Nano)
}

func marked(dir, want string) bool {
	b, err := os.ReadFile(filepath.Join(dir, moveMarker))
	return err == nil && string(b) == want
}

// device answers 0 for a path that is missing or not a directory.
func device(path string) uint64 {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return 0
	}
	return uint64(info.Sys().(*syscall.Stat_t).Dev)
}

// resolve follows symlinks up to the last element, so a broken link's target
// and a missing destination still compare.
func resolve(path string) string {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Join(parent, filepath.Base(path))
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}
