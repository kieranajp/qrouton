package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

type outcome string

const (
	moved     outcome = "moved"
	unchanged outcome = "unchanged"
	finished  outcome = "finished"
	conflict  outcome = "conflict"
	skipped   outcome = "skipped"
	refused   outcome = "refused"
	failed    outcome = "failed"
)

var outcomes = []outcome{moved, unchanged, finished, conflict, skipped, refused, failed}

const linkStaging = ".moving"

type result struct {
	Slug    string
	Outcome outcome
	Detail  string
}

type mover struct {
	cfg    *config.Config
	dryRun bool
	device func(string) uint64
}

// run considers only direct children of the sessions root with a manifest, so
// a thoughts directory without a session is never touched.
func (mv mover) run() ([]result, error) {
	entries, err := os.ReadDir(mv.cfg.Root)
	if err != nil {
		return nil, err
	}
	var out []result
	for _, e := range entries {
		dir := filepath.Join(mv.cfg.Root, e.Name())
		m, err := session.Load(dir)
		switch {
		case !e.IsDir() || errors.Is(err, os.ErrNotExist):
		case err != nil:
			out = append(out, result{Slug: e.Name(), Outcome: skipped, Detail: "unreadable manifest: " + err.Error()})
		default:
			out = append(out, mv.one(dir, m))
		}
	}
	return out, nil
}

// one moves only a session whose repos all route to an extra root. Anything
// doubtful stays where it is.
func (mv mover) one(dir string, m session.Manifest) result {
	r := result{Slug: filepath.Base(dir), Outcome: skipped}
	orgs := make([]string, len(m.Repos))
	for i, repo := range m.Repos {
		orgs[i] = repo.Org
	}
	routed := mv.cfg.RouteThoughts(orgs)
	target, err := os.Readlink(filepath.Join(dir, sessionpaths.ThoughtsDirName))
	switch {
	case m.ThoughtsRoot != "":
		r.Detail = "thoughtsRoot is " + m.ThoughtsRoot
	case m.Escalation != nil:
		r.Detail = "escalated from a repo-free session"
	case routed.ID == config.DefaultThoughtsID:
		r.Detail = "routes to the default root"
	case err != nil:
		r.Detail = "thoughts is not a symlink"
	}
	if r.Detail != "" {
		return r
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	target = resolve(target)
	home := filepath.Join(routed.Path, r.Slug)
	from, to := resolve(filepath.Join(mv.cfg.ThoughtsRoots()[0].Path, r.Slug)), resolve(home)
	r.Detail = from + " -> " + to
	switch {
	case target == to && isDir(to):
		r.Outcome, r.Detail = unchanged, to
		return mv.apply(r, func() error { return record(dir, routed.ID) })
	case target != from:
		r.Detail = "links outside the default and routed roots: " + target
	case exists(from) && exists(to):
		r.Outcome, r.Detail = conflict, to+" already exists"
	case !exists(from) && isDir(to):
		r.Outcome = finished
		return mv.apply(r, func() error { return mv.relink(dir, home, routed.ID) })
	case !isDir(from):
		r.Detail = "link is broken or not a directory: " + target
	case mv.device(routed.Path) == 0:
		r.Outcome, r.Detail = refused, "routed root is missing: "+routed.Path
	case mv.device(from) != mv.device(routed.Path):
		r.Outcome, r.Detail = refused, "cross-device move: "+r.Detail
	default:
		r.Outcome = moved
		return mv.apply(r, func() error {
			if err := os.Rename(from, to); err != nil {
				return err
			}
			return mv.relink(dir, home, routed.ID)
		})
	}
	return r
}

func (mv mover) apply(r result, change func() error) result {
	if mv.dryRun {
		return r
	}
	if err := change(); err != nil {
		r.Outcome, r.Detail = failed, r.Detail+": "+err.Error()
	}
	return r
}

// relink swaps the link before recording, so a crash in between leaves a
// session the next run finishes.
func (mv mover) relink(dir, home, id string) error {
	link := filepath.Join(dir, sessionpaths.ThoughtsDirName)
	staged := link + linkStaging
	_ = os.Remove(staged)
	if err := os.Symlink(session.ThoughtsLink(mv.cfg.Root, dir, home), staged); err != nil {
		return err
	}
	if err := os.Rename(staged, link); err != nil {
		return err
	}
	return record(dir, id)
}

func record(dir, id string) error {
	return session.UpdateManifest(dir, func(m session.Manifest) (session.Manifest, error) {
		m.ThoughtsRoot = id
		return m, nil
	})
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
