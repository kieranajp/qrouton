// Package markdown reads titles, bodies, and section state without rendering.
package markdown

import "strings"

// fence opens and closes a frontmatter block.
const fence = "---"

// heading names the document and subheading opens a section; a code fence
// suspends subheading, and a blockquote under one is framing rather than an
// answer.
const (
	heading    = "# "
	subheading = "## "
	codeFence  = "```"
	quote      = ">"
)

// marpKey declares a document a deck, and only when set to marpEnabled.
const (
	marpKey      = "marp"
	marpEnabled  = "true"
	keySeparator = ":"
)

// Title accepts only the first visible element after frontmatter as the document title.
func Title(text string) (string, bool) {
	for _, line := range body(text) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if title, ok := strings.CutPrefix(trimmed, heading); ok {
			return strings.TrimSpace(title), true
		}
		return "", false
	}
	return "", false
}

// Body is the document with any frontmatter dropped: what a reader would see.
// Empty for a document that is nothing but frontmatter, closed or not.
func Body(text string) string {
	return strings.Join(body(text), "\n")
}

// Marp reports whether a document declares itself a deck. Only the leading
// frontmatter block counts, so a marp: line in the body is prose.
func Marp(text string) bool {
	for _, line := range frontmatter(text) {
		key, value, ok := strings.Cut(line, keySeparator)
		if !ok || strings.TrimSpace(key) != marpKey {
			continue
		}
		return strings.EqualFold(strings.TrimSpace(value), marpEnabled)
	}
	return false
}

// body is text's lines with any frontmatter block dropped, which is parsed but
// never shown.
func body(text string) []string {
	_, rest := split(text)
	return rest
}

func frontmatter(text string) []string {
	front, _ := split(text)
	return front
}

// split separates the frontmatter block from what a reader would see. A block
// that never closes is neither: its contents are not the document, and a block
// with no end declares nothing either.
func split(text string) (front, rest []string) {
	lines := strings.Split(text, "\n")
	for at, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed != fence {
			return nil, lines[at:]
		}
		for closing := at + 1; closing < len(lines); closing++ {
			if strings.TrimSpace(lines[closing]) == fence {
				return lines[at+1 : closing], lines[closing+1:]
			}
		}
		return nil, nil
	}
	return nil, nil
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

// Sections reads a document's second-level headings in order. A heading inside
// a fenced block opens nothing, and text above the first heading belongs to no
// section.
func Sections(text string) []Section {
	var sections []Section
	fenced := false
	for _, line := range body(text) {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, codeFence) {
			fenced = !fenced
		} else if !fenced {
			if name, ok := strings.CutPrefix(line, subheading); ok {
				sections = append(sections, Section{Name: strings.TrimSpace(name)})
				continue
			}
		}
		if len(sections) == 0 || trimmed == "" {
			continue
		}
		section := &sections[len(sections)-1]
		if strings.HasPrefix(trimmed, quote) {
			if section.State == SectionEmpty {
				section.State = SectionFramed
			}
			continue
		}
		section.State = SectionAnswered
	}
	return sections
}
