package vault

import (
	"path"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Edge struct {
	Relation string `yaml:"relation" json:"relation"`
	Target   string `yaml:"target" json:"target"`
}

// Document is what the index knows about one file. Canonical is false for a
// note without schema_version, whose id is its path and whose kind is trusted
// only when it names a known kind.
type Document struct {
	ID        string `json:"id"`
	Kind      string `json:"kind,omitempty"`
	Title     string `json:"title"`
	Canonical bool   `json:"canonical"`
	Lineage   []Edge `json:"lineage,omitempty"`
}

type frontmatter struct {
	SchemaVersion *int   `yaml:"schema_version"`
	ID            string `yaml:"id"`
	Kind          string `yaml:"kind"`
	Title         string `yaml:"title"`
	Date          string `yaml:"date"`
	Author        string `yaml:"author"`
	Session       string `yaml:"session"`
	State         string `yaml:"state"`
	Repos         *[]any `yaml:"repos"`
	Lineage       []Edge `yaml:"lineage"`
}

// parseDocument splits a file into its document, body and the source line the
// body starts on. ok is false for a file the index must leave out: one whose
// schema_version is unsupported, or a version 1 file missing required fields.
func parseDocument(name, text string) (doc Document, body string, bodyLine int, ok bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	body, bodyLine = text, 1
	var fm frontmatter
	if strings.HasPrefix(text, frontmatterOpen) {
		rest := text[len(frontmatterOpen):]
		if end := closingFence(rest); end >= 0 {
			if yaml.Unmarshal([]byte(rest[:end]), &fm) != nil {
				fm = frontmatter{}
			}
			body = strings.TrimPrefix(rest[end+len(frontmatterClose):], "\n")
			bodyLine = strings.Count(text[:len(text)-len(body)], "\n") + 1
		}
	}
	if fm.SchemaVersion != nil {
		if *fm.SchemaVersion != schemaVersion || !fm.complete() {
			return Document{}, "", 0, false
		}
		return Document{ID: fm.ID, Kind: fm.Kind, Title: fm.Title, Canonical: true, Lineage: typed(fm.Lineage)}, body, bodyLine, true
	}
	doc = Document{ID: strings.TrimSuffix(name, path.Ext(name)), Title: fm.Title}
	if kinds[fm.Kind] {
		doc.Kind = fm.Kind
	}
	if doc.Title == "" {
		doc.Title = firstHeading(body)
	}
	if doc.Title == "" {
		doc.Title = path.Base(doc.ID)
	}
	return doc, body, bodyLine, true
}

func closingFence(rest string) int {
	for at := 0; ; {
		i := strings.Index(rest[at:], frontmatterClose)
		if i < 0 {
			return -1
		}
		end := at + i
		after := rest[end+len(frontmatterClose):]
		if after == "" || after[0] == '\n' {
			return end
		}
		at = end + len(frontmatterClose)
	}
}

func (fm frontmatter) complete() bool {
	for _, field := range []string{fm.ID, fm.Kind, fm.Title, fm.Date, fm.Author, fm.Session, fm.State} {
		if strings.TrimSpace(field) == "" {
			return false
		}
	}
	return kinds[fm.Kind] && fm.Repos != nil && strings.HasPrefix(fm.ID, fm.Session+"/") && len(fm.ID) > len(fm.Session)+1
}

func typed(edges []Edge) []Edge {
	var out []Edge
	for _, edge := range edges {
		if (edge.Relation == RelationDerivedFrom || edge.Relation == RelationSupersedes) && edge.Target != "" {
			out = append(out, edge)
		}
	}
	return out
}

func firstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if title, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(title)
		}
	}
	return ""
}
