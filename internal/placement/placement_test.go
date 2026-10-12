package placement

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/config"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
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

// interrupt leaves a session as a crash just after the rename would.
func (f *fixture) interrupt(dir string) {
	f.t.Helper()
	m, err := session.Load(dir)
	if err != nil {
		f.t.Fatal(err)
	}
	slug := filepath.Base(dir)
	mustWrite(f.t, filepath.Join(f.def, slug, moveMarker), marker(slug, m))
	if err := os.Rename(filepath.Join(f.def, slug), filepath.Join(f.work, slug)); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) link(dir, target string) {
	f.t.Helper()
	link := filepath.Join(dir, "thoughts")
	_ = os.Remove(link)
	if err := os.Symlink(target, link); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) scan() map[string]Item {
	f.t.Helper()
	return f.scanWith(newPlacer(f.cfg))
}

func (f *fixture) scanWith(p placer) map[string]Item {
	f.t.Helper()
	report, err := p.scan()
	if err != nil {
		f.t.Fatal(err)
	}
	out := map[string]Item{}
	for _, item := range report.Items {
		out[item.Slug] = item
	}
	return out
}

func (f *fixture) move(slugs ...string) map[string]Outcome {
	f.t.Helper()
	return f.moveWith(newPlacer(f.cfg), slugs...)
}

func (f *fixture) moveWith(p placer, slugs ...string) map[string]Outcome {
	f.t.Helper()
	out := map[string]Outcome{}
	for _, o := range p.move(slugs) {
		out[o.Slug] = o
	}
	return out
}

