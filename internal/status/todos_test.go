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
// open, another folder's tasks counted, or a document dropped or retitled in
// the document list.
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
		{Text: "Apply the migration", Path: "thoughts/shared/todos/T1-first.md", Line: 11},
		{Text: "Announce it", Path: "thoughts/shared/todos/T2-later.md", Line: 5},
		{Text: "Rotate the token", Path: "thoughts/shared/todos/T2-later.md", Line: 6},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("open items = %+v\nwant %+v", items, want)
	}

	wantDocs := []TodoDocument{
		{Path: "thoughts/shared/todos/T1-first.md", Title: "First", Open: 1},
		{Path: "thoughts/shared/todos/T2-later.md", Title: "Later", Open: 2},
	}
	if docs := TodoDocumentsIn(dir); !reflect.DeepEqual(docs, wantDocs) {
		t.Fatalf("documents = %+v\nwant %+v", docs, wantDocs)
	}

	rollup := Todos(root)["rollout"]
	wantRollup := TodoRollup{Open: 3, Items: []TodoItem{want[0]}}
	if !reflect.DeepEqual(rollup, wantRollup) {
		t.Fatalf("roll-up = %+v\nwant %+v", rollup, wantRollup)
	}
}

// Catches the card listing every open item, or one other than the first.
func TestARollupListsOnlyTheNextOpenItem(t *testing.T) {
	next := TodoItem{Text: "Merge", Line: 3}
	got := Rollup([]TodoItem{next, {Text: "Apply", Line: 4}, {Text: "Announce", Line: 5}})
	want := TodoRollup{Open: 3, Items: []TodoItem{next}}
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
	if docs := TodoDocumentsIn(dir); docs == nil || len(docs) != 0 {
		t.Fatalf("documents = %#v, want an empty list", docs)
	}
	if row := Sessions(root)[0]; row.Todos.Items == nil {
		t.Fatal("a session row starts with a nil todo list")
	}
}

// Catches a todo document with no heading, or no open item, missing from the
// document list, or titled with its extension.
func TestATodoDocumentWithoutHeadingOrOpenItemsIsStillListed(t *testing.T) {
	dir := sessionDir(t, t.TempDir(), session.Manifest{Slug: "plain"})
	todoDoc(t, dir, "T4-bare.md", time.Now(), "- [x] Done")
	want := []TodoDocument{{Path: "thoughts/shared/todos/T4-bare.md", Title: "T4-bare", Open: 0}}
	if docs := TodoDocumentsIn(dir); !reflect.DeepEqual(docs, want) {
		t.Fatalf("documents = %+v\nwant %+v", docs, want)
	}
}
