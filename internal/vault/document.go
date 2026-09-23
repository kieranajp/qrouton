package vault

import (
	"path"
	"strings"

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
	doc = Document{ID: strings.TrimSuffix(name, path.Ext(name)), Kind: pathKind(name), Title: fm.Title}
	if kinds[fm.Kind] {
		doc.Kind = fm.Kind
	}
	if doc.Title == "" {
		doc.Title = firstHeading(body)
	}
	if doc.Title == "" {
		doc.Title = path.Base(doc.ID)
	}
	return doc, body, bodyLine
}

// pathKind reads the kind from a shared/research, specs or plans directory.
func pathKind(name string) string {
	parts := strings.Split(path.Dir(name), "/")
	for i := 1; i < len(parts); i++ {
		if parts[i-1] == sessionpaths.SharedDirName {
			return dirKinds[parts[i]]
		}
	}
	return ""
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

func firstHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if title, ok := strings.CutPrefix(line, "# "); ok {
			return strings.TrimSpace(title)
		}
	}
	return ""
}
