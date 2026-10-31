package vault

import (
	"path"
	"slices"
	"strings"

	"github.com/kieranajp/qrouton/internal/kinds"
	"github.com/kieranajp/qrouton/internal/markdown"
	"github.com/kieranajp/qrouton/internal/sessionpaths"
	"go.yaml.in/yaml/v3"
)

// Document is what the index knows about one file: its path is its id, and
// its kind is trusted only when it names a known kind.
type Document struct {
	ID    string `json:"id"`
	Kind  string `json:"kind,omitempty"`
	Title string `json:"title"`
}

type frontmatter struct {
	Kind  string `yaml:"kind"`
	Title string `yaml:"title"`
}

// parseDocument splits a file into its document, body and the source line the
// body starts on.
func parseDocument(name, text string) (doc Document, body string, bodyLine int) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	front, body, bodyLine := markdown.Split(text)
	if bodyLine == 0 {
		// An opener that never closes is a rule, and the note is still searchable.
		body, bodyLine = text, 1
	}
	var fm frontmatter
	if yaml.Unmarshal([]byte(front), &fm) != nil {
		fm = frontmatter{}
	}
	doc = Document{ID: strings.TrimSuffix(name, path.Ext(name)), Kind: pathKind(name), Title: fm.Title}
	if slices.Contains(Kinds(), fm.Kind) {
		doc.Kind = fm.Kind
	}
	if doc.Title == "" {
		doc.Title, _ = markdown.Title(text)
	}
	if doc.Title == "" {
		doc.Title = path.Base(doc.ID)
	}
	return doc, body, bodyLine
}

func pathKind(name string) string {
	parts := strings.Split(path.Dir(name), "/")
	for i := 1; i < len(parts); i++ {
		if parts[i-1] == sessionpaths.SharedDirName {
			return kindByDir[parts[i]]
		}
	}
	return ""
}

var kindByDir = func() map[string]string {
	out := map[string]string{}
	for _, k := range kinds.All() {
		out[k.Dir] = k.Name
	}
	return out
}()

// Kinds is every kind a document can carry and a search can filter by.
func Kinds() []string {
	var out []string
	for _, k := range kinds.All() {
		out = append(out, k.Name)
	}
	return append(out, kindNote, kindDeadEnd)
}
