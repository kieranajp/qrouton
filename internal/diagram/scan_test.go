package diagram

import (
	"reflect"
	"testing"
)

func TestScanFindsTopLevelFencesOnly(t *testing.T) {
	cases := map[string]struct {
		document string
		want     []Fence
	}{
		"a plain fence": {
			document: "# Doc\n\n```d2\na -> b\n```\n",
			want:     []Fence{{Line: 3, EndLine: 5, Source: "a -> b"}},
		},
		"a tilde fence opens a code block too": {
			document: "~~~d2\na -> b\n~~~\n",
			want:     []Fence{{Line: 1, EndLine: 3, Source: "a -> b"}},
		},
		"a tilde fence is not closed by backticks": {
			document: "~~~d2\na -> b\n```\n~~~\n",
			want:     []Fence{{Line: 1, EndLine: 4, Source: "a -> b\n```"}},
		},
		"four spaces of indent is code, not a fence": {
			document: "    ```d2\n    a -> b\n    ```\n",
		},
		"a tab of indent is code too": {
			document: "\t```d2\n\ta -> b\n\t```\n",
		},
		"two spaces and a tab reach the fourth column": {
			document: "  \t```d2\n  \ta -> b\n  \t```\n",
		},
		"three spaces of indent is still a fence": {
			document: "   ```d2\n   a -> b\n   ```\n",
			want:     []Fence{{Line: 1, EndLine: 3, Source: "a -> b"}},
		},
		"a fence inside a longer fence is quoted, not a diagram": {
			document: "````markdown\n```d2\na -> b\n```\n````\n",
		},
		"a fence in a list is still top level": {
			document: "- item\n\n  ```d2\n  a -> b\n  ```\n",
			want:     []Fence{{Line: 3, EndLine: 5, Source: "a -> b"}},
		},
		"a blockquote marker stops a line looking like a fence": {
			document: "> ```d2\n> a -> b\n> ```\n",
		},
		"an unterminated fence runs to the end of the document": {
			document: "text\n\n```d2\na -> b\n",
			want:     []Fence{{Line: 3, EndLine: 5, Source: "a -> b\n"}},
		},
		"the info string may carry more than the language": {
			document: "```d2 title=x\na -> b\n```\n",
			want:     []Fence{{Line: 1, EndLine: 3, Source: "a -> b"}},
		},
		"the language is matched exactly": {
			document: "```D2\na -> b\n```\n\n```d2x\na -> b\n```\n",
		},
		"a closing fence must be at least as long as its opener": {
			document: "```d2\na -> b\n``\n```\n",
			want:     []Fence{{Line: 1, EndLine: 4, Source: "a -> b\n``"}},
		},
		"a closing fence carries no info string": {
			document: "```d2\na -> b\n``` d2\n```\n",
			want:     []Fence{{Line: 1, EndLine: 4, Source: "a -> b\n``` d2"}},
		},
		"several fences are found in order": {
			document: "```d2\na\n```\n\n```go\nb\n```\n\n```d2\nc\n```\n",
			want: []Fence{
				{Line: 1, EndLine: 3, Source: "a"},
				{Line: 9, EndLine: 11, Source: "c"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := Scan(tc.document)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Scan() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
