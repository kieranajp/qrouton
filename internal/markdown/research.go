package markdown

import "strings"

// summaryHeading opens a research document with what was known at framing
// time, so it answers no question.
const summaryHeading = "Summary"

// Researched reports whether a research document has an answered question and
// none left holding only its framing. An opening Summary answers nothing; one
// further down is a question like any other.
func Researched(text string) bool {
	answered := false
	for at, section := range Sections(text) {
		switch {
		case section.State == SectionFramed:
			return false
		case section.State == SectionAnswered && (at > 0 || !strings.EqualFold(section.Name, summaryHeading)):
			answered = true
		}
	}
	return answered
}
