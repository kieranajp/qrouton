package vault

import (
	"slices"
	"testing"
)

const canonical = `---
schema_version: 1
id: s1/R1-2026-01-01-topic
kind: research
title: Topic
date: 2026-01-01
author: A
session: s1
repos:
  - path: github.com/o/r
    role: editing
    revision: null
state: active
lineage:
  - relation: derived_from
    target: s1/Q1
  - relation: mentions
    target: s1/Other
---

# Topic
`

func TestCanonicalDocumentsExposeIdentityKindAndTypedLineage(t *testing.T) {
	doc, body, line, ok := parseDocument("s1/research/R1-2026-01-01-topic.md", canonical)
	if !ok || !doc.Canonical || doc.ID != "s1/R1-2026-01-01-topic" || doc.Kind != KindResearch || doc.Title != "Topic" {
		t.Fatalf("doc = %+v ok=%v", doc, ok)
	}
	if want := []Edge{{RelationDerivedFrom, "s1/Q1"}}; !slices.Equal(doc.Lineage, want) {
		t.Fatalf("lineage = %+v", doc.Lineage)
	}
	if line != 20 || body != "\n# Topic\n" {
		t.Fatalf("body %q starts at line %d", body, line)
	}
}

func TestDocumentsTheIndexLeavesOut(t *testing.T) {
	for name, text := range map[string]string{
		"future schema":  "---\nschema_version: 2\nid: s/x\n---\nBody\n",
		"missing author": "---\nschema_version: 1\nid: s/x\nkind: note\ntitle: X\ndate: 2026-01-01\nsession: s\nrepos: []\nstate: active\n---\n",
		"foreign id":     "---\nschema_version: 1\nid: other/x\nkind: note\ntitle: X\ndate: 2026-01-01\nauthor: A\nsession: s\nrepos: []\nstate: active\n---\n",
		"unknown kind":   "---\nschema_version: 1\nid: s/x\nkind: memo\ntitle: X\ndate: 2026-01-01\nauthor: A\nsession: s\nrepos: []\nstate: active\n---\n",
	} {
		if _, _, _, ok := parseDocument("s/x.md", text); ok {
			t.Errorf("%s was indexed", name)
		}
	}
}

func TestPlainNotesKeepPathIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, text, kind, title string
	}{
		{"notes/a.md", "# Heading title\n\nText\n", "", "Heading title"},
		{"notes/b.md", "---\nkind: plan\ntitle: From frontmatter\n---\nText\n", "plan", "From frontmatter"},
		{"notes/c.md", "---\nkind: whatever\n---\nno heading\n", "", "c"},
		{"notes/d.md", "---\nbroken: [yaml\n---\n# D\n", "", "D"},
	} {
		doc, _, _, ok := parseDocument(tc.name, tc.text)
		if !ok || doc.Canonical || doc.ID != tc.name[:len(tc.name)-3] || doc.Kind != tc.kind || doc.Title != tc.title {
			t.Errorf("%s = %+v ok=%v", tc.name, doc, ok)
		}
	}
}
