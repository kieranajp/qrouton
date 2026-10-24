package vault

import "testing"

func TestDocumentsTakeTheirIdentityFromThePath(t *testing.T) {
	for _, tc := range []struct {
		name, text, kind, title string
	}{
		{"notes/a.md", "# Heading title\n\nText\n", "", "Heading title"},
		{"notes/e.md", "Prose first.\n\n# Later heading\n", "", "e"},
		{"notes/f.md", "\n---\ntitle: After blanks\n---\n# F\n", "", "After blanks"},
		{"notes/b.md", "---\nkind: plan\ntitle: From frontmatter\n---\nText\n", "plan", "From frontmatter"},
		{"notes/c.md", "---\nkind: whatever\n---\nno heading\n", "", "c"},
		{"notes/d.md", "---\nbroken: [yaml\n---\n# D\n", "", "D"},
		{"s/shared/research/R1-x.md", "# Old research\n", "research", "Old research"},
		{"s/shared/plans/P1-x.md", "---\nkind: note\n---\n# Relabelled\n", "note", "Relabelled"},
		{"s/shared/specs/S1-x.md", "---\nschema_version: 1\nid: other/x\n---\n# Spec\n", "spec", "Spec"},
		{"research/R1-x.md", "# Not shared\n", "", "Not shared"},
		{"s/shared/research/R2-x.md", "---\nkind: research\ntitle: 'Team: sync'\nauthor: Kieran\ncreated: \"2026-10-12T09:30:00+02:00\"\nsession: team-sync\nrevisions:\n  o/app: abc\n---\n# Team: sync\n", "research", "Team: sync"},
	} {
		doc, _, _ := parseDocument(tc.name, tc.text)
		if doc.ID != tc.name[:len(tc.name)-3] || doc.Kind != tc.kind || doc.Title != tc.title {
			t.Errorf("%s = %+v", tc.name, doc)
		}
	}
}

func TestFrontmatterBodyKeepsItsSourceLine(t *testing.T) {
	_, body, line := parseDocument("a.md", "---\nkind: note\ntitle: T\n---\n\n# T\n")
	if line != 5 || body != "\n# T\n" {
		t.Fatalf("body %q starts at line %d", body, line)
	}
}

func TestAnUnclosedOpenerKeepsTheWholeNote(t *testing.T) {
	text := "---\n# Unclosed opener\n\nSearchable retry deadline text.\n"
	doc, body, line := parseDocument("notes/u.md", text)
	if body != text || line != 1 || doc.Title != "u" {
		t.Fatalf("doc %+v, body %q at line %d", doc, body, line)
	}
}
