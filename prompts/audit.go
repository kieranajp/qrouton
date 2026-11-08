package prompts

import (
	"fmt"
	"slices"
	"strings"
)

type AuditTell struct {
	Slug, Heading, Legacy, Caption string
	LegacyDefault                  bool
}

func AuditTells() []AuditTell { return slices.Clone(auditTellMetadata) }

func auditTellSlugs() string {
	var slugs []string
	for _, tell := range auditTellMetadata {
		slugs = append(slugs, tell.Slug)
	}
	return strings.Join(slugs, ", ")
}

func auditTellSections() string {
	var sections []string
	for _, tell := range auditTellMetadata {
		sections = append(sections, fmt.Sprintf("## %s\n\n> <%s>", tell.Heading, tell.Caption))
	}
	return strings.Join(sections, "\n\n")
}

func auditTellTable() string {
	rows := []string{"| Slug | Section |", "| --- | --- |"}
	for _, tell := range auditTellMetadata {
		heading := fmt.Sprintf("`## %s`", tell.Heading)
		if tell.Legacy != "" {
			heading += fmt.Sprintf("; an older audit's `## %s` still reads as this tell", tell.Legacy)
		}
		rows = append(rows, fmt.Sprintf("| `%s` | %s |", tell.Slug, heading))
	}
	return strings.Join(rows, "\n")
}
