// Package markdown reads titles, bodies, and section state without rendering.
package markdown

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// fence opens and closes a frontmatter block.
const fence = "---"

var parser = goldmark.New(goldmark.WithExtensions(extension.TaskList)).Parser()

// Title accepts only the first visible element after frontmatter as the document title.
func Title(text string) (string, bool) {
	source := []byte(Body(text))
	heading, ok := parse(source).FirstChild().(*ast.Heading)
	if !ok || heading.Level != 1 {
		return "", false
	}
	title := headingText(heading, source)
	return title, title != ""
}

// Body is the document with any frontmatter dropped: what a reader would see.
// Empty for a document that is nothing but frontmatter, closed or not.
func Body(text string) string {
	_, body, _ := Split(text)
	return body
}

// Marp reports whether a document declares itself a deck. Only the leading
// frontmatter block counts, so a marp: line in the body is prose.
func Marp(text string) bool {
	front, _, _ := Split(text)
	directive := marpDirective.FindStringSubmatch(front)
	return directive != nil && strings.TrimRight(directive[1], " \t\r") == marpEnabled
}

// marpDirective is Marp's own deck test, so qrouton and Marp agree on what a
// deck is: the first unindented marp line, its value compared as text.
var marpDirective = regexp.MustCompile(`(?m)^marp\s*: +(.*)$`)

const marpEnabled = "true"

// Split returns the frontmatter, the body and the body's first source line.
// An unclosed block yields nothing at all.
func Split(text string) (front, body string, line int) {
	lines := strings.Split(text, "\n")
	for at, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if trimmed != fence {
			break
		}
		for closing := at + 1; closing < len(lines); closing++ {
			if strings.TrimSpace(lines[closing]) == fence {
				return strings.Join(lines[at+1:closing], "\n"), strings.Join(lines[closing+1:], "\n"), closing + 2
			}
		}
		return "", "", 0
	}
	return "", text, 1
}

// Heading is one top-level heading of a body. Line counts from the body's
// first line.
type Heading struct {
	Line  int
	Level int
	Text  string
}

// Headings reads a body's headings in order, leaving out any inside a fence,
// a quote or a list. The body must already be clear of frontmatter.
func Headings(body string) []Heading {
	source := []byte(body)
	var headings []Heading
	for node := parse(source).FirstChild(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*ast.Heading); ok {
			line := bytes.Count(source[:max(heading.Pos(), 0)], []byte("\n")) + 1
			headings = append(headings, Heading{Line: line, Level: heading.Level, Text: headingText(heading, source)})
		}
	}
	return headings
}

func parse(source []byte) ast.Node {
	return parser.Parse(text.NewReader(source))
}

func headingText(heading *ast.Heading, source []byte) string {
	lines := heading.Lines()
	parts := make([]string, 0, lines.Len())
	for i := range lines.Len() {
		segment := lines.At(i)
		parts = append(parts, strings.TrimSpace(string(segment.Value(source))))

	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// section opens at a named second-level heading and holds the blocks up to the
// next one.
func section(node ast.Node, source []byte) (string, bool) {
	heading, ok := node.(*ast.Heading)
	if !ok || heading.Level != 2 {
		return "", false
	}
	name := headingText(heading, source)
	return name, name != ""
}

// SectionState is how far a section has been taken. A research document is
// framed before it is answered: its questions are written as headings with the
// context a researcher needs in a blockquote, and answering replaces that
// blockquote with the finding.
type SectionState int

const (
	SectionEmpty SectionState = iota
	SectionFramed
	SectionAnswered
)

type Section struct {
	Name  string
	State SectionState
}

// Sections reads a document's second-level headings in order. Text above the
// first heading belongs to no section.
func Sections(text string) []Section {
	source := []byte(Body(text))
	var sections []Section
	for node := parse(source).FirstChild(); node != nil; node = node.NextSibling() {
		if name, ok := section(node, source); ok {
			sections = append(sections, Section{Name: name})
			continue
		}
		if len(sections) == 0 {
			continue
		}
		current := &sections[len(sections)-1]
		if node.Kind() != ast.KindBlockquote {
			current.State = SectionAnswered
		} else if current.State == SectionEmpty {
			current.State = SectionFramed
		}
	}
	return sections
}

// Phase is one `## Phase <n> — <name>` section of a plan, counted by the task
// items under its `### Verify` heading. A phase inserted mid-flight carries one
// lowercase letter, `## Phase 5b — <name>`: Number is 5 and Label is "5b".
type Phase struct {
	Number int
	Label  string
	Name   string
	Met    int
	Total  int
}

var (
	phaseHeading = regexp.MustCompile(`^Phase\s+(\d+)([a-z])?\s*[—–:-]\s*(\S.*?)\s*$`)
	otherMarker  = regexp.MustCompile(`^\[[^\]\s]\]\s`)
	boxedText    = regexp.MustCompile(`^\[.\][ \t]+\S`)
)

const verifyHeading = "verify"

// PlanPhases reads a plan's phases the way the plan pane does. Only the first
// `### Verify` of a phase counts, up to the next heading at its depth or above.
func PlanPhases(text string) []Phase {
	source := []byte(Body(text))
	var phases []Phase
	var phase *Phase
	verifying, verified := false, false
	for node := parse(source).FirstChild(); node != nil; node = node.NextSibling() {
		if name, ok := section(node, source); ok {
			phase, verifying, verified = nil, false, false
			if found := phaseHeading.FindStringSubmatch(name); found != nil {
				number, _ := strconv.Atoi(found[1])
				phases = append(phases, Phase{Number: number, Label: found[1] + found[2], Name: found[3]})
				phase = &phases[len(phases)-1]
			}
			continue
		}
		if heading, ok := node.(*ast.Heading); ok {
			if verifying && heading.Level <= 3 {
				verifying, verified = false, true
			} else if phase != nil && !verifying && !verified && heading.Level == 3 {
				verifying = strings.EqualFold(headingText(heading, source), verifyHeading)
			}
			continue
		}
		if verifying {
			countCriteria(node, source, phase)
		}
	}
	return phases
}

// An item boxed with a marker other than space or x parses as prose but is still an unmet criterion.
func countCriteria(node ast.Node, source []byte, phase *Phase) {
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || child.Kind() != ast.KindListItem || child.FirstChild() == nil {
			return ast.WalkContinue, nil
		}
		lead := child.FirstChild()
		lines := lead.Lines()
		if lines == nil || lines.Len() == 0 {
			return ast.WalkContinue, nil
		}
		segment := lines.At(0)
		first := segment.Value(source)
		if box, ok := lead.FirstChild().(*extast.TaskCheckBox); ok {
			if boxedText.Match(first) {
				phase.Total++
				if box.IsChecked {
					phase.Met++
				}
			}
		} else if otherMarker.Match(first) {
			phase.Total++
		}
		return ast.WalkContinue, nil
	})
}
