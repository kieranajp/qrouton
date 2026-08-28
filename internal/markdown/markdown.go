// Package markdown answers the one question qrouton asks of a document's text:
// what does it call itself. A tab label and a shared page's title are the same
// question, so they are the same answer. Nothing here renders anything — the
// workbench's page and the share bundle each do their own.
package markdown

import "strings"

// fence opens and closes a frontmatter block.
const fence = "---"

// heading marks the level-one heading a document titles itself with.
const heading = "# "

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
