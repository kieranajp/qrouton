package status

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// TodoItem is one open item of a session's todo documents. Path is
// session-relative and Line one-based, so a click opens the pane at the item.
type TodoItem struct {
	Text string `json:"text"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// TodoDocument is one todo document of a session. Path is session-relative and
// Open counts its open items.
type TodoDocument struct {
	Path  string `json:"path"`
	Title string `json:"title"`
	Open  int    `json:"open"`
}

// TodoRollup is what a session row says about its open todos. Items holds the
// next open item.
type TodoRollup struct {
	Open  int        `json:"open"`
	Items []TodoItem `json:"items"`
}

// TodosIn is a session's open todo items in document order, reading its todo
// documents oldest first.
func TodosIn(sessionRoot string) []TodoItem {
	_, items := readTodos(sessionRoot)
	return items
}

// TodoDocumentsIn is a session's todo documents ordered by number, each with
// its open count. A document with no open items is still listed.
func TodoDocumentsIn(sessionRoot string) []TodoDocument {
	docs, _ := readTodos(sessionRoot)
	return docs
}

func readTodos(sessionRoot string) ([]TodoDocument, []TodoItem) {
	thoughts := sessionpaths.Thoughts(sessionRoot)
	found := artifacts(filepath.Join(thoughts, todosDir), "")
	slices.SortStableFunc(found, func(a, b artifact) int {
		return cmp.Or(cmp.Compare(a.number, b.number), cmp.Compare(a.at, b.at))
	})
	docs := []TodoDocument{}
	items := []TodoItem{}
	for _, doc := range found {
		text, err := os.ReadFile(doc.path)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(sessionRoot, doc.path)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		title, ok := markdown.Title(string(text))
		if !ok {
			title = strings.TrimSuffix(filepath.Base(doc.path), filepath.Ext(doc.path))
		}
		open := 0
		for _, item := range markdown.TodoItems(string(text)) {
			if item.State != markdown.TodoOpen {
				continue
			}
			open++
			items = append(items, TodoItem{Text: item.Text, Path: rel, Line: item.Line})
		}
		docs = append(docs, TodoDocument{Path: rel, Title: title, Open: open})
	}
	return docs, items
}

// Rollup counts open items in document order and picks the next one for a card.
func Rollup(items []TodoItem) TodoRollup {
	out := TodoRollup{Open: len(items), Items: []TodoItem{}}
	if len(items) > 0 {
		out.Items = append(out.Items, items[0])
	}
	return out
}

// Todos maps every session's slug to its roll-up.
func Todos(root string) map[string]TodoRollup {
	found, err := session.Scan(root)
	if err != nil {
		return map[string]TodoRollup{}
	}
	out := make(map[string]TodoRollup, len(found))
	for _, m := range found {
		out[m.Slug] = Rollup(TodosIn(filepath.Join(root, m.Slug)))
	}
	return out
}
