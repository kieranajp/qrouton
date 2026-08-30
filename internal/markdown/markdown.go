// Package markdown answers what qrouton asks of a document's text: what it
// calls itself, and which of its sections have been written. A tab label and a
// shared page's title are the same question, so they are the same answer.
// Nothing here renders anything — the workbench's page and the share bundle
// each do their own.
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

// Title is the level-one heading a document opens with, after any frontmatter,
// and false for a document that opens with anything else — prose, a lower
// heading, a code fence. Deliberately the opening heading rather than the first
// one anywhere: a `# ` further down belongs to a later section or to the inside
// of a fenced block, and neither names the document.
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

// body is text's lines with any frontmatter block dropped, which is parsed but
// never shown. A block that never closes leaves no body at all rather than
// treating its own contents as one.
func body(text string) []string {
	lines := strings.Split(text, "\n")
	for at, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed != fence {
			return lines[at:]
		}
		for closing := at + 1; closing < len(lines); closing++ {
			if strings.TrimSpace(lines[closing]) == fence {
				return lines[closing+1:]
			}
		}
		return nil
	}
	return nil
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
