package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
)

type fixture struct {
	t                     *testing.T
	base, root, def, work string
	cfg                   *config.Config
}

// newFixture maps org acme to a work root beside the sessions root.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, base: base, root: filepath.Join(base, "sessions"), work: filepath.Join(base, "work")}
	f.def = filepath.Join(f.root, "thoughts")
	for _, d := range []string{f.def, f.work} {
		mustMkdir(t, d)
	}
	f.cfg = &config.Config{Root: f.root, Thoughts: config.Thoughts{Roots: []config.ThoughtsRoot{
		{ID: "work", Path: f.work, Orgs: []string{"acme"}},
	}}}
	return f
}

// legacy writes a pre-roots session linked to <default>/<slug>.
func (f *fixture) legacy(slug string, orgs ...string) string {
	f.t.Helper()
	dir := filepath.Join(f.root, slug)
	mustMkdir(f.t, dir)
	m := session.Manifest{SchemaVersion: 2, Name: slug, Slug: slug, CreatedAt: time.Now()}
	for _, org := range orgs {
		m.Repos = append(m.Repos, session.ManifestRepo{Name: "repo-" + org, Org: org, WorktreePath: "src/" + org})
	}
	if err := session.WriteManifest(dir, m); err != nil {
		f.t.Fatal(err)
	}
	home := filepath.Join(f.def, slug, "shared", "research")
	mustMkdir(f.t, home)
	mustWrite(f.t, filepath.Join(home, "R1.md"), "# "+slug)
	f.link(dir, filepath.Join("..", "thoughts", slug))
	return dir
}

func (f *fixture) link(dir, target string) {
	f.t.Helper()
	link := filepath.Join(dir, "thoughts")
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) run(dryRun bool) map[string]result {
	f.t.Helper()
	return f.runWith(mover{cfg: f.cfg, dryRun: dryRun, device: device})
}

func (f *fixture) runWith(mv mover) map[string]result {
	f.t.Helper()
	results, err := mv.run()
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[string]result{}
	for _, r := range results {
		out[r.Slug] = r
	}
	return out
}

// snapshot records every path under base with its kind, link target and bytes.
func (f *fixture) snapshot() map[string]string {
	f.t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(f.base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(f.base, path)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(path)
			out[rel] = "link " + target
		case d.IsDir():
			out[rel] = "dir"
		default:
			b, _ := os.ReadFile(path)
			out[rel] = "file " + string(b)
		}
		return nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return out
}

func thoughtsRoot(t *testing.T, dir string) string {
	t.Helper()
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return m.ThoughtsRoot
}

func readLink(t *testing.T, dir string) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(dir, "thoughts"))
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func expect(t *testing.T, results map[string]result, slug string, want outcome) result {
	t.Helper()
	r, ok := results[slug]
	if !ok {
		t.Fatalf("%s: no result", slug)
	}
	if r.Outcome != want {
		t.Fatalf("%s: %s (%s), want %s", slug, r.Outcome, r.Detail, want)
	}
	return r
}

func TestMovesALegacySessionIntoItsRoutedRoot(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("routed", "acme", "ACME")
	expect(t, f.run(false), "routed", moved)
	if exists(filepath.Join(f.def, "routed")) {
		t.Fatal("source left in the default root")
	}
	if got := readLink(t, dir); got != filepath.Join(f.work, "routed") {
		t.Fatalf("thoughts -> %q", got)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "thoughts", "shared", "research", "R1.md")); err != nil || string(b) != "# routed" {
		t.Fatalf("document through the link: %q, %v", b, err)
	}
	if got := thoughtsRoot(t, dir); got != "work" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
	if exists(filepath.Join(dir, "thoughts"+linkStaging)) {
		t.Fatal("staged link left behind")
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	f := newFixture(t)
	f.legacy("routed", "acme")
	f.legacy("private", "kieranajp")
	crashed := f.legacy("crashed", "acme")
	if err := os.Rename(filepath.Join(f.def, "crashed"), filepath.Join(f.work, "crashed")); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot()
	results := f.run(true)
	expect(t, results, "routed", moved)
	expect(t, results, "private", skipped)
	expect(t, results, "crashed", finished)
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("dry run changed the tree:\nbefore %v\nafter  %v", before, after)
	}
	if got := thoughtsRoot(t, crashed); got != "" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestSessionsThatRouteToTheDefaultAreSkippedUntouched(t *testing.T) {
	f := newFixture(t)
	f.legacy("private", "kieranajp")
	f.legacy("mixed", "acme", "kieranajp")
	f.legacy("scratch")
	before := f.snapshot()
	results := f.run(false)
	for _, slug := range []string{"private", "mixed", "scratch"} {
		expect(t, results, slug, skipped)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a default-routed session was changed")
	}
}

