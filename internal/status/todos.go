package status

import (
	"cmp"
	"os"
	"path/filepath"
	"slices"

	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/session"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
)

// TodoItem is one open item of a session's todo documents. Path is
// session-relative and Line one-based, so a click opens the pane at the item.
type TodoItem struct {
	Text    string `json:"text"`
	Section string `json:"section"`
	Where   string `json:"where"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
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
	thoughts := sessionpaths.Thoughts(sessionRoot)
	found := artifacts(filepath.Join(thoughts, todosDir), "")
	slices.SortStableFunc(found, func(a, b artifact) int {
		return cmp.Or(cmp.Compare(a.number, b.number), cmp.Compare(a.at, b.at))
	})
	out := []TodoItem{}
	for _, doc := range found {
		text, err := os.ReadFile(doc.path)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(sessionRoot, doc.path)
		if err != nil {
			continue
		}
		for _, item := range markdown.TodoItems(string(text)) {
			if item.State != markdown.TodoOpen {
				continue
			}
			out = append(out, TodoItem{
				Text: item.Text, Section: item.Section, Where: item.Where,
				Path: filepath.ToSlash(rel), Line: item.Line,
			})
		}
	}
	return out
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
