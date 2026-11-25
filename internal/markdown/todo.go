package markdown

import (
	"regexp"
	"sort"
	"strings"

	"github.com/yuin/goldmark/ast"
)

type TodoState string

const (
	TodoOpen    TodoState = "open"
	TodoDone    TodoState = "done"
	TodoSkipped TodoState = "skipped"
)

// TodoItem is one task-list item of a todo document. Text and Note are the
// item line's source, split at its last spaced dash. Line counts from the
// document's first line, frontmatter included.
type TodoItem struct {
	Section string
	Text    string
	Note    string
	State   TodoState
	Line    int
	Where   string
	Check   string
	Run     string
}

var (
	todoLine  = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[([ xX-])\][ \t]+(\S.*?)\s*$`)
	todoField = regexp.MustCompile(`^(?:[-*+]\s+)?(Where|Check|Run):[ \t]*(.*?)\s*$`)
)

const todoNote = " — "

// TodoItems reads a todo document's items in document order. GFM knows only
// `[ ]` and `[x]`, so the box is read from the item's source line.
func TodoItems(text string) []TodoItem {
	_, body, first := Split(text)
	source := []byte(body)
	r := todoReader{lines: strings.Split(body, "\n")}
	for at, b := range source {
		if b == '\n' {
			r.starts = append(r.starts, at+1)
		}
	}
	var items []TodoItem
	current := ""
	for node := parse(source).FirstChild(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*ast.Heading); ok && heading.Level == 2 {
			if name := plainHeading(heading, source); name != "" {
				current = name
			}
			continue
		}
		_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering || child.Kind() != ast.KindListItem {
				return ast.WalkContinue, nil
			}
			line, found := r.item(child)
			if found == nil {
				return ast.WalkContinue, nil
			}
			item := TodoItem{Section: current, Text: found[2], State: todoStateOf(found[1]), Line: line + first - 1}
			if at := strings.LastIndex(found[2], todoNote); at >= 0 {
				item.Text, item.Note = strings.TrimSpace(found[2][:at]), strings.TrimSpace(found[2][at+len(todoNote):])
			}
			for _, at := range r.body(child, line) {
				if field := todoField.FindStringSubmatch(strings.TrimSpace(r.lines[at-1])); field != nil {
					item.setField(field[1], field[2])
				}
			}
			items = append(items, item)
			return ast.WalkContinue, nil
		})
	}
	return items
}

func todoStateOf(box string) TodoState {
	switch box {
	case "x", "X":
		return TodoDone
	case "-":
		return TodoSkipped
	default:
		return TodoOpen
	}
}

func (item *TodoItem) setField(key, value string) {
	switch key {
	case "Where":
		item.Where = value
	case "Check":
		item.Check = value
	case "Run":
		item.Run = value
	}
}

// todoReader maps goldmark's byte offsets back to the body's source lines.
type todoReader struct {
	lines  []string
	starts []int
}

func (r todoReader) lineAt(offset int) int {
	return sort.Search(len(r.starts), func(i int) bool { return r.starts[i] > offset }) + 1
}

// item is a list item's line and its todo match, or no match when the item is
// not a todo.
func (r todoReader) item(node ast.Node) (int, []string) {
	lead := node.FirstChild()
	if lead == nil || lead.Type() != ast.TypeBlock || lead.Lines().Len() == 0 {
		return 0, nil
	}
	line := r.lineAt(lead.Lines().At(0).Start)
	return line, todoLine.FindStringSubmatch(r.lines[line-1])
}

// body is every source line under an item that could hold a field: not its
// own, not inside a code block, and not in a nested item that is a todo itself.
func (r todoReader) body(item ast.Node, own int) []int {
	seen := map[int]bool{own: true}
	var out []int
	_ = ast.Walk(item, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || child.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		if child.Kind() == ast.KindFencedCodeBlock || child.Kind() == ast.KindCodeBlock {
			return ast.WalkSkipChildren, nil
		}
		if child != item && child.Kind() == ast.KindListItem {
			if _, found := r.item(child); found != nil {
				return ast.WalkSkipChildren, nil
			}
		}
		segments := child.Lines()
		for i := range segments.Len() {
			if line := r.lineAt(segments.At(i).Start); !seen[line] {
				seen[line] = true
				out = append(out, line)
			}
		}
		return ast.WalkContinue, nil
	})
	sort.Ints(out)
	return out
}

// plainHeading is a heading's text without its markup, as the pane names a section.
func plainHeading(heading *ast.Heading, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(heading, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch inline := child.(type) {
		case *ast.Text:
			b.Write(inline.Segment.Value(source))
		case *ast.String:
			b.Write(inline.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}