func TestALinkAlreadyInTheRoutedRootIsRecorded(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("linked", "acme")
	if err := os.Rename(filepath.Join(f.def, "linked"), filepath.Join(f.work, "linked")); err != nil {
		t.Fatal(err)
	}
	f.link(dir, filepath.Join(f.work, "linked"))
	expect(t, f.run(false), "linked", unchanged)
	if got := thoughtsRoot(t, dir); got != "work" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestACrashBetweenRenameAndLinkSwapIsFinished(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("crashed", "acme")
	if err := os.Rename(filepath.Join(f.def, "crashed"), filepath.Join(f.work, "crashed")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("stale", filepath.Join(dir, "thoughts"+linkStaging)); err != nil {
		t.Fatal(err)
	}
	expect(t, f.run(false), "crashed", finished)
	if got := readLink(t, dir); got != filepath.Join(f.work, "crashed") {
		t.Fatalf("thoughts -> %q", got)
	}
	if got := thoughtsRoot(t, dir); got != "work" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestAnExistingDestinationIsAConflict(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("taken", "acme")
	mustMkdir(t, filepath.Join(f.work, "taken"))
	before := f.snapshot()
	r := f.run(false)["taken"]
	if r.Outcome != conflict {
		t.Fatalf("outcome = %s (%s)", r.Outcome, r.Detail)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a conflict changed the tree")
	}
	if got := thoughtsRoot(t, dir); got != "" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestSessionsOutsideTheMoveAreLeftAlone(t *testing.T) {
	f := newFixture(t)
	elsewhere := filepath.Join(f.base, "elsewhere", "outside")
	mustMkdir(t, elsewhere)
	f.link(f.legacy("outside", "acme"), elsewhere)

	real := f.legacy("real-dir", "acme")
	_ = os.Remove(filepath.Join(real, "thoughts"))
	mustMkdir(t, filepath.Join(real, "thoughts"))

	chosen := f.legacy("chosen", "acme")
	if err := session.UpdateManifest(chosen, func(m session.Manifest) (session.Manifest, error) {
		m.ThoughtsRoot = config.DefaultThoughtsID
		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	escalated := f.legacy("escalated", "acme")
	if err := session.UpdateManifest(escalated, func(m session.Manifest) (session.Manifest, error) {
		m.Escalation = &session.EscalationOutcome{Status: session.EscalationConfirmed, At: time.Now()}
		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	f.legacy("broken", "acme")
	if err := os.RemoveAll(filepath.Join(f.def, "broken")); err != nil {
		t.Fatal(err)
	}

	before := f.snapshot()
	results := f.run(false)
	for _, slug := range []string{"outside", "real-dir", "chosen", "escalated", "broken"} {
		expect(t, results, slug, skipped)
	}
	if !strings.Contains(results["outside"].Detail, elsewhere) {
		t.Fatalf("outside detail = %q", results["outside"].Detail)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a session left alone was changed")
	}
}

func TestACrossDeviceMoveIsRefused(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("far", "acme")
	before := f.snapshot()
	fake := func(path string) uint64 {
		if strings.HasPrefix(path, f.work) {
			return 2
		}
		return 1
	}
	r := f.runWith(mover{cfg: f.cfg, device: fake})["far"]
	if r.Outcome != refused || !strings.Contains(r.Detail, "cross-device") {
		t.Fatalf("outcome = %s (%s)", r.Outcome, r.Detail)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a refused move changed the tree")
	}
	if got := thoughtsRoot(t, dir); got != "" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestAMissingRoutedRootIsRefused(t *testing.T) {
	f := newFixture(t)
	f.legacy("orphaned", "acme")
	if err := os.Remove(f.work); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot()
	if r := f.run(false)["orphaned"]; r.Outcome != refused {
		t.Fatalf("outcome = %s (%s)", r.Outcome, r.Detail)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a refused move changed the tree")
	}
}

func TestASecondRunMovesNothing(t *testing.T) {
	f := newFixture(t)
	f.legacy("routed", "acme")
	f.legacy("private", "kieranajp")
	f.run(false)
	before := f.snapshot()
	for slug, r := range f.run(false) {
		if r.Outcome != skipped {
			t.Fatalf("%s: %s (%s)", slug, r.Outcome, r.Detail)
		}
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a second run changed the tree")
	}
}

func TestThoughtsWithoutASessionAreNeverTouched(t *testing.T) {
	f := newFixture(t)
	orphan := filepath.Join(f.def, "orphan", "shared", "plans")
	mustMkdir(t, orphan)
	mustWrite(t, filepath.Join(orphan, "P1.md"), "# orphan")
	loose := filepath.Join(f.root, "loose")
	mustMkdir(t, loose)
	f.link(loose, filepath.Join("..", "thoughts", "orphan"))
	f.legacy("routed", "acme")
	f.run(false)
	if b, err := os.ReadFile(filepath.Join(orphan, "P1.md")); err != nil || string(b) != "# orphan" {
		t.Fatalf("orphan changed: %q, %v", b, err)
	}
	if got := readLink(t, loose); got != filepath.Join("..", "thoughts", "orphan") {
		t.Fatalf("loose thoughts -> %q", got)
	}
	if exists(filepath.Join(f.work, "orphan")) {
		t.Fatal("orphan reached the work root")
	}
}
