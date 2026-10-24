package markdown

import "strings"

// summaryHeading opens a research document with what was known at framing
// time, so it answers no question.
const summaryHeading = "Summary"

// Researched reports whether some question is answered and none holds only its framing.
// Only an opening Summary is exempt from counting as a question.
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
