package commentdiscipline

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const guidance = "See AGENTS.md: comments default to none, state what IS, and stay one line where earned or two for a real trap."

var (
	goDirective = regexp.MustCompile(`(?i)^\s*(//|/\*+)?\s*(go:|\+build\b|line\b|nolint\b|lint:|revive:|gosec\b)`)
	urlSpan     = regexp.MustCompile(`https?://[^\s<>()]+`)
)

type Diagnostic struct {
	Path    string
	Line    int
	Column  int
	Rule    string
	Message string
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", d.Path, d.Line, d.Column, d.Rule, d.Message)
}

func CheckGoSource(path string, source []byte, policy Policy) ([]Diagnostic, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	if ast.IsGenerated(file) {
		return nil, nil
	}
	lines := bytes.Split(source, []byte("\n"))
	pointer := pathPointer(policy.PathExtensions)
	var diagnostics []Diagnostic
	var run []*ast.Comment

	flush := func() {
		if len(run) == 0 {
			return
		}
		first := fset.Position(run[0].Slash)
		last := fset.Position(run[len(run)-1].End())
		height := last.Line - first.Line + 1
		if height > policy.MaxCommentRun {
			diagnostics = append(diagnostics, Diagnostic{
				Path: path, Line: first.Line, Column: first.Column,
				Rule:    "comment-discipline/max-comment-run",
				Message: fmt.Sprintf("comment runs %d lines; the cap is %d. Say the one thing code cannot, or delete it. %s", height, policy.MaxCommentRun, guidance),
			})
		}
		run = nil
	}

	for _, group := range file.Comments {
		for _, comment := range group.List {
			start := fset.Position(comment.Slash)
			directive := isGoDirective(comment.Text)
			if !directive {
				body := normalizeComment(comment.Text)
				lower := strings.ToLower(body)
				for _, phrase := range policy.NarrationPhrases {
					if strings.Contains(lower, phrase) {
						diagnostics = append(diagnostics, Diagnostic{
							Path: path, Line: start.Line, Column: start.Column,
							Rule:    "comment-discipline/no-narration",
							Message: fmt.Sprintf("comment narration contains %q. State what IS, not how the code got here. %s", phrase, guidance),
						})
						break
					}
				}
				withoutURLs := urlSpan.ReplaceAllString(body, "")
				if pointer.MatchString(withoutURLs) {
					diagnostics = append(diagnostics, Diagnostic{
						Path: path, Line: start.Line, Column: start.Column,
						Rule:    "comment-discipline/no-path-pointer",
						Message: "file and line pointers go stale. Name the subject, not its location. " + guidance,
					})
				}
			}

			if directive || !ownsLine(lines, start) {
				flush()
				continue
			}
			if len(run) > 0 {
				previousEnd := fset.Position(run[len(run)-1].End()).Line
				if start.Line != previousEnd+1 {
					flush()
				}
			}
			run = append(run, comment)
		}
	}
	flush()
	sortDiagnostics(diagnostics)
	return diagnostics, nil
}

func CheckGoTree(root string, policy Policy) ([]Diagnostic, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var diagnostics []Diagnostic
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if path != root && (entry.Name() == ".git" || entry.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found, err := CheckGoSource(filepath.ToSlash(relative), source, policy)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.ToSlash(relative), err)
		}
		diagnostics = append(diagnostics, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortDiagnostics(diagnostics)
	return diagnostics, nil
}

func isGoDirective(text string) bool {
	return goDirective.MatchString(text)
}

func normalizeComment(text string) string {
	text = strings.TrimPrefix(text, "//")
	text = strings.TrimPrefix(text, "/*")
	text = strings.TrimSuffix(text, "*/")
	lines := strings.Split(text, "\n")
	for index := range lines {
		lines[index] = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[index]), "*"))
	}
	return strings.Join(lines, " ")
}

func ownsLine(lines [][]byte, position token.Position) bool {
	if position.Line < 1 || position.Line > len(lines) {
		return false
	}
	prefix := lines[position.Line-1]
	column := position.Column - 1
	if column < 0 || column > len(prefix) {
		return false
	}
	return len(bytes.TrimSpace(prefix[:column])) == 0
}

func pathPointer(extensions []string) *regexp.Regexp {
	escaped := make([]string, len(extensions))
	for index, extension := range extensions {
		escaped[index] = regexp.QuoteMeta(extension)
	}
	suffix := `(?:` + strings.Join(escaped, "|") + `)`
	return regexp.MustCompile(`(?i)(?:^|[\s(` + "`" + `'\"])(?:[\w.-]+/)+[\w.-]+\.` + suffix + `\b|\b[\w.-]+\.` + suffix + `:\d+\b`)
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i], diagnostics[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		return left.Rule < right.Rule
	})
}
