// Package diagram renders the d2 fences of a markdown document to inline SVG,
// repainted in qrouton's palette and vetted before it reaches a webview.
package diagram

import "strings"

// Fence is one d2 code block: the 1-based document lines its opening and
// closing markers sit on, and the source between them.
type Fence struct {
	Line    int
	EndLine int
	Source  string
}

// Scan finds the d2 fences in raw markdown. Only top-level ones count — under
// four spaces of indent, outside any other open fence, and outside a
// blockquote, whose marker stops the line looking like a fence at all. That is
// the set whose mdast line numbers the page stamps onto its rendered blocks,
// and both directions of error are safe: a fence found here that the page never
// stamped is dropped, and one missed stays code.
func Scan(document string) []Fence {
	lines := strings.Split(document, "\n")
	var found []Fence
	for at := 0; at < len(lines); at++ {
		open, ok := opens(lines[at])
		if !ok {
			continue
		}
		source, end := block(lines, at, open)
		if open.info == fenceInfo {
			found = append(found, Fence{Line: at + 1, EndLine: end + 1, Source: source})
		}
		at = end
	}
	return found
}

type fence struct {
	char   byte
	length int
	indent int
	info   string
}

// opens reports the fence a line starts, for any info string. Fences qrouton
// does not render still have to be tracked, or a d2 block quoted inside one
// would be picked up as a diagram.
func opens(line string) (fence, bool) {
	indent, rest := split(line)
	if indent >= maxFenceIndent || rest == "" {
		return fence{}, false
	}
	char := rest[0]
	if char != fenceBacktick && char != fenceTilde {
		return fence{}, false
	}
	length := run(rest, char)
	if length < minFenceLength {
		return fence{}, false
	}
	info := strings.TrimSpace(rest[length:])
	// CommonMark: a backtick fence's info string may not contain a backtick.
	if char == fenceBacktick && strings.ContainsRune(info, fenceBacktick) {
		return fence{}, false
	}
	return fence{char: char, length: length, indent: indent, info: word(info)}, true
}

func closes(line string, open fence) bool {
	indent, rest := split(line)
	if indent >= maxFenceIndent {
		return false
	}
	length := run(rest, open.char)
	return length >= open.length && strings.TrimSpace(rest[length:]) == ""
}

// block returns the source and the index of the line that closed it. A fence
// nothing closes runs to the end of the document, as it does in a parser.
func block(lines []string, at int, open fence) (string, int) {
	var source []string
	for cursor := at + 1; cursor < len(lines); cursor++ {
		if closes(lines[cursor], open) {
			return strings.Join(source, "\n"), cursor
		}
		source = append(source, undent(lines[cursor], open.indent))
	}
	return strings.Join(source, "\n"), len(lines) - 1
}

func split(line string) (int, string) {
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	return indent, line[indent:]
}

func run(text string, char byte) int {
	length := 0
	for length < len(text) && text[length] == char {
		length++
	}
	return length
}

func undent(line string, indent int) string {
	stripped := 0
	for stripped < indent && stripped < len(line) && line[stripped] == ' ' {
		stripped++
	}
	return line[stripped:]
}

func word(info string) string {
	fields := strings.Fields(info)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
