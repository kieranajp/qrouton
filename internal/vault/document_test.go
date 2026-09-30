package vault

import "testing"

func TestDocumentsTakeTheirIdentityFromThePath(t *testing.T) {
	for _, tc := range []struct {
		name, text, kind, title string
	}{
		{"notes/a.md", "# Heading title\n\nText\n", "", "Heading title"},
		{"notes/b.md", "---\nkind: plan\ntitle: From frontmatter\n---\nText\n", "plan", "From frontmatter"},
		{"notes/c.md", "---\nkind: whatever\n---\nno heading\n", "", "c"},
		{"notes/d.md", "---\nbroken: [yaml\n---\n# D\n", "", "D"},
		{"s/shared/research/R1-x.md", "# Old research\n", KindResearch, "Old research"},
		{"s/shared/plans/P1-x.md", "---\nkind: note\n---\n# Relabelled\n", "note", "Relabelled"},
		{"s/shared/specs/S1-x.md", "---\nschema_version: 1\nid: other/x\n---\n# Spec\n", "spec", "Spec"},
		{"research/R1-x.md", "# Not shared\n", "", "Not shared"},
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
