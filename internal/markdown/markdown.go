// Package markdown reads titles, bodies, and section state without rendering.
package markdown

import (
	"regexp"
	"strconv"
	"strings"
)

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

// Phase is one `## Phase <n> — <name>` section of a plan, counted by the task
// items under its `### Verify` heading.
type Phase struct {
	Number int
	Name   string
	Met    int
	Total  int
}

var (
	phaseHeading = regexp.MustCompile(`^Phase\s+(\d+)\s*[—–:-]\s*(\S.*?)\s*$`)
	taskItem     = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+\[([^\]])\]\s`)
	atxHeading   = regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.*?)\s*#*\s*$`)
)

const verifyHeading = "verify"

// PlanPhases reads a plan's phases the way the plan pane does. A task item
// with any marker but x counts as unmet, and items under any other heading,
// `### See` included, count for nothing.
func PlanPhases(text string) []Phase {
	var phases []Phase
	var phase *Phase
	verifying, verified, fenced := false, false, ""
	for _, line := range body(text) {
		trimmed := strings.TrimSpace(line)
		if marker := fenceMarker(trimmed); marker != "" {
			switch {
			case fenced == "":
				fenced = marker
				continue
			case marker[0] == fenced[0] && len(marker) >= len(fenced) && strings.TrimLeft(trimmed, marker[:1]) == "":
				fenced = ""
				continue
			}
		}
		if fenced != "" {
			continue
		}
		if match := atxHeading.FindStringSubmatch(line); match != nil {
			depth, name := len(match[1]), strings.TrimSpace(match[2])
			if verifying && depth <= 3 {
				verifying, verified = false, true
			}
			if depth <= 2 {
				phase, verified = nil, false
				if found := phaseHeading.FindStringSubmatch(name); depth == 2 && found != nil {
					number, _ := strconv.Atoi(found[1])
					phases = append(phases, Phase{Number: number, Name: found[2]})
					phase = &phases[len(phases)-1]
				}
			} else if depth == 3 && !verified {
				verifying = phase != nil && strings.EqualFold(name, verifyHeading)
			}
			continue
		}
		if !verifying {
			continue
		}
		if match := taskItem.FindStringSubmatch(line); match != nil {
			phase.Total++
			if match[1] == "x" || match[1] == "X" {
				phase.Met++
			}
		}
	}
	return phases
}

// fenceMarker is the run of backticks or tildes a fence line opens with.
func fenceMarker(trimmed string) string {
	for _, char := range []string{"`", "~"} {
		run := len(trimmed) - len(strings.TrimLeft(trimmed, char))
		if run >= 3 {
			return trimmed[:run]
		}
	}
	return ""
}