func (f *fixture) personalInWork(slug string) string {
	f.t.Helper()
	dir := f.legacy(slug, "kieranajp")
	if err := os.Rename(filepath.Join(f.def, slug), filepath.Join(f.work, slug)); err != nil {
		f.t.Fatal(err)
	}
	f.link(dir, filepath.Join(f.work, slug))
	return dir
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

func expectItem(t *testing.T, items map[string]Item, slug string, want State) Item {
	t.Helper()
	item, ok := items[slug]
	if !ok {
		t.Fatalf("%s: not reported", slug)
	}
	if item.State != want {
		t.Fatalf("%s: %s (%s), want %s", slug, item.State, item.Detail, want)
	}
	return item
}

func expectOutcome(t *testing.T, outcomes map[string]Outcome, slug string, want Result) Outcome {
	t.Helper()
	o, ok := outcomes[slug]
	if !ok {
		t.Fatalf("%s: no outcome", slug)
	}
	if o.Result != want {
		t.Fatalf("%s: %s (%s), want %s", slug, o.Result, o.Detail, want)
	}
	return o
}

func TestMovesAnOrgSessionOutOfTheDefaultRootIntoItsRoutedRoot(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("routed", "acme", "ACME")
	item := expectItem(t, f.scan(), "routed", StateMisplaced)
	if item.From.ID != config.DefaultThoughtsID || item.To.ID != "work" {
		t.Fatalf("from %+v to %+v", item.From, item.To)
	}
	expectOutcome(t, f.move("routed"), "routed", ResultMoved)
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
	if exists(filepath.Join(f.work, "routed", moveMarker)) {
		t.Fatal("move marker left behind")
	}
}

func TestAPersonalSessionInASharedFolderMovesOut(t *testing.T) {
	f := newFixture(t)
	dir := f.personalInWork("personal")
	item := expectItem(t, f.scan(), "personal", StateMisplaced)
	if item.From.ID != "work" || item.To.ID != config.DefaultThoughtsID {
		t.Fatalf("from %+v to %+v", item.From, item.To)
	}
	expectOutcome(t, f.move("personal"), "personal", ResultMoved)
	if got := readLink(t, dir); got != filepath.Join("..", "thoughts", "personal") {
		t.Fatalf("thoughts -> %q", got)
	}
	if got := thoughtsRoot(t, dir); got != config.DefaultThoughtsID {
		t.Fatalf("thoughtsRoot = %q", got)
	}
	if !isDir(filepath.Join(f.def, "personal", "shared", "research")) {
		t.Fatal("documents did not arrive in the default root")
	}
}

func TestScanMovesNothingMisplaced(t *testing.T) {
	f := newFixture(t)
	f.legacy("routed", "acme")
	f.personalInWork("personal")
	before := f.snapshot()
	items := f.scan()
	expectItem(t, items, "routed", StateMisplaced)
	expectItem(t, items, "personal", StateMisplaced)
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatalf("a scan changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

func TestSessionsAlreadyWhereTheyRouteAreRecordedAndNotReported(t *testing.T) {
	f := newFixture(t)
	private := f.legacy("private", "kieranajp")
	mixed := f.legacy("mixed", "acme", "kieranajp")
	scratch := f.legacy("scratch")
	linked := f.legacy("linked", "acme")
	if err := os.Rename(filepath.Join(f.def, "linked"), filepath.Join(f.work, "linked")); err != nil {
		t.Fatal(err)
	}
	f.link(linked, filepath.Join(f.work, "linked"))
	items := f.scan()
	if len(items) != 0 {
		t.Fatalf("placed sessions reported: %v", items)
	}
	for dir, want := range map[string]string{private: "default", mixed: "default", scratch: "default", linked: "work"} {
		if got := thoughtsRoot(t, dir); got != want {
			t.Errorf("%s: thoughtsRoot = %q, want %q", filepath.Base(dir), got, want)
		}
	}
}

func TestACrashBetweenRenameAndLinkSwapIsFinishedByTheNextScan(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("crashed", "acme")
	f.interrupt(dir)
	if err := os.Symlink("stale", filepath.Join(dir, "thoughts"+linkStaging)); err != nil {
		t.Fatal(err)
	}
	expectItem(t, f.scan(), "crashed", StateFinished)
	if got := readLink(t, dir); got != filepath.Join(f.work, "crashed") {
		t.Fatalf("thoughts -> %q", got)
	}
	if got := thoughtsRoot(t, dir); got != "work" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
	if exists(filepath.Join(f.work, "crashed", moveMarker)) {
		t.Fatal("move marker left behind")
	}
}

func TestAnInterruptedMoveThatCannotBeFinishedIsPending(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("crashed", "acme")
	f.interrupt(dir)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	report, err := newPlacer(f.cfg).scan()
	if err != nil {
		t.Fatal(err)
	}
	if !report.Pending() {
		t.Fatalf("report = %+v, want a pending move", report)
	}
}

func TestAnExistingDestinationIsAConflict(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("taken", "acme")
	mustMkdir(t, filepath.Join(f.work, "taken"))
	before := f.snapshot()
	expectItem(t, f.scan(), "taken", StateSkipped)
	expectOutcome(t, f.move("taken"), "taken", ResultConflict)
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a conflict changed the tree")
	}
	if got := thoughtsRoot(t, dir); got != "" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestSessionsThatCannotBeMovedAreSkippedUntouched(t *testing.T) {
	f := newFixture(t)
	elsewhere := filepath.Join(f.base, "elsewhere", "outside")
	mustMkdir(t, elsewhere)
	f.link(f.legacy("outside", "acme"), elsewhere)

	real := f.legacy("real-dir", "acme")
	_ = os.Remove(filepath.Join(real, "thoughts"))
	mustMkdir(t, filepath.Join(real, "thoughts"))

	f.legacy("broken", "acme")
	if err := os.RemoveAll(filepath.Join(f.def, "broken")); err != nil {
		t.Fatal(err)
	}

	before := f.snapshot()
	items := f.scan()
	for slug, want := range map[string]Reason{"outside": ReasonOutsideRoots, "real-dir": ReasonLinkBroken, "broken": ReasonLinkBroken} {
		if item := expectItem(t, items, slug, StateSkipped); item.Reason != want {
			t.Errorf("%s: reason %s, want %s", slug, item.Reason, want)
		}
		expectOutcome(t, f.move(slug), slug, ResultRefused)
	}
	if !strings.Contains(items["outside"].Detail, elsewhere) {
		t.Fatalf("outside detail = %q", items["outside"].Detail)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a session left alone was changed")
	}
}

func TestARecordedRootAndAnEscalatedSessionAreStillChecked(t *testing.T) {
	f := newFixture(t)
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
	items := f.scan()
	expectItem(t, items, "chosen", StateMisplaced)
	expectItem(t, items, "escalated", StateMisplaced)
	expectOutcome(t, f.move("chosen"), "chosen", ResultMoved)
	if got := thoughtsRoot(t, chosen); got != "work" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestARunningAgentSkipsTheSession(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("busy", "acme")
	if err := os.MkdirAll(sessionpaths.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, sessionpaths.AgentPID(dir), strconv.Itoa(os.Getpid()))
	before := f.snapshot()
	if item := expectItem(t, f.scan(), "busy", StateSkipped); item.Reason != ReasonAgentRunning {
		t.Fatalf("reason = %s", item.Reason)
	}
	expectOutcome(t, f.move("busy"), "busy", ResultRefused)
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a running agent's session was changed")
	}
}

func TestACrossDeviceMoveIsSkipped(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("far", "acme")
	before := f.snapshot()
	p := newPlacer(f.cfg)
	p.device = func(path string) uint64 {
		if strings.HasPrefix(path, f.work) {
			return 2
		}
		return 1
	}
	item := expectItem(t, f.scanWith(p), "far", StateSkipped)
	if item.Reason != ReasonCrossDevice || !strings.Contains(item.Detail, "cross-device") {
		t.Fatalf("item = %+v", item)
	}
	expectOutcome(t, f.moveWith(p, "far"), "far", ResultRefused)
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a refused move changed the tree")
	}
	if got := thoughtsRoot(t, dir); got != "" {
		t.Fatalf("thoughtsRoot = %q", got)
	}
}

func TestAMissingDestinationRootIsSkipped(t *testing.T) {
	f := newFixture(t)
	f.legacy("orphaned", "acme")
	if err := os.Remove(f.work); err != nil {
		t.Fatal(err)
	}
	before := f.snapshot()
	if item := expectItem(t, f.scan(), "orphaned", StateSkipped); item.Reason != ReasonDestinationMissing {
		t.Fatalf("reason = %s", item.Reason)
	}
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a skipped session changed the tree")
	}
}

func TestASecondMoveFindsNothingLeftToDo(t *testing.T) {
	f := newFixture(t)
	f.legacy("routed", "acme")
	f.legacy("private", "kieranajp")
	expectOutcome(t, f.move("routed"), "routed", ResultMoved)
	if items := f.scan(); len(items) != 0 {
		t.Fatalf("scan after the move: %v", items)
	}
	before := f.snapshot()
	expectOutcome(t, f.move("routed"), "routed", ResultRefused)
	if after := f.snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a second pass changed the tree")
	}
}

