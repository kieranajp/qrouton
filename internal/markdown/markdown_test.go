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
		{"a section with prose", "## How does it retry?\n\nThree times.\n", []Section{{"How does it retry?", SectionAnswered}}},
		{"a heading with nothing under it", "## Open Questions\n", []Section{{"Open Questions", SectionEmpty}}},
		{"framing is not an answer", "## How?\n\n> Start in retry.go.\n", []Section{{"How?", SectionFramed}}},
		{
			"an answer beside its framing",
			"## How?\n\n> Start in retry.go.\n\nIt doubles the wait.\n",
			[]Section{{"How?", SectionAnswered}},
		},
		{
			"a fenced heading opens nothing and answers its own section",
			"## How?\n\n```go\n## not a heading\n```\n",
			[]Section{{"How?", SectionAnswered}},
		},
		{
			"frontmatter naming a section opens nothing",
			"---\n## not a section\n---\n\n## Real\n\nBody.\n",
			[]Section{{"Real", SectionAnswered}},
		},
		{"unclosed frontmatter leaves no document to read", "---\n## Real\n\nBody.\n", nil},
		{"empty", "", nil},
		{"prose above the first heading belongs to no section", "Lead prose.\n\n## First\n", []Section{{"First", SectionEmpty}}},
		{
			"each section is judged on its own",
			"## One\n\nAnswered.\n\n## Two\n\n> Framed.\n\n## Three\n",
			[]Section{{"One", SectionAnswered}, {"Two", SectionFramed}, {"Three", SectionEmpty}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Sections(tc.text)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Sections(%q) = %#v; want %#v", tc.text, got, tc.want)
			}
		})
	}
}

func TestBodyIsWhatIsLeftAfterFrontmatter(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"no frontmatter", "# Retry\n\nbody\n", "# Retry\n\nbody\n"},
		{"behind frontmatter", "---\na: 1\n---\n# Retry\n", "# Retry\n"},
		{"frontmatter is the whole file", "---\na: 1\n---\n", ""},
		{"unclosed frontmatter", "---\n# Retry\n", ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Body(tc.text); got != tc.want {
				t.Fatalf("Body(%q) = %q; want %q", tc.text, got, tc.want)
			}
		})
	}
}

func TestMarpIsDeclaredInFrontmatter(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{"no frontmatter", "# Deck\n\nmarp: true\n", false},
		{"declared", "---\nmarp: true\ntheme: qrouton\n---\n\n# Deck\n", true},
		{"declared after other keys", "---\ntitle: x\nmarp: true\n---\n", true},
		{"declared uppercase", "---\nmarp: True\n---\n", true},
		{"padded", "---\n  marp :  true  \n---\n", true},
		{"declined", "---\nmarp: false\n---\n", false},
		{"another value", "---\nmarp: maybe\n---\n", false},
		{"absent", "---\ntheme: qrouton\n---\n", false},
		{"in the body, not the frontmatter", "---\na: 1\n---\n\nmarp: true\n", false},
		{"unclosed frontmatter", "---\nmarp: true\n", false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Marp(tc.text); got != tc.want {
				t.Fatalf("Marp(%q) = %v; want %v", tc.text, got, tc.want)
			}
		})
	}
}
