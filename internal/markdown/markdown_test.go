package markdown

import "testing"

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