func TestAFailureInOneSessionDoesNotStopTheOthers(t *testing.T) {
	f := newFixture(t)
	f.legacy("one", "acme")
	f.legacy("two", "acme")
	outcomes := f.move("../escape", "one", "missing", "two")
	expectOutcome(t, outcomes, "../escape", ResultRefused)
	expectOutcome(t, outcomes, "missing", ResultRefused)
	expectOutcome(t, outcomes, "one", ResultMoved)
	expectOutcome(t, outcomes, "two", ResultMoved)
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
	f.move("routed", "loose", "orphan")
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

func TestAnUnmarkedDestinationIsNeverAdopted(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("borrowed", "acme")
	if err := os.RemoveAll(filepath.Join(f.def, "borrowed")); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(f.work, "borrowed", "shared", "research")
	mustMkdir(t, other)
	mustWrite(t, filepath.Join(other, "R1.md"), "# another machine's session")
	for _, mark := range []string{"", "borrowed 2020-01-01T00:00:00Z"} {
		if mark != "" {
			mustWrite(t, filepath.Join(f.work, "borrowed", moveMarker), mark)
		}
		before := f.snapshot()
		if item := expectItem(t, f.scan(), "borrowed", StateSkipped); item.Reason != ReasonConflict {
			t.Fatalf("marker %q: reason %s", mark, item.Reason)
		}
		if after := f.snapshot(); !reflect.DeepEqual(before, after) {
			t.Fatalf("marker %q: a conflict changed the tree", mark)
		}
		if got := thoughtsRoot(t, dir); got != "" {
			t.Fatalf("marker %q: thoughtsRoot = %q", mark, got)
		}
	}
}

func TestAStaleMoveMarkerInTheSourceIsClearedByScan(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("stale", "acme")
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(f.def, "stale", moveMarker), marker("stale", m))
	expectItem(t, f.scan(), "stale", StateMisplaced)
	if exists(filepath.Join(f.def, "stale", moveMarker)) {
		t.Fatal("stale marker left in the source folder")
	}
}

func TestAStaleMarkerThatCannotBeClearedIsPending(t *testing.T) {
	f := newFixture(t)
	dir := f.legacy("stuck", "acme")
	m, err := session.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(f.def, "stuck")
	mustWrite(t, filepath.Join(source, moveMarker), marker("stuck", m))
	if err := os.Chmod(source, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(source, 0o755) })
	report, err := newPlacer(f.cfg).scan()
	if err != nil {
		t.Fatal(err)
	}
	if !report.Pending() {
		t.Fatalf("report = %+v, want a pending marker", report)
	}
}
