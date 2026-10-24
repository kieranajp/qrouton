package markdown

import (
	"encoding/json"
	"os"
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
		{"a tilde fence hides its heading", "## How?\n\n~~~\n## not a heading\n~~~\n", []Section{{"How?", SectionAnswered}}},
		{
			"a shorter fence does not close a longer one",
			"## How?\n\n````md\n```\n## not a heading\n```\n````\n",
			[]Section{{"How?", SectionAnswered}},
		},
		{"an indented heading still opens a section", "   ## How?\n\n> Start here.\n", []Section{{"How?", SectionFramed}}},
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
		{"spaced before the colon", "---\nmarp :  true\n---\n", true},
		{"trailing whitespace", "---\nmarp: true  \r\n---\n", true},
		{"no space after the colon", "---\nmarp:true\n---\n", false},
		{"uppercase", "---\nmarp: True\n---\n", false},
		{"YAML 1.1 yes", "---\nmarp: yes\n---\n", false},
		{"YAML 1.1 on", "---\nmarp: on\n---\n", false},
		{"quoted true", "---\nmarp: \"true\"\n---\n", false},
		{"quoted key", "---\n\"marp\": true\n---\n", false},
		{"nested under another key", "---\nstyle:\n  marp: true\n---\n", false},
		{"inside a block scalar", "---\nnotes: |\n  marp: true\n---\n", false},
		{"the first marp line decides", "---\nmarp: false\nmarp: true\n---\n", false},
		{"declined", "---\nmarp: false\n---\n", false},
		{"another value", "---\nmarp: maybe\n---\n", false},
		{"beside invalid YAML", "---\nmarp: true\nfooter: Lifesum: Q3\ntitle: [unclosed\n---\n", true},
		{"quoted true beside invalid YAML", "---\nmarp: \"true\"\nfooter: Lifesum: Q3\n---\n", false},
		{"beside a quoted colon", "---\ntitle: \"Deck: one\"\nmarp: true\n---\n", true},
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

// The plan pane parses the same cases, so the stage mark and the pane's meter
// cannot disagree about a plan.
func TestPlanPhasesMatchesTheSharedCases(t *testing.T) {
	body, err := os.ReadFile("testdata/plan-phases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string `json:"name"`
		Text   string `json:"text"`
		Phases []struct {
			Number int    `json:"number"`
			Label  string `json:"label"`
			Met    int    `json:"met"`
			Total  int    `json:"total"`
		} `json:"phases"`
	}
	if err := json.Unmarshal(body, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		got := PlanPhases(tc.Text)
		if len(got) != len(tc.Phases) {
			t.Errorf("%s: %d phases, want %d: %+v", tc.Name, len(got), len(tc.Phases), got)
			continue
		}
		for i, want := range tc.Phases {
			if got[i].Number != want.Number || got[i].Label != want.Label || got[i].Met != want.Met || got[i].Total != want.Total {
				t.Errorf("%s: phase %d = %+v, want %+v", tc.Name, i, got[i], want)
			}
		}
	}
}

func TestResearchedSkipsOnlyAnOpeningSummary(t *testing.T) {
	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{"an answered question", "## Summary\n\nFraming.\n\n## How?\n\nLike this.\n", true},
		{"only the opening summary answered", "## Summary\n\nFraming.\n\n## How?\n", false},
		{"a late summary is a question like any other", "## How?\n\n## Summary\n\nFindings.\n", true},
		{"a framed question is outstanding", "## Summary\n\nFraming.\n\n## How?\n\n> Look here.\n\n## Why?\n\nBecause.\n", false},
		{"no sections", "Just prose.\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Researched(tc.text); got != tc.want {
				t.Fatalf("Researched(%q) = %v; want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestHeadingsKeepTheirSourceLines(t *testing.T) {
	body := "# Guide\n\nIntro.\n\n## Setup ##\n\n~~~sh\n# not a heading\n~~~\n\n> # quoted\n\n### Linux\n"
	want := []Heading{{1, 1, "Guide"}, {5, 2, "Setup"}, {13, 3, "Linux"}}
	if got := Headings(body); !slices.Equal(got, want) {
		t.Fatalf("Headings = %+v; want %+v", got, want)
	}
}

func TestSplitGivesTheLineTheBodyStartsOn(t *testing.T) {
	for _, tc := range []struct {
		name, text, front, body string
		line                    int
	}{
		{"no frontmatter", "\n# T\n", "", "\n# T\n", 1},
		{"frontmatter", "---\nkind: note\n---\n\n# T\n", "kind: note", "\n# T\n", 4},
		{"after blank lines", "\n---\na: 1\n---\n# T\n", "a: 1", "# T\n", 5},
		{"unclosed", "---\na: 1\n# T\n", "", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			front, body, line := Split(tc.text)
			if front != tc.front || body != tc.body || line != tc.line {
				t.Fatalf("Split(%q) = %q, %q, %d", tc.text, front, body, line)
			}
		})
	}
}
