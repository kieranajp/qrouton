package markdown

import (
	"slices"
	"testing"
)

func TestTitleIsTheHeadingTheDocumentOpensWith(t *testing.T) {
	for _, tc := range []struct {
		name  string
		text  string
		want  string
		found bool
	}{
		{"plain heading", "# Retry backoff\n\nbody\n", "Retry backoff", true},
		{"leading blank lines", "\n\n# Retry backoff\n", "Retry backoff", true},
		{"padded heading", "   #    Retry backoff   \n", "Retry backoff", true},
		{"heading only", "# Retry backoff", "Retry backoff", true},

		{"behind frontmatter", "---\ntitle: not this one\n---\n\n# Retry backoff\n", "Retry backoff", true},
		{"frontmatter after blank lines", "\n---\na: 1\n---\n# Retry backoff\n", "Retry backoff", true},
		{"frontmatter is the whole file", "---\na: 1\n---\n", "", false},
		// An unterminated block is frontmatter all the way down, so its contents
		// are not the document.
		{"unclosed frontmatter", "---\n# Retry backoff\n", "", false},

		{"prose first", "No heading here, just prose.\n\n# Later\n", "", false},
		{"second-level heading", "## Only a second-level heading\n", "", false},
		{"empty", "", "", false},
		{"blank lines only", "\n\n\n", "", false},

		// The reason for opening-heading-only: a `# ` further down belongs to a
		// later section or to the inside of a fenced block, and neither names
		// the document.
		{"heading inside a code fence", "```sh\n# not a title, a shell comment\n```\n", "", false},
		{"a later section", "Intro prose.\n\n# Section two\n", "", false},

		{"not a heading without the space", "#Retry backoff\n", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Title(tc.text)
			if got != tc.want || ok != tc.found {
				t.Fatalf("Title(%q) = %q, %v; want %q, %v", tc.text, got, ok, tc.want, tc.found)
			}
		})
	}
}

func TestSectionsSeparatesFramingFromWhatWasWritten(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want []Section
	}{
		{"nothing to open", "# Title\n\nJust prose.\n", nil},
		{"a section with prose", "## How does it retry?\n\nThree times.\n", []Section{{"How does it retry?", true}}},
		{"a heading with nothing under it", "## Open Questions\n", []Section{{"Open Questions", false}}},
		{"framing is not an answer", "## How?\n\n> Start in retry.go.\n", []Section{{"How?", false}}},
		{
			"an answer beside its framing",
			"## How?\n\n> Start in retry.go.\n\nIt doubles the wait.\n",
			[]Section{{"How?", true}},
		},
		{
			"a fenced heading opens nothing and answers its own section",
			"## How?\n\n```go\n## not a heading\n```\n",
			[]Section{{"How?", true}},
		},
		{
			"frontmatter naming a section opens nothing",
			"---\n## not a section\n---\n\n## Real\n\nBody.\n",
			[]Section{{"Real", true}},
		},
		{"prose above the first heading belongs to no section", "Lead prose.\n\n## First\n", []Section{{"First", false}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Sections(tc.text)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Sections(%q) = %#v; want %#v", tc.text, got, tc.want)
			}
		})
	}
}
