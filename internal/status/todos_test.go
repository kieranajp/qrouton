package status

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kieranajp/qrouton/internal/session"
)

func todoDoc(t *testing.T, dir, name string, at time.Time, lines ...string) {
	t.Helper()
	path := filepath.Join(dir, "thoughts", "shared", "todos", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// Catches documents read newest first, a done or not-needed item counted as
// open, a blocking next item listed twice, or another folder's tasks counted.
func TestTodosRollUpOpenItemsFromEveryTodoDocumentOldestFirst(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "rollout"})
	now := time.Now()
	todoDoc(t, dir, "T2-later.md", now.Add(-time.Hour),
		"# Later",
		"",
		"## Ship",
		"",
		"- [ ] Announce it",
		"- [ ] Rotate the token",
		"  - Blocks: the agent",
	)
	todoDoc(t, dir, "T1-first.md", now,
		"---",
		"kind: todo",
		"---",
		"",
		"# First",
		"",
		"## Before merge",
		"",
		"- [x] Merge acme/api#12",
		"- [-] Roll back the cache",
		"- [ ] Apply the migration",
		"  - Where: enclave · production",
		"  - Blocks: Phase 3",
	)
	plan := filepath.Join(dir, "thoughts", "shared", "plans", "P1-plan.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan, []byte("- [ ] Ship the plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	items := TodosIn(dir)
	want := []TodoItem{
		{Text: "Apply the migration", Section: "Before merge", Where: "enclave · production", Blocks: "Phase 3", Path: "thoughts/shared/todos/T1-first.md", Line: 11},
		{Text: "Announce it", Section: "Ship", Path: "thoughts/shared/todos/T2-later.md", Line: 5},
		{Text: "Rotate the token", Section: "Ship", Blocks: "the agent", Path: "thoughts/shared/todos/T2-later.md", Line: 6},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("open items = %+v\nwant %+v", items, want)
	}

	rollup := Todos(root)["rollout"]
	wantRollup := TodoRollup{Open: 3, Blocking: 2, Items: []TodoItem{want[0], want[2]}}
	if !reflect.DeepEqual(rollup, wantRollup) {
		t.Fatalf("roll-up = %+v\nwant %+v", rollup, wantRollup)
	}
}

// Catches a next item dropped when it does not block, or listed ahead of the
// blocking ones.
func TestARollupListsBlockingItemsThenTheNextOpenOne(t *testing.T) {
	next := TodoItem{Text: "Merge", Line: 3}
	blocking := TodoItem{Text: "Apply", Blocks: "Phase 3", Line: 4}
	got := Rollup([]TodoItem{next, blocking, {Text: "Announce", Line: 5}})
	want := TodoRollup{Open: 3, Blocking: 1, Items: []TodoItem{blocking, next}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("roll-up = %+v, want %+v", got, want)
	}
}

// Catches a nil list reaching the page as JSON null for a session with no todos.
func TestASessionWithNoTodosFolderRollsUpEmpty(t *testing.T) {
	root := t.TempDir()
	dir := sessionDir(t, root, session.Manifest{Slug: "bare"})
	if items := TodosIn(dir); items == nil || len(items) != 0 {
		t.Fatalf("open items = %#v, want an empty list", items)
	}
	rollup, ok := Todos(root)["bare"]
	if !ok || rollup.Open != 0 || rollup.Items == nil || len(rollup.Items) != 0 {
		t.Fatalf("roll-up = %#v, want an empty one", rollup)
	}
	if row := Sessions(root)[0]; row.Todos.Items == nil {
		t.Fatal("a session row starts with a nil todo list")
	}
}
